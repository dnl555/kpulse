package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFullYAML(t *testing.T) {
	data, err := os.ReadFile("testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Cluster.Name != "prod-eks-1" {
		t.Errorf("cluster.name = %q", cfg.Cluster.Name)
	}
	if !cfg.Channels.Slack.Default {
		t.Error("slack.default should be true")
	}
	if cfg.Monitors.PVCUsage.WarnAt != 80 || cfg.Monitors.PVCUsage.CritAt != 90 {
		t.Errorf("pvc thresholds wrong: %+v", cfg.Monitors.PVCUsage)
	}
	if cfg.Monitors.PVCUsage.Interval != 10*time.Minute {
		t.Errorf("pvc interval = %v", cfg.Monitors.PVCUsage.Interval)
	}
	if cfg.Dedupe.Window != 30*time.Minute {
		t.Errorf("dedupe window = %v", cfg.Dedupe.Window)
	}
	if len(cfg.Routing) != 2 {
		t.Errorf("routing len = %d", len(cfg.Routing))
	}
}

func TestDefaultsApplied(t *testing.T) {
	cfg, err := Parse([]byte("cluster: {name: x}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Monitors.PodCrashes.Enabled {
		t.Error("pod_crashes default should be enabled")
	}
	if cfg.Dedupe.Window == 0 {
		t.Error("dedupe.window default missing")
	}
	if len(cfg.Monitors.PodCrashes.Reasons) == 0 {
		t.Error("pod_crashes default reasons missing")
	}
}

func TestMissingClusterNameFails(t *testing.T) {
	_, err := Parse([]byte("monitors: {}\n"))
	if err == nil {
		t.Error("expected validate error for missing cluster.name")
	}
}

func TestNamespaceMatch(t *testing.T) {
	ns := NamespaceFilter{Include: []string{"*"}, Exclude: []string{"kube-system"}}
	if !ns.Allows("default") {
		t.Error("default should be allowed")
	}
	if ns.Allows("kube-system") {
		t.Error("kube-system should be excluded")
	}
}
