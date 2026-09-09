package resources

import (
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildArgs_TypedFieldsMapToFlags(t *testing.T) {
	fee := "0.001"
	dryRunWallet := resource.MustParse("1000")
	spec := freqtradev1beta1.BacktestSpec{
		RunSpec: freqtradev1beta1.RunSpec{
			Timerange: "20230101-20230201",
			Timeframe: "5m",
			Pairs:     []string{"BTC/USDT", "ETH/USDT"},
		},
		TimeframeDetail:   "1m",
		MaxOpenTrades:     ptr.To(3),
		StakeAmount:       "unlimited",
		DryRunWallet:      &dryRunWallet,
		Fee:               &fee,
		EnableProtections: ptr.To(true),
		Breakdown:         []string{"day", "week"},
		Cache:             "day",
	}

	args := buildArgs(spec, "SampleStrategy", false, nil)

	want := []string{
		"backtesting",
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", "SampleStrategy",
		"--userdir", "/freqtrade/user_data",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
		"--timerange", "20230101-20230201",
		"--timeframe", "5m",
		"--timeframe-detail", "1m",
		"--pairs", "BTC/USDT", "ETH/USDT",
		"--max-open-trades", "3",
		"--stake-amount", "unlimited",
		"--dry-run-wallet", "1000",
		"--fee", "0.001",
		"--enable-protections",
		"--breakdown", "day", "week",
		"--cache", "day",
	}
	if len(args) != len(want) {
		t.Fatalf("buildArgs() = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("buildArgs()[%d] = %q, want %q (full: %v)", i, args[i], want[i], args)
		}
	}
}

func TestBuildArgs_HasCacheAddsDatadir(t *testing.T) {
	args := buildArgs(freqtradev1beta1.BacktestSpec{}, "SampleStrategy", true, nil)
	found := false
	for i, a := range args {
		if a == "--datadir" && i+1 < len(args) && args[i+1] == "/cache" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --datadir /cache in args, got %v", args)
	}
}

func TestBuildArgs_ExtraArgsAppendedLast(t *testing.T) {
	args := buildArgs(freqtradev1beta1.BacktestSpec{}, "SampleStrategy", false, []string{"--enable-position-stacking"})
	if args[len(args)-1] != "--enable-position-stacking" {
		t.Errorf("expected extraArgs appended last, got %v", args)
	}
}

func TestBuildPod_DefaultsAreRestrictedAndNeverAutomountsToken(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{}
	podSpec := BuildPod(
		backtest, "freqtradeorg/freqtrade:2024.1", "operator-img", "SampleStrategy", "my-bt-config", "my-bt-strategy",
	)

	if podSpec.AutomountServiceAccountToken == nil || *podSpec.AutomountServiceAccountToken {
		t.Error("expected AutomountServiceAccountToken to default to false")
	}
	if podSpec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("expected RestartPolicyNever, got %v", podSpec.RestartPolicy)
	}
	for _, c := range podSpec.Containers {
		if c.SecurityContext == nil || c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
			t.Errorf("container %s: expected RunAsNonRoot securityContext", c.Name)
		}
	}
	if len(podSpec.Containers) != 1 || podSpec.Containers[0].Name != "freqtrade" {
		t.Fatalf("expected exactly one freqtrade container, got %+v", podSpec.Containers)
	}
}

func TestBuildPod_NoCacheMeansNoCacheVolumeOrInitContainer(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{}
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "operator-img", "SampleStrategy", "cfg", "strategy-cm")

	for _, v := range podSpec.Volumes {
		if v.Name == "cache" {
			t.Error("expected no cache volume when spec.data is unset")
		}
	}
	// init-user-data + the P6-2 results sidecar, always present regardless of cache.
	if len(podSpec.InitContainers) != 2 {
		t.Errorf("expected exactly two init containers with no cache, got %d: %+v",
			len(podSpec.InitContainers), podSpec.InitContainers)
	}
}

