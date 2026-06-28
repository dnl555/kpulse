package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	"github.com/dnl555/kpulse/internal/engine"
	"github.com/dnl555/kpulse/internal/httpsrv"
	"github.com/dnl555/kpulse/internal/monitors"
	"github.com/dnl555/kpulse/internal/notifiers"
	"github.com/dnl555/kpulse/internal/state"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "dev"

func main() {
	cfgPath := flag.String("config", "/etc/kpulse/config.yaml", "ConfigMap-mounted config path")
	secretPath := flag.String("secrets", "/etc/kpulse/secrets", "directory containing per-key secret files")
	ns := flag.String("namespace", envOr("POD_NAMESPACE", "kpulse"), "kpulse namespace (state ConfigMap location)")
	httpAddr := flag.String("http", ":8080", "HTTP listen addr")
	flag.Parse()

	log.Printf("kpulse %s starting", version)

	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	cfg, err := config.Parse(raw)
	if err != nil {
		log.Fatalf("parse config: %v", err)
	}

	sec, err := loadSecretDir(*secretPath)
	if err != nil {
		log.Fatalf("load secrets: %v", err)
	}

	reg, err := notifiers.Build(cfg, sec)
	if err != nil {
		log.Fatalf("build notifiers: %v", err)
	}
	if len(reg.Names()) == 0 {
		log.Print("warn: no channels configured; alerts will be dropped. Add credentials to Secret/kpulse-secrets and enable a channel in ConfigMap/kpulse-config.")
	} else {
		log.Printf("notifiers ready: %v", reg.Names())
	}

	k8s, err := k8sClient()
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}

	store := state.New(k8s, *ns, "kpulse-state")
	deduper := engine.NewDeduper(cfg.Dedupe.Window)
	if loaded, err := store.Load(context.Background()); err == nil && loaded != nil {
		deduper.Restore(loaded)
		log.Printf("restored %d dedupe entries from state", len(loaded))
	}

	digestSevs := []alert.Severity{}
	for _, s := range cfg.Dedupe.Digest.Severities {
		if sv, err := alert.ParseSeverity(s); err == nil {
			digestSevs = append(digestSevs, sv)
		}
	}
	defaults := defaultChannels(cfg, reg)
	router := engine.NewRouter(cfg.Routing, defaults)

	eng := engine.New(engine.Options{
		Dedupe: deduper, Router: router, Registry: reg, Cluster: cfg.Cluster.Name,
		DigestEnabled: cfg.Dedupe.Digest.Enabled, DigestInterval: cfg.Dedupe.Digest.Interval, DigestSeverities: digestSevs,
		ResolutionEnabled: cfg.Resolution.Enabled,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var readyMu sync.Mutex
	isReady := false
	getReady := func() bool { readyMu.Lock(); defer readyMu.Unlock(); return isReady }

	mons := buildMonitors(k8s, cfg)
	log.Printf("starting %d monitors", len(mons))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); eng.Run(ctx) }()
	for _, m := range mons {
		wg.Add(1)
		go func(m monitors.Monitor) {
			defer wg.Done()
			if err := m.Run(ctx, eng); err != nil {
				log.Printf("monitor %s exited: %v", m.Name(), err)
			}
		}(m)
	}

	go snapshotLoop(ctx, store, deduper)

	readyMu.Lock()
	isReady = true
	readyMu.Unlock()

	srv := httpsrv.New(reg, getReady).WithReset(eng, store)
	startedAt := time.Now().UTC()
	if cfg.UI.Enabled {
		srv = srv.WithUI(eng, clusterInfoFn(cfg, startedAt), monitorsViewFn(cfg), httpsrv.UIFS())
		log.Print("UI enabled, serving at /")
	}
	go func() {
		if err := srv.ListenAndServe(ctx, *httpAddr); err != nil && err.Error() != "http: Server closed" {
			log.Printf("http: %v", err)
		}
	}()

	log.Printf("kpulse ready, watching cluster %q", cfg.Cluster.Name)
	<-ctx.Done()
	log.Print("shutting down")
	wg.Wait()
	if err := store.Save(context.Background(), deduper.Snapshot()); err != nil {
		log.Printf("final state save: %v", err)
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func loadSecretDir(dir string) (config.SecretMap, error) {
	out := config.SecretMap{}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		// Kubernetes projects Secrets via a "..data" symlink to a timestamped
		// dir (e.g. "..2026_05_24_09_06_08.123456"). Skip the bookkeeping
		// entries; the real keys are flat files in the same directory.
		if strings.HasPrefix(name, "..") {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out[name] = string(trimNewline(data))
	}
	return out, nil
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func k8sClient() (kubernetes.Interface, error) {
	rc, err := rest.InClusterConfig()
	if err != nil {
		rc, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(rc)
}

func defaultChannels(cfg *config.Config, reg *notifiers.Registry) []string {
	if cfg.Channels.Slack.Default {
		if _, ok := reg.Get("slack"); ok {
			return []string{"slack"}
		}
	}
	names := reg.Names()
	if len(names) > 0 {
		return names[:1]
	}
	return nil
}

func buildMonitors(cs kubernetes.Interface, cfg *config.Config) []monitors.Monitor {
	var out []monitors.Monitor
	if cfg.Monitors.PodCrashes.Enabled {
		out = append(out, monitors.NewPodCrashes(cs, cfg))
	}
	if cfg.Monitors.PodRestarts.Enabled {
		out = append(out, monitors.NewPodRestarts(cs, cfg))
	}
	if cfg.Monitors.WarningEvents.Enabled {
		out = append(out, monitors.NewWarningEvents(cs, cfg))
	}
	if cfg.Monitors.PVCUsage.Enabled {
		out = append(out, monitors.NewPVCUsage(cs, cfg))
	}
	if cfg.Monitors.NodeConditions.Enabled {
		out = append(out, monitors.NewNodeConditions(cs, cfg))
	}
	if cfg.Monitors.NodeDisk.Enabled {
		out = append(out, monitors.NewNodeDisk(cs, cfg))
	}
	if cfg.Monitors.TLSCertExpiry.Enabled {
		out = append(out, monitors.NewTLSCertExpiry(cs, cfg))
	}
	if cfg.Monitors.RolloutStuck.Enabled {
		out = append(out, monitors.NewRolloutStuck(cs, cfg))
	}
	if cfg.Monitors.JobFailed.Enabled {
		out = append(out, monitors.NewJobFailed(cs, cfg))
	}
	if cfg.Monitors.CronJobMissed.Enabled {
		out = append(out, monitors.NewCronJobMissed(cs, cfg))
	}
	if cfg.Monitors.HPAAtMax.Enabled {
		out = append(out, monitors.NewHPAAtMax(cs, cfg))
	}
	if cfg.Monitors.DaemonSetUnscheduled.Enabled {
		out = append(out, monitors.NewDaemonSetUnscheduled(cs, cfg))
	}
	return out
}

// clusterInfoFn returns a closure that produces the snapshot the UI's
// /api/v1/cluster endpoint serves. We capture the runtime config + startup
// timestamp once; everything else (uptime, etc.) is derived in the UI.
func clusterInfoFn(cfg *config.Config, startedAt time.Time) func() httpsrv.ClusterInfo {
	return func() httpsrv.ClusterInfo {
		return httpsrv.ClusterInfo{
			Name:              cfg.Cluster.Name,
			Version:           version,
			StartedAt:         startedAt,
			NamespacesInclude: cfg.Namespaces.Include,
			NamespacesExclude: cfg.Namespaces.Exclude,
			DedupeWindow:      cfg.Dedupe.Window.String(),
			DigestEnabled:     cfg.Dedupe.Digest.Enabled,
			DigestInterval:    cfg.Dedupe.Digest.Interval.String(),
			ResolutionEnabled: cfg.Resolution.Enabled,
		}
	}
}

// monitorsViewFn returns a closure that lists every built-in monitor with its
// enabled flag + per-monitor knobs, for the UI's /api/v1/monitors endpoint.
// The engine merges per-monitor fires/resolves on top of this view.
func monitorsViewFn(cfg *config.Config) httpsrv.MonitorsProvider {
	return func() []httpsrv.MonitorView {
		m := cfg.Monitors
		return []httpsrv.MonitorView{
			{Name: "pod_crashes", Enabled: m.PodCrashes.Enabled, Knobs: map[string]any{"reasons": m.PodCrashes.Reasons, "include_recent_logs": m.PodCrashes.IncludeRecentLogs}},
			{Name: "pod_restarts", Enabled: m.PodRestarts.Enabled, Knobs: map[string]any{"threshold": m.PodRestarts.Threshold, "window": m.PodRestarts.Window.String()}},
			{Name: "warning_events", Enabled: m.WarningEvents.Enabled, Knobs: map[string]any{"reasons_ignore": m.WarningEvents.ReasonsIgnore}},
			{Name: "pvc_usage", Enabled: m.PVCUsage.Enabled, Knobs: map[string]any{"warn_at": m.PVCUsage.WarnAt, "crit_at": m.PVCUsage.CritAt, "interval": m.PVCUsage.Interval.String()}},
			{Name: "node_conditions", Enabled: m.NodeConditions.Enabled, Knobs: map[string]any{"alert_on": m.NodeConditions.AlertOn}},
			{Name: "node_disk", Enabled: m.NodeDisk.Enabled, Knobs: map[string]any{"warn_at": m.NodeDisk.WarnAt, "crit_at": m.NodeDisk.CritAt, "interval": m.NodeDisk.Interval.String()}},
			{Name: "tls_cert_expiry", Enabled: m.TLSCertExpiry.Enabled, Knobs: map[string]any{"warn_days": m.TLSCertExpiry.WarnDays, "crit_days": m.TLSCertExpiry.CritDays, "interval": m.TLSCertExpiry.Interval.String()}},
			{Name: "rollout_stuck", Enabled: m.RolloutStuck.Enabled, Knobs: map[string]any{"threshold": m.RolloutStuck.Threshold.String()}},
			{Name: "job_failed", Enabled: m.JobFailed.Enabled},
			{Name: "cronjob_missed", Enabled: m.CronJobMissed.Enabled, Knobs: map[string]any{"miss_threshold": m.CronJobMissed.MissThreshold}},
			{Name: "hpa_at_max", Enabled: m.HPAAtMax.Enabled, Knobs: map[string]any{"duration": m.HPAAtMax.Duration.String()}},
			{Name: "daemonset_unscheduled", Enabled: m.DaemonSetUnscheduled.Enabled, Knobs: map[string]any{"threshold": m.DaemonSetUnscheduled.Threshold.String()}},
		}
	}
}

func snapshotLoop(ctx context.Context, st *state.Store, d *engine.Deduper) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := st.Save(ctx, d.Snapshot()); err != nil {
				log.Printf("snapshot save: %v", err)
			}
		}
	}
}
