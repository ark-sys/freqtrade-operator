package resources

import (
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "SampleStrategy", "my-bt-config", "my-bt-strategy")

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
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "SampleStrategy", "cfg", "strategy-cm")

	for _, v := range podSpec.Volumes {
		if v.Name == "cache" {
			t.Error("expected no cache volume when spec.data is unset")
		}
	}
	if len(podSpec.InitContainers) != 1 {
		t.Errorf("expected exactly one init container (init-user-data) with no cache, got %d", len(podSpec.InitContainers))
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
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "SampleStrategy", "cfg", "strategy-cm")

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
	if len(podSpec.InitContainers) != 2 {
		t.Fatalf("expected init-user-data + init-download-data, got %d: %+v",
			len(podSpec.InitContainers), podSpec.InitContainers)
	}
	if podSpec.InitContainers[1].Name != "init-download-data" {
		t.Errorf("expected second init container to be init-download-data, got %s", podSpec.InitContainers[1].Name)
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

func TestMergePodSpecOverrides_ImageOverride(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{Pod: &freqtradev1beta1.PodSpec{Image: "custom/freqtrade:latest"}},
		},
	}
	podSpec := BuildPod(backtest, "freqtradeorg/freqtrade:2024.1", "SampleStrategy", "cfg", "strategy-cm")
	if podSpec.Containers[0].Image != "custom/freqtrade:latest" {
		t.Errorf("expected overridden image, got %s", podSpec.Containers[0].Image)
	}
}