// Regression test: the results PVC BuildResultsPVC provisions was never actually mounted
// anywhere - a Backtest's raw result file only ever lived on the pod's own ephemeral user-data
// emptyDir, so it never survived past whatever TTLSecondsAfterFinished eventually reaped the Job
// pod, despite status.resultsPVCName naming a real, durable-looking PVC. Unlike the cache volume,
// this is unconditional: every Backtest gets a results PVC (BuildResultsPVC has no gating field of
// its own), so BuildPod must always mount it, not just when spec.data is set.
func TestBuildPod_AlwaysMountsResultsPVCIntoSidecarOnly(t *testing.T) {
	const resultsVolumeName = "results"
	backtest := freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run"}}
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "operator-img", "SampleStrategy", "cfg", "strategy-cm")

	var resultsVol *corev1.Volume
	for i := range podSpec.Volumes {
		if podSpec.Volumes[i].Name == resultsVolumeName {
			resultsVol = &podSpec.Volumes[i]
		}
	}
	if resultsVol == nil || resultsVol.PersistentVolumeClaim == nil ||
		resultsVol.PersistentVolumeClaim.ClaimName != ResultsPVCName("my-run") {
		t.Fatalf("expected a results volume bound to %s, got %+v", ResultsPVCName("my-run"), podSpec.Volumes)
	}

	sidecar := podSpec.InitContainers[len(podSpec.InitContainers)-1]
	if sidecar.Name != "collect-results" {
		t.Fatalf("expected the last init container to be collect-results, got %s", sidecar.Name)
	}
	found := false
	for _, vm := range sidecar.VolumeMounts {
		if vm.Name == resultsVolumeName {
			found = true
			if vm.ReadOnly {
				t.Error("expected the sidecar's results mount to be writable, got ReadOnly")
			}
			if vm.MountPath != ResultsPVCMountPath {
				t.Errorf("expected mount path %s, got %s", ResultsPVCMountPath, vm.MountPath)
			}
		}
	}
	if !found {
		t.Errorf("expected the sidecar to mount the results volume, got %+v", sidecar.VolumeMounts)
	}

	argFound := false
	for i, a := range sidecar.Args {
		if a == "--results-pvc-dir" && i+1 < len(sidecar.Args) && sidecar.Args[i+1] == ResultsPVCMountPath {
			argFound = true
		}
	}
	if !argFound {
		t.Errorf("expected --results-pvc-dir %s in sidecar Args, got %v", ResultsPVCMountPath, sidecar.Args)
	}

	for _, c := range podSpec.Containers {
		for _, vm := range c.VolumeMounts {
			if vm.Name == resultsVolumeName {
				t.Errorf("expected only the sidecar to mount results, but main container %s does too", c.Name)
			}
		}
	}
}

func TestBuildPod_CacheAddsVolumeAndDownloadInitContainer(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{
				Data: &freqtradev1beta1.DataSourceSpec{PVCName: "shared-cache"},
			},
		},
	}
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "operator-img", "SampleStrategy", "cfg", "strategy-cm")

	var cacheVol *corev1.Volume
	for i := range podSpec.Volumes {
		if podSpec.Volumes[i].Name == "cache" {
			cacheVol = &podSpec.Volumes[i]
		}
	}
	if cacheVol == nil || cacheVol.PersistentVolumeClaim == nil ||
		cacheVol.PersistentVolumeClaim.ClaimName != "shared-cache" {
		t.Fatalf("expected a cache volume bound to shared-cache, got %+v", podSpec.Volumes)
	}
	// init-user-data + init-download-data + the P6-2 results sidecar.
	if len(podSpec.InitContainers) != 3 {
		t.Fatalf("expected three init containers, got %d: %+v", len(podSpec.InitContainers), podSpec.InitContainers)
	}
	if podSpec.InitContainers[1].Name != "init-download-data" {
		t.Errorf("expected second init container to be init-download-data, got %s", podSpec.InitContainers[1].Name)
	}
	if podSpec.InitContainers[2].Name != "collect-results" {
		t.Errorf("expected third init container to be collect-results, got %s", podSpec.InitContainers[2].Name)
	}
}

