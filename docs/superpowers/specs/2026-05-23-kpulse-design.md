# kpulse Design Spec

**Date:** 2026-05-23
**Status:** Approved (pending review of this written spec)
**Author:** Danilo Acquaviva

## 1. Purpose

kpulse is a single-binary, lightweight Kubernetes monitoring agent that ships sensible defaults out of the box and notifies on the failure modes operators actually care about (pod crashes, PVC pressure, node conditions, certificate expiry, stuck rollouts, etc.).

It is a fresh implementation, not a fork of kwatch. kwatch is acknowledged as inspiration (MIT) in the project NOTICE, but no kwatch code is reused.

Design goals, in priority order:

1. **One-command install.** `curl ... | bash` followed by editing one Secret + one ConfigMap.
2. **Useful from minute one.** All 12 built-in monitors enabled with defaults that are quiet on a healthy cluster and loud on a sick one.
3. **One process, one image.** No sidecars, no operators, no CRDs. ~20 MB distroless image.
4. **Pluggable channels.** Slack, email (SMTP), generic webhook, Discord, Teams in v1.
5. **Hands-off operation.** Dedupe + rate limit + optional digest so it does not spam.

Explicit non-goals for v1: web UI, persistent storage, Helm chart (added later), CRDs, PagerDuty/Opsgenie, scraping Prometheus.

## 2. Architecture

Single Go binary deployed as a single Deployment (1 replica) in namespace `kpulse`.

```
kpulse process
├── config/        loads + validates ConfigMap, resolves Secret refs, hot-reloads on SIGHUP
├── informers/     shared k8s informers for Pods, Events, Nodes, Deployments,
│                  StatefulSets, DaemonSets, Jobs, CronJobs, HPAs, PVCs, Secrets
├── checkers/      periodic scanners for things informers cannot detect:
│                  pvc_usage, tls_cert_expiry, node_disk, rollout_stuck,
│                  hpa_at_max, cronjob_missed, daemonset_unscheduled
├── engine/        Alert dedupe (key = monitor + namespace + object + reason),
│                  rate-limit, severity routing, optional digest batching
├── notifiers/     Slack webhook, SMTP, generic webhook, Discord, Teams.
│                  Each implements one interface: Send(ctx, Alert) error
├── state/         in-memory map of last-fired-at + counter per dedupe key;
│                  periodic snapshot to a ConfigMap so restarts do not re-spam
└── http/          :8080/healthz, :8080/readyz, :8080/metrics (self-only),
                   :8080/test-channel?name=slack for sanity checks
```

Concurrency model: one goroutine per informer event handler, one goroutine per periodic checker, one goroutine for the notifier dispatch queue. All work flows into a single channel of `Alert` structs that the engine consumes.

RBAC: ClusterRole with read-only verbs (`get`, `list`, `watch`) on every resource above. No write permissions anywhere except updating the engine state ConfigMap inside its own namespace.

## 3. Configuration

Two objects, both in the `kpulse` namespace:

- `Secret/kpulse-secrets` — channel credentials (SMTP user/pass, Slack webhook URL, etc.). Keys referenced by name from the ConfigMap.
- `ConfigMap/kpulse-config` — everything else. Heavily commented so editing it teaches the operator what each knob does.

Full ConfigMap shape:

