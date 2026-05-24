# High-Level Architecture

kpulse is intentionally one process. Everything below runs in a single Go binary inside a single Pod.

## Diagram

```
+-----------------------------------------------------------+
|                       kpulse Pod                          |
|                                                           |
|  +-------------+   +------------+   +------------------+  |
|  |  informers  |   |  checkers  |   |  http server     |  |
|  |  pods       |   |  pvc       |   |  /healthz        |  |
|  |  events     |   |  node_disk |   |  /readyz         |  |
|  |  nodes      |   |  tls_cert  |   |  /metrics        |  |
|  |  deploys    |   |  cronjob   |   |  /test-channel   |  |
|  |  hpas...    |   |  ...       |   +------------------+  |
|  +------+------+   +------+-----+                         |
|         |                  |                              |
|         v                  v                              |
|       +----------------------+                            |
|       |       engine         |                            |
|       |  dedupe + route +    |                            |
|       |  digest batching     |                            |
|       +----------+-----------+                            |
|                  |                                        |
|         +--------+--------+                               |
|         v        v        v                               |
|     +-------+ +-----+ +---------+                         |
|     | slack | |email| | webhook |  ... (5 sinks)          |
|     +-------+ +-----+ +---------+                         |
|                                                           |
|       +--------------------+                              |
|       | state snapshot     |    ----> ConfigMap/kpulse-   |
|       |   every 60s        |          state               |
|       +--------------------+                              |
+-----------------------------------------------------------+
              |
              v
       k8s API server
       (watches + periodic reads)
```

## Source layout

```
cmd/kpulse/                main wiring (config load, signal handling, goroutines)
internal/
  alert/                   Alert struct, Severity, dedupe key
  config/                  ConfigMap parser, SecretMap with $TOKEN expansion
  notifiers/               Notifier interface, 5 sinks, Registry, Build()
  engine/                  Deduper, Router, Engine with digest batching
  state/                   ConfigMap-backed dedupe persistence
  monitors/                12 monitor implementations
  httpsrv/                 /healthz, /readyz, /metrics, /test-channel
```

## Lifecycle of an alert

1. **Detection.** An informer event handler (e.g. pod_crashes seeing an OOMKilled container) or a periodic scan (e.g. pvc_usage finding a 95% PVC) builds an `alert.Alert{}` and calls `engine.Submit`.
2. **Dedupe.** The engine's `Deduper` checks if `(monitor, namespace, kind, name, reason)` fired within `dedupe.window`. If yes, drop.
3. **Digest decision.** If the severity is in `dedupe.digest.severities`, queue into the digest buffer. Otherwise dispatch immediately.
4. **Routing.** `Router.Channels(alert)` returns the channel list to use (first-match `routing` rule, or the default channel).
5. **Dispatch.** `Registry.Send(ctx, alert, channels)` calls each channel's `Notifier.Send` and aggregates errors.
6. **State.** Every 60 seconds the engine snapshots its dedupe map to `ConfigMap/kpulse-state`. On restart this is loaded so kpulse doesn't re-fire every existing condition.

## Why one process

- **Easier to run.** One Pod, one Deployment, one resource ask. A team running their first cluster does not need a sidecar diagram.
- **Easier to reason about.** Every alert flows through one engine. No queues, no sidecars, no shared state across processes.
- **Easier to test.** `client-go/kubernetes/fake` for informer-based monitors, an interface for the kubelet stats fetcher, a `smtpSender` interface for email. No integration test harness required.

## Why event-driven (not metrics)

Storing metrics is a big problem with a well-known solution (Prometheus + Grafana + Alertmanager). kpulse does not try to compete with that stack. Instead it consumes the same signals Kubernetes already produces (Events, Pod state, Node conditions, certificate `NotAfter` fields) and turns them into actionable human-readable notifications.

This means kpulse stays small (no time-series DB, no scraper config), starts giving value the instant it's installed, and never replaces Prometheus &mdash; when you eventually need Prometheus, kpulse keeps running alongside it.

## What kpulse does NOT do

- No metrics scraping (it exposes its own `/metrics` but doesn't pull from elsewhere)
- No persistent storage beyond the small `kpulse-state` ConfigMap
- No CRDs
- No multi-cluster fan-in (one kpulse per cluster)
- No alert acknowledgement / silencing UI (that's what Alertmanager is for, when you need it)