func TestBuildDownloadDataInitContainer_NeverPolicyIsANoOp(t *testing.T) {
	spec := freqtradev1beta1.BacktestSpec{
		RunSpec: freqtradev1beta1.RunSpec{
			Data: &freqtradev1beta1.DataSourceSpec{PVCName: "cache", DownloadPolicy: "never"},
		},
	}
	c := buildDownloadDataInitContainer(spec, "img", nil)
	if len(c.Command) != 1 || c.Command[0] != "true" {
		t.Errorf("expected a no-op command for policy=never, got %v", c.Command)
	}
}

func TestBuildDownloadDataInitContainer_IfMissingWrapsInShellCheck(t *testing.T) {
	spec := freqtradev1beta1.BacktestSpec{
		RunSpec: freqtradev1beta1.RunSpec{
			Data: &freqtradev1beta1.DataSourceSpec{PVCName: "cache", DownloadPolicy: "ifMissing"},
		},
	}
	c := buildDownloadDataInitContainer(spec, "img", nil)
	if len(c.Command) < 2 || c.Command[0] != "sh" {
		t.Errorf("expected a sh -c wrapper for policy=ifMissing, got %v", c.Command)
	}
}

// Regression test: this init container's Args previously never included --config at all, even
// though the config Secret is mounted into it (see initMounts in BuildPod) - "freqtrade
// download-data" with no --config and no --exchange has no idea which exchange to talk to and
// fails immediately with "This command requires a configured exchange". --timerange and -t were
// missing too, which is a quieter failure: download-data still exits 0, but silently downloads
// whatever its own default window is instead of the data the run's own --timerange/--timeframe
// will actually look for, so backtesting then fails with "No data found" despite the cache PVC
// having *something* on it. Found directly on a real cluster, not envtest: a Backtest whose
// pods.data.pvcName was correctly wired still failed both ways in sequence.
func TestBuildDownloadDataInitContainer_AlwaysPolicyPassesConfigTimerangeAndTimeframe(t *testing.T) {
	spec := freqtradev1beta1.BacktestSpec{
		RunSpec: freqtradev1beta1.RunSpec{
			Timerange: "20230101-20230201",
			Timeframe: "5m",
			Pairs:     []string{"BTC/USDT", "ETH/USDT"},
			Data:      &freqtradev1beta1.DataSourceSpec{PVCName: "cache"},
		},
	}
	c := buildDownloadDataInitContainer(spec, "img", nil)

	want := []string{
		"download-data", "--config", "/config/config.json",
		"--userdir", "/freqtrade/user_data", "--datadir", "/cache",
		"--timerange", "20230101-20230201",
		"-t", "5m",
		"--pairs", "BTC/USDT", "ETH/USDT",
	}
	if len(c.Command) != 1 || c.Command[0] != "freqtrade" {
		t.Fatalf("expected command [freqtrade] for policy=always, got %v", c.Command)
	}
	if len(c.Args) != len(want) {
		t.Fatalf("buildDownloadDataInitContainer().Args = %v, want %v", c.Args, want)
	}
	for i := range want {
		if c.Args[i] != want[i] {
			t.Errorf("Args[%d] = %q, want %q (full: %v)", i, c.Args[i], want[i], c.Args)
		}
	}
}

func TestBuildDownloadDataInitContainer_DownloadArgsAppendedLast(t *testing.T) {
	spec := freqtradev1beta1.BacktestSpec{
		RunSpec: freqtradev1beta1.RunSpec{
			Data: &freqtradev1beta1.DataSourceSpec{PVCName: "cache", DownloadArgs: []string{"--days", "30"}},
		},
	}
	c := buildDownloadDataInitContainer(spec, "img", nil)
	if len(c.Args) < 2 || c.Args[len(c.Args)-2] != "--days" || c.Args[len(c.Args)-1] != "30" {
		t.Errorf("expected DownloadArgs appended last, got %v", c.Args)
	}
}

func TestMergePodSpecOverrides_ImageOverride(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{Pod: &freqtradev1beta1.PodSpec{Image: "custom/freqtrade:latest"}},
		},
	}
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "operator-img", "SampleStrategy", "cfg", "strategy-cm")
	if podSpec.Containers[0].Image != "custom/freqtrade:latest" {
		t.Errorf("expected overridden image, got %s", podSpec.Containers[0].Image)
	}
}
