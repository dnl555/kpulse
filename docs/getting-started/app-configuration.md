# App Configuration

This page covers the alert model, routing rules, and the test endpoints.

## Alert shape

Every monitor produces an `Alert` with the same fields:

| Field | Example | Notes |
|---|---|---|
| `Monitor` | `pod_crashes` | which monitor produced it |
| `Severity` | `info` / `warning` / `critical` | drives color, routing, digest behavior |
| `Cluster` | `prod-eks-1` | from `cluster.name` |
| `Namespace` | `checkout` | empty for cluster-scoped objects (Node, etc.) |
| `ObjectKind` | `Pod` | k8s kind |
| `ObjectName` | `api-7d9f` | k8s name |
| `Reason` | `OOMKilled` | machine key; also used in dedupe |
| `Title` | `OOMKilled on api-7d9f/server` | one-line summary |
| `Body` | longer text | extra context, sometimes log tails |
| `FiredAt` | RFC3339 UTC | when kpulse generated it |

Notifiers convert this into channel-specific formats (Slack blocks, HTML email, MessageCard for Teams, raw JSON for webhooks).

## Routing rules

Without rules, every alert goes to the default channel (Slack if `slack.default: true`, otherwise the first registered notifier). Add `routing` to send specific alerts to specific channels. First match wins.

```yaml
routing:
  # Critical -> both Slack and email
  - match: { severity: critical }
    channels: [slack, email]

  # Cert reminders -> email only (people read those at desk)
  - match: { monitor: tls_cert_expiry }
    channels: [email]

  # Everything else -> default
```

Match fields available:

- `severity`: `info`, `warning`, `critical`
- `monitor`: any of the 12 monitor names (`pod_crashes`, `pvc_usage`, etc.)

You can combine: `match: { severity: critical, monitor: pvc_usage }` matches only critical PVC alerts.

## Severity guide

| Severity | When monitors use it | Default behavior |
|---|---|---|
| `info` | Warning-type k8s Events, digest summary | batched into digest |
| `warning` | Things that need attention but aren't broken yet (PVC 80%, HPA at max, rollout stuck) | batched into digest |
| `critical` | Things that are actively broken (CrashLoopBackOff, OOMKilled, NotReady node, PVC 90%, cert < 3 days) | fires immediately |

## Testing channels

kpulse exposes `/test-channel?name=<channel>` on port 8080. It sends a synthetic `info` alert through the named channel.

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=slack'   # -> "sent"
curl 'http://localhost:8080/test-channel?name=email'
curl 'http://localhost:8080/test-channel?name=nope'    # -> 404 unknown channel
```

If you see the test message in your channel, credentials and connectivity are good.

## Health endpoints

| Path | Use |
|---|---|
| `/healthz` | Liveness probe target; always `200 ok` if the process is alive |
| `/readyz` | Readiness probe target; `200` once monitors have started |
| `/metrics` | Self-metrics (only kpulse's own counters; not a Prometheus scrape target for the cluster) |
| `/test-channel?name=...` | See above |

## Inspecting state

The dedupe map lives in `ConfigMap/kpulse-state` (key `dedupe.json`). To force kpulse to "forget" everything and re-fire all current alerts:

```bash
kubectl -n kpulse delete configmap kpulse-state
kubectl -n kpulse rollout restart deploy/kpulse
```
