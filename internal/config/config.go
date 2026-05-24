// Package config loads and validates the kpulse ConfigMap.
package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Cluster    Cluster         `yaml:"cluster"`
	Channels   Channels        `yaml:"channels"`
	Namespaces NamespaceFilter `yaml:"namespaces"`
	Monitors   Monitors        `yaml:"monitors"`
	Dedupe     Dedupe          `yaml:"dedupe"`
	Resolution Resolution      `yaml:"resolution"`
	Routing    []RoutingRule   `yaml:"routing"`
}

type Resolution struct {
	Enabled bool `yaml:"enabled"`
}

type Cluster struct {
	Name string `yaml:"name"`
}

type Channels struct {
	Slack   SlackChannel   `yaml:"slack"`
	Email   EmailChannel   `yaml:"email"`
	Webhook WebhookChannel `yaml:"webhook"`
	Discord DiscordChannel `yaml:"discord"`
	Teams   TeamsChannel   `yaml:"teams"`
}

type SlackChannel struct {
	WebhookURLFromSecret string `yaml:"webhook_url_from_secret"`
	Default              bool   `yaml:"default"`
}
type EmailChannel struct {
	SMTPHost       string   `yaml:"smtp_host"`
	SMTPPort       int      `yaml:"smtp_port"`
	From           string   `yaml:"from"`
	To             []string `yaml:"to"`
	UserFromSecret string   `yaml:"user_from_secret"`
	PassFromSecret string   `yaml:"pass_from_secret"`
}
type WebhookChannel struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}
type DiscordChannel struct {
	WebhookURLFromSecret string `yaml:"webhook_url_from_secret"`
}
type TeamsChannel struct {
	WebhookURLFromSecret string `yaml:"webhook_url_from_secret"`
}

type NamespaceFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

func (n NamespaceFilter) Allows(ns string) bool {
	for _, e := range n.Exclude {
		if e == ns {
			return false
		}
	}
	if len(n.Include) == 0 {
		return true
	}
	for _, i := range n.Include {
		if i == "*" || i == ns {
			return true
		}
	}
	return false
}

type Monitors struct {
	PodCrashes           PodCrashesMon       `yaml:"pod_crashes"`
	PodRestarts          PodRestartsMon      `yaml:"pod_restarts"`
	WarningEvents        WarningEventsMon    `yaml:"warning_events"`
	PVCUsage             PVCUsageMon         `yaml:"pvc_usage"`
	NodeConditions       NodeConditionsMon   `yaml:"node_conditions"`
	NodeDisk             NodeDiskMon         `yaml:"node_disk"`
	TLSCertExpiry        TLSCertExpiryMon    `yaml:"tls_cert_expiry"`
	RolloutStuck         RolloutStuckMon     `yaml:"rollout_stuck"`
	JobFailed            JobFailedMon        `yaml:"job_failed"`
	CronJobMissed        CronJobMissedMon    `yaml:"cronjob_missed"`
	HPAAtMax             HPAAtMaxMon         `yaml:"hpa_at_max"`
	DaemonSetUnscheduled DaemonSetUnschedMon `yaml:"daemonset_unscheduled"`
}

type PodCrashesMon struct {
	Enabled           bool     `yaml:"enabled"`
	Reasons           []string `yaml:"reasons"`
	IncludeRecentLogs bool     `yaml:"include_recent_logs"`
	MaxLogLines       int      `yaml:"max_log_lines"`
}
type PodRestartsMon struct {
	Enabled   bool          `yaml:"enabled"`
	Threshold int           `yaml:"threshold"`
	Window    time.Duration `yaml:"window"`
}
type WarningEventsMon struct {
	Enabled       bool     `yaml:"enabled"`
	ReasonsIgnore []string `yaml:"reasons_ignore"`
}
type PVCUsageMon struct {
	Enabled  bool          `yaml:"enabled"`
	WarnAt   float64       `yaml:"warn_at"`
	CritAt   float64       `yaml:"crit_at"`
	Interval time.Duration `yaml:"interval"`
}
type NodeConditionsMon struct {
	Enabled bool     `yaml:"enabled"`
	AlertOn []string `yaml:"alert_on"`
}
type NodeDiskMon struct {
	Enabled  bool          `yaml:"enabled"`
	WarnAt   float64       `yaml:"warn_at"`
	CritAt   float64       `yaml:"crit_at"`
	Interval time.Duration `yaml:"interval"`
}
type TLSCertExpiryMon struct {
	Enabled  bool          `yaml:"enabled"`
	WarnDays int           `yaml:"warn_days"`
	CritDays int           `yaml:"crit_days"`
	Interval time.Duration `yaml:"interval"`
}
type RolloutStuckMon struct {
	Enabled   bool          `yaml:"enabled"`
	Threshold time.Duration `yaml:"threshold"`
}
type JobFailedMon struct {
	Enabled bool `yaml:"enabled"`
}
type CronJobMissedMon struct {
	Enabled       bool `yaml:"enabled"`
	MissThreshold int  `yaml:"miss_threshold"`
}
type HPAAtMaxMon struct {
	Enabled  bool          `yaml:"enabled"`
	Duration time.Duration `yaml:"duration"`
}
type DaemonSetUnschedMon struct {
	Enabled   bool          `yaml:"enabled"`
	Threshold time.Duration `yaml:"threshold"`
}

