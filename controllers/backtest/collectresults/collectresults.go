// Package collectresults is the P6-2 results-collection sidecar's own
// logic: it runs as a native sidecar (RestartPolicy: Always init
// container, see controllers/backtest/resources/pod.go) inside a
// Backtest's Job pod, waits for the freqtrade container to exit, reads
// its result file off their shared user-data volume, and writes a summary
// ConfigMap the operator's own reconcile loop later reads back
// (controllers/backtest/results.go).
//
// This package never runs inside the manager - it's invoked as
// `manager collect-results ...` from the operator's own binary, running
// as an entirely separate process in a different pod (see cmd/main.go).
package collectresults

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// pollInterval is how often this checks whether the main container has
// exited yet.
const pollInterval = 2 * time.Second

// maxWait bounds the total time spent waiting for the main container to
// exit, purely as a safety net against an infinite hang inside this one
// sidecar invocation (RestartPolicy: Always means kubelet restarts it if
// it exits non-zero, so this isn't the only line of defense) - a backtest
// run legitimately taking hours is not this sidecar's problem to solve, so
// this is deliberately generous.
const maxWait = 24 * time.Hour

// resultsConfigMapDataKey is where a successful extraction's marshaled
// BacktestResults JSON lives in the ConfigMap this writes - the operator's
// own read side (controllers/backtest/results.go) unmarshals this key
// directly back into *v1beta1.BacktestResults.
const resultsConfigMapDataKey = "results.json"

// errorConfigMapDataKey holds a human-readable description of why
// extraction failed, on the (still successful, not fatal) other path -
// see Run's own doc comment.
const errorConfigMapDataKey = "error"

// Options configures one Run - see cmd/main.go for how each field is
// sourced (mostly flags plus the POD_NAME/POD_NAMESPACE downward API env
// vars every Backtest Job pod carries).
type Options struct {
	BacktestName      string
	StrategyName      string
	PodName           string
	Namespace         string
	MainContainerName string
	ResultsDir        string
}

// Run waits for the main freqtrade container to exit, then attempts to
// extract a summary from its result file and writes it to
// "<BacktestName>-results". A failure to find or parse that file is
// reported via the ConfigMap's own errorConfigMapDataKey, not a non-zero
// exit - per P6-2's own design, the run already happened and only the
// summary is missing, which controllers/backtest/results.go surfaces as
// ResultsAvailable=False/ResultsUnavailable rather than failing the
// Backtest. A non-nil return here is reserved for something this sidecar
// itself couldn't recover from (no Kubernetes API access at all, its own
// config malformed) - kubelet restarts it (RestartPolicy: Always) rather
// than the Job silently completing with no results ConfigMap at all.
func Run(ctx context.Context, opts Options) error {
	logger := log.FromContext(ctx).WithName("collect-results")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to build in-cluster config: %w", err)
	}
	c, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return fmt.Errorf("failed to build Kubernetes client: %w", err)
	}

	if err := waitForMainContainerExit(ctx, c, opts); err != nil {
		return err
	}

	data := extract(opts, logger)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: opts.BacktestName + "-results", Namespace: opts.Namespace},
		Data:       data,
	}
	if err := c.Create(ctx, &cm); err != nil {
		return fmt.Errorf("failed to create results ConfigMap: %w", err)
	}
	logger.Info("wrote results ConfigMap", "name", cm.Name, "hasResults", data[resultsConfigMapDataKey] != "")
	return nil
}

// waitForMainContainerExit polls this sidecar's own Pod until
// opts.MainContainerName's status reports Terminated.
func waitForMainContainerExit(ctx context.Context, c client.Client, opts Options) error {
	deadline := time.Now().Add(maxWait)
	key := types.NamespacedName{Name: opts.PodName, Namespace: opts.Namespace}

	for {
		var pod corev1.Pod
		if err := c.Get(ctx, key, &pod); err != nil {
			return fmt.Errorf("failed to get own Pod %s: %w", key, err)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == opts.MainContainerName && cs.State.Terminated != nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for container %q to exit", maxWait, opts.MainContainerName)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// extract reads and parses the freqtrade result file, returning ConfigMap
// data with either resultsConfigMapDataKey or errorConfigMapDataKey set -
// never both, never neither.
func extract(opts Options, logger logr.Logger) map[string]string {
	resultFile, err := latestResultFile(opts.ResultsDir)
	if err != nil {
		logger.Error(err, "failed to determine the latest result file")
		return map[string]string{errorConfigMapDataKey: err.Error()}
	}

	results, err := parseResultFile(resultFile, opts.StrategyName)
	if err != nil {
		logger.Error(err, "failed to parse the result file", "file", resultFile)
		return map[string]string{errorConfigMapDataKey: err.Error()}
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		// Marshaling our own struct back to JSON failing would be a bug in
		// this package, not a bad result file - still handled the same
		// non-fatal way, since a Backtest losing its summary is never worth
		// failing the whole run over.
		logger.Error(err, "failed to marshal extracted results")
		return map[string]string{errorConfigMapDataKey: err.Error()}
	}
	return map[string]string{resultsConfigMapDataKey: string(resultsJSON)}
}

// lastResultPointer is freqtrade's own .last_result.json shape.
type lastResultPointer struct {
	LatestBacktest string `json:"latest_backtest"`
}

func latestResultFile(resultsDir string) (string, error) {
	pointerPath := filepath.Join(resultsDir, ".last_result.json")
	raw, err := os.ReadFile(pointerPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", pointerPath, err)
	}
	var pointer lastResultPointer
	if err := json.Unmarshal(raw, &pointer); err != nil {
		return "", fmt.Errorf("parsing %s: %w", pointerPath, err)
	}
	if pointer.LatestBacktest == "" {
		return "", fmt.Errorf("%s has no latest_backtest entry", pointerPath)
	}
	return filepath.Join(resultsDir, pointer.LatestBacktest), nil
}
