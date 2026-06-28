# Browser UI

kpulse v0.3.0 ships a small read-only browser UI embedded in the binary. Useful when you want a quick visual snapshot of what's firing, what's been firing recently, and what each monitor is configured to do, without parsing kubectl describe output or your Slack history.

## Enable it

Off by default. Flip one knob in `kpulse-config`:

```yaml
ui:
  enabled: true
```

Apply + restart:

```bash
kubectl -n kpulse rollout restart deploy/kpulse
```

Then port-forward and open it:

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080
# open http://localhost:8080
```

Helm equivalent:

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values --set ui.enabled=true
```

## What you see

- **Dashboard** — counts of active alerts by severity, 5 most recent active, 8 most recent events (firing + resolved).
- **Alerts** — full active set and the full in-memory recent ring buffer. Has a *Reset dedupe* button that calls `POST /reset-dedupe` from the browser.
- **Monitors** — all 12 built-in monitors with their enabled flag, per-monitor knobs, and live fires/resolves counters since process start.
- **Channels** — registered notification channels with a *Send test* button per channel (calls `GET /test-channel?name=...`).
- **About** — runtime config snapshot (cluster name, uptime, namespace filters, dedupe window, links to docs/source).

Dark theme by default; toggle to light with the sun icon in the bottom of the sidebar.

## What it does NOT do

- **No authentication.** The UI is gated only by the fact that kpulse listens on a ClusterIP service. Reach it via `kubectl port-forward` only; do not expose it through an Ingress without putting auth in front of it.
- **No write actions beyond `Reset dedupe` and `Send test`.** No silencing, no editing config from the UI, no acknowledgements.
- **No historical persistence.** The "Recent" feed is an in-memory ring buffer (last 100 events). It resets when the kpulse Pod restarts.
- **No graphs / metrics.** kpulse stays event-driven; for time-series alongside it, install Prometheus.

## JSON API

The UI is a thin client over a read-only JSON API. Use it directly from scripts if you want:

| Endpoint | Returns |
|---|---|
| `GET /api/v1/cluster` | Cluster name, kpulse version, uptime start, namespace filters, dedupe + digest + resolution settings |
| `GET /api/v1/alerts/active` | All alerts currently firing (in-memory active set) |
| `GET /api/v1/alerts/recent` | Recent ring buffer (last 100 fired + resolved alerts) |
| `GET /api/v1/monitors` | All 12 monitors with enabled flag, knobs, and per-monitor counters |
| `GET /api/v1/channels` | List of currently-registered notification channels |

These endpoints work whether or not `ui.enabled` is true. The UI assets at `/` and `/app.js` etc. only ship when `ui.enabled: true`.

## Resource impact

The UI adds about 25 KB to the kpulse binary (1 HTML + 1 CSS + 1 JS file, no external dependencies). Memory and CPU overhead at idle is unmeasurable; under load the bottleneck is whatever the API handlers do (which is read in-memory state and JSON-encode).