type Dedupe struct {
	Window time.Duration `yaml:"window"`
	Digest Digest        `yaml:"digest"`
}
type Digest struct {
	Enabled    bool          `yaml:"enabled"`
	Interval   time.Duration `yaml:"interval"`
	Severities []string      `yaml:"severities"`
}

type RoutingRule struct {
	Match    RoutingMatch `yaml:"match"`
	Channels []string     `yaml:"channels"`
}
type RoutingMatch struct {
	Severity string `yaml:"severity"`
	Monitor  string `yaml:"monitor"`
}

func Parse(b []byte) (*Config, error) {
	cfg := defaults()
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	cfg.fillZeroDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Namespaces: NamespaceFilter{Include: []string{"*"}, Exclude: []string{"kube-system", "kube-public", "kpulse"}},
		Monitors: Monitors{
			PodCrashes: PodCrashesMon{
				Enabled:           true,
				Reasons:           []string{"CrashLoopBackOff", "OOMKilled", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError", "FailedScheduling", "FailedMount", "Evicted"},
				IncludeRecentLogs: true, MaxLogLines: 50,
			},
			PodRestarts: PodRestartsMon{Enabled: true, Threshold: 5, Window: 15 * time.Minute},
			WarningEvents: WarningEventsMon{Enabled: true, ReasonsIgnore: []string{
				// Probe flap noise.
				"FailedGracefulShutdown", "Unhealthy",
				// Owned by pod_crashes; would dupe-alert.
				"Failed", "BackOff",
				// Owned by job_failed.
				"BackoffLimitExceeded",
				// k3d/Docker Desktop kubelet quirk; not actionable.
				"InvalidDiskCapacity",
			}},
			PVCUsage:             PVCUsageMon{Enabled: true, WarnAt: 80, CritAt: 90, Interval: 10 * time.Minute},
			NodeConditions:       NodeConditionsMon{Enabled: true, AlertOn: []string{"DiskPressure", "MemoryPressure", "PIDPressure", "NotReady"}},
			NodeDisk:             NodeDiskMon{Enabled: true, WarnAt: 85, CritAt: 92, Interval: 10 * time.Minute},
			TLSCertExpiry:        TLSCertExpiryMon{Enabled: true, WarnDays: 14, CritDays: 3, Interval: 6 * time.Hour},
			RolloutStuck:         RolloutStuckMon{Enabled: true, Threshold: 15 * time.Minute},
			JobFailed:            JobFailedMon{Enabled: true},
			CronJobMissed:        CronJobMissedMon{Enabled: true, MissThreshold: 2},
			HPAAtMax:             HPAAtMaxMon{Enabled: true, Duration: 30 * time.Minute},
			DaemonSetUnscheduled: DaemonSetUnschedMon{Enabled: true, Threshold: 10 * time.Minute},
		},
		Dedupe:     Dedupe{Window: 30 * time.Minute, Digest: Digest{Enabled: true, Interval: 10 * time.Minute, Severities: []string{"info", "warning"}}},
		Resolution: Resolution{Enabled: true},
	}
}

func (c *Config) fillZeroDefaults() {
	if c.Dedupe.Window == 0 {
		c.Dedupe.Window = 30 * time.Minute
	}
	if c.Monitors.PVCUsage.Interval == 0 {
		c.Monitors.PVCUsage.Interval = 10 * time.Minute
	}
	if c.Monitors.NodeDisk.Interval == 0 {
		c.Monitors.NodeDisk.Interval = 10 * time.Minute
	}
	if c.Monitors.TLSCertExpiry.Interval == 0 {
		c.Monitors.TLSCertExpiry.Interval = 6 * time.Hour
	}
}

func (c *Config) validate() error {
	if c.Cluster.Name == "" {
		return fmt.Errorf("cluster.name is required")
	}
	if c.Monitors.PVCUsage.WarnAt > c.Monitors.PVCUsage.CritAt {
		return fmt.Errorf("pvc_usage warn_at (%.0f) must be <= crit_at (%.0f)", c.Monitors.PVCUsage.WarnAt, c.Monitors.PVCUsage.CritAt)
	}
	return nil
}