```yaml
cluster:
  name: prod-eks-1                    # appears in every alert title

channels:
  slack:
    webhook_url_from_secret: SLACK_WEBHOOK_URL
    default: true
  email:
    smtp_host: smtp.fastmail.com
    smtp_port: 587
    from: alerts@example.com
    to: [me@example.com]
    user_from_secret: SMTP_USER
    pass_from_secret: SMTP_PASS
  webhook:
    url: https://hooks.example.com/kpulse
    headers: { Authorization: "Bearer $WEBHOOK_TOKEN" }
  discord:
    webhook_url_from_secret: DISCORD_WEBHOOK_URL
  teams:
    webhook_url_from_secret: TEAMS_WEBHOOK_URL

namespaces:
  include: ["*"]
  exclude: [kube-system, kube-public, kpulse]

monitors:
  pod_crashes:
    enabled: true
    reasons:
      - CrashLoopBackOff
      - OOMKilled
      - ImagePullBackOff
      - ErrImagePull
      - CreateContainerConfigError
      - FailedScheduling
      - FailedMount
      - Evicted
    include_recent_logs: true
    max_log_lines: 50
  pod_restarts:
    enabled: true
    threshold: 5
    window: 15m
  warning_events:
    enabled: true
    reasons_ignore: [FailedGracefulShutdown, Unhealthy]
  pvc_usage:
    enabled: true
    warn_at: 80
    crit_at: 90
    interval: 10m
  node_conditions:
    enabled: true
    alert_on: [DiskPressure, MemoryPressure, PIDPressure, NotReady]
  node_disk:
    enabled: true
    warn_at: 85
    crit_at: 92
    interval: 10m
  tls_cert_expiry:
    enabled: true
    warn_days: 14
    crit_days: 3
    interval: 6h
  rollout_stuck:
    enabled: true
    threshold: 15m
  job_failed:
    enabled: true
  cronjob_missed:
    enabled: true
    miss_threshold: 2
  hpa_at_max:
    enabled: true
    duration: 30m
  daemonset_unscheduled:
    enabled: true
    threshold: 10m

dedupe:
  window: 30m
  digest:
    enabled: true
    interval: 10m
    severities: [info, warning]      # critical always fires immediately

routing:
  - match: { severity: critical }
    channels: [slack, email]
  - match: { monitor: tls_cert_expiry }
    channels: [email]
```

Validation runs at startup; fatal config errors prevent the pod from going Ready (visible via `kubectl describe`) instead of silent partial enablement.

Hot reload: `kubectl rollout restart deploy/kpulse` is the documented path. A SIGHUP-based reload is nice-to-have, not required for v1.

## 4. Day-1 monitor preset

The 12 monitors below are all enabled by default. Thresholds are picked to be quiet on a healthy cluster and to alert before things become incidents.

| # | Monitor | Trigger | Severity | Source |
|---|---|---|---|---|
| 1 | pod_crashes | Pod status reason matches list | critical | Pod informer |
| 2 | pod_restarts | container restart delta > N in window | warning | Pod informer |
| 3 | warning_events | k8s Event type=Warning, reason not in ignore list | info | Event informer |
| 4 | pvc_usage | PVC used/capacity > warn/crit threshold | warn/crit | kubelet stats/summary (periodic) |
| 5 | node_conditions | Node condition True for {DiskPressure, MemoryPressure, PIDPressure} or NotReady | critical | Node informer |
| 6 | node_disk | node rootfs/imagefs > warn/crit | warn/crit | kubelet stats/summary (periodic) |
| 7 | tls_cert_expiry | any `kubernetes.io/tls` Secret expiring in < warn/crit days | warn/crit | Secret informer + periodic parse |
| 8 | rollout_stuck | Deployment/StatefulSet `Progressing=False` OR Progressing for > threshold | warning | Deployment/StatefulSet informer |
| 9 | job_failed | Job condition `Failed=True` | warning | Job informer |
| 10 | cronjob_missed | `.status.lastScheduleTime` older than schedule by > miss_threshold cycles | warning | CronJob informer (periodic check) |
| 11 | hpa_at_max | `currentReplicas == maxReplicas` continuously > duration | warning | HPA informer |
| 12 | daemonset_unscheduled | `desiredNumberScheduled != numberReady` for > threshold | warning | DaemonSet informer |

PVC checker (#4) is a direct port of the working Python CronJob captured in `~/devel/kevra/monitoring/_cluster-snapshot/cm_kwatch-pvc-checker-script.yaml`, rewritten in Go and embedded in the main binary as a periodic task.

## 5. Alert lifecycle

```
event/scan -> Alert{key, severity, title, body, attachments?}
           -> engine.dedupe(window)              # suppress repeats inside window
           -> engine.route(rules)                # pick channels
           -> if digestable -> digest queue      # batch low-severity
              else          -> immediate fanout
           -> notifier.Send(ctx, alert)          # parallel per channel, with retry
           -> state.record(key, time.Now())
```

Dedupe key: `sha1(monitor || namespace || objectKind || objectName || reason)`. Window is per-monitor configurable, default 30 m.

Digest: every `dedupe.digest.interval`, all info/warning alerts queued since the last flush are sent as one message per channel ("8 alerts in the last 10 min: ..."). Critical bypasses digest.

State persistence: every 60 s the engine writes its dedupe map to `ConfigMap/kpulse-state`. On startup it reads that ConfigMap and rehydrates, so a pod restart does not re-fire every existing condition.

## 6. Notifier interface

```go
type Notifier interface {
    Name() string
    Send(ctx context.Context, a Alert) error
}

type Alert struct {
    Key         string
    Severity    Severity   // info | warning | critical
    Monitor     string     // "pod_crashes" etc.
    Cluster     string
    Namespace   string
    Object      string     // "deploy/foo" or "pod/bar"
    Title       string
    Body        string     // markdown; notifier converts as needed
    Attachments []Attachment
    FiredAt     time.Time
}
```

Each notifier owns its own formatting (Slack blocks, HTML email, Discord embeds, Teams MessageCard, plain JSON for generic webhook). Retry policy: 3 attempts with exponential backoff (1 s, 4 s, 16 s); on final failure, log and increment a `kpulse_notifier_failures_total` self-metric.

## 7. Install UX

`deploy/install.sh` is intentionally minimal:

```bash
#!/usr/bin/env sh
set -e
VERSION="${KPULSE_VERSION:-latest}"
URL="https://github.com/<owner>/kpulse/releases/${VERSION}/download/kpulse.yaml"
[ "$VERSION" = "latest" ] && URL="https://github.com/<owner>/kpulse/releases/latest/download/kpulse.yaml"
echo "Installing kpulse ($VERSION)..."
kubectl apply -f "$URL"
echo
echo "✓ kpulse installed in namespace 'kpulse'."
echo "Next steps:"
echo "  1. Add channel credentials:  kubectl -n kpulse edit secret kpulse-secrets"
echo "  2. Tune monitors/channels:    kubectl -n kpulse edit configmap kpulse-config"
echo "  3. Apply changes:             kubectl -n kpulse rollout restart deploy/kpulse"
echo "  4. Test a channel:            kubectl -n kpulse port-forward svc/kpulse 8080:8080 \\"
echo "                                  && curl 'http://localhost:8080/test-channel?name=slack'"
```

`deploy/kpulse.yaml` is a single file (Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding, ConfigMap with full defaults + comments, Secret stub, Deployment, Service). Target: under 300 lines, all readable, no magic.

A GitHub Actions release workflow builds the image, renders `kpulse.yaml` with the pinned image tag, and attaches both `kpulse.yaml` and `install.sh` to the release.

## 8. Repository layout

```
kpulse/
├── README.md
├── LICENSE                       # MIT
├── NOTICE                        # acknowledges kwatch as inspiration
├── cmd/kpulse/main.go
├── internal/
│   ├── config/                   # loader, validator, secret resolution
│   ├── informers/                # shared informer factory + wiring
│   ├── checkers/                 # pvc, tls, node_disk, rollout_stuck, ...
│   ├── engine/                   # dedupe, rate-limit, routing, digest
│   ├── notifiers/                # slack, email, webhook, discord, teams
│   ├── alert/                    # Alert, Severity, formatting helpers
│   ├── state/                    # in-memory + ConfigMap snapshot
│   └── http/                     # /healthz, /readyz, /metrics, /test-channel
├── deploy/
│   ├── kpulse.yaml
│   └── install.sh
├── docs/
│   ├── monitors.md
│   ├── channels.md
│   └── superpowers/specs/        # design specs (this file)
├── Dockerfile                    # multi-stage: build on golang:1.23, final distroless/static
├── .github/workflows/
│   ├── ci.yml
│   └── release.yml
├── .golangci.yml
├── Makefile                      # build, test, lint, image, render-yaml
└── go.mod
```

## 9. Testing strategy

- **Unit tests** on engine (dedupe windows, routing rules, digest batching), each notifier (mocked HTTP / SMTP), each checker against client-go fake clientset.
- **Integration test** using `envtest` (real apiserver, no kubelet) for the pod-crash -> notifier path; verifies dedupe + routing end-to-end.
- **Manual smoke test** matrix documented in `docs/monitors.md`: how to deliberately trigger each monitor (e.g. apply a Pod that OOMs, fill a PVC, create a Secret with an expired cert) on a dev cluster.
- **CI**: `go test ./...`, `golangci-lint run`, image build on every PR; release pipeline tags + publishes image + manifest.

## 10. Out of scope for v1

- Web UI / dashboard (separate landing-page work, deferred)
- Persistent storage (ConfigMap snapshot of dedupe state is sufficient)
- Helm chart (will add post-v1)
- CRDs / operator pattern
- PagerDuty, Opsgenie, VictorOps
- Prometheus scraping (kpulse exposes its own self-metrics but does not pull from anywhere)
- Multi-cluster federation
- Alert acknowledgement / on-call rotation logic
