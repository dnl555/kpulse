# General Configuration

kpulse is driven by exactly two objects in the `kpulse` namespace:

- `ConfigMap/kpulse-config` &mdash; all tuning knobs
- `Secret/kpulse-secrets` &mdash; channel credentials

After editing either one, apply:

```bash
kubectl -n kpulse rollout restart deploy/kpulse
```

## ConfigMap shape

```yaml
cluster:
  name: prod-eks-1          # REQUIRED, shown in every alert title

channels:
  # see Channels section; each channel is opt-in

namespaces:
  include: ["*"]            # alert on all namespaces by default
  exclude: [kube-system, kube-public, kpulse]

monitors:
  # 12 monitor blocks, each with enabled + per-monitor knobs

dedupe:
  window: 30m               # suppress the same alert for this window
  digest:
    enabled: true
    interval: 10m
    severities: [info, warning]   # critical bypasses the digest

routing: []                 # optional, see App Configuration
```

The first install ships with the full file as comments &mdash; uncomment what you need.

## Required minimum

The only required field is `cluster.name`. Without channels, kpulse runs but silently drops alerts (you'll see a warning in the logs).

```yaml
cluster:
  name: prod-eks-1
channels:
  slack:
    webhook_url_from_secret: SLACK_WEBHOOK_URL
    default: true
```

Plus the matching secret:

```bash
kubectl -n kpulse edit secret kpulse-secrets
# under stringData add:
#   SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0/B0/xxx
```

## Namespace filtering

```yaml
namespaces:
  include: ["*"]
  exclude: [kube-system, kube-public, kpulse]
```

- `include: ["*"]` (default) means watch every namespace
- `include: [prod, staging]` restricts to just those
- `exclude: [foo]` removes a namespace from whatever `include` matches

Cluster-scoped resources (Nodes, ClusterRoles) ignore this filter.

## Dedupe

Every alert is keyed by `(monitor, namespace, kind, name, reason)`. Within `dedupe.window`, the same key fires once and the rest are suppressed. The default `30m` is a good fit for most teams: loud enough that you know something is wrong, quiet enough that one CrashLoopBackOff doesn't generate 200 messages.

State is persisted to `ConfigMap/kpulse-state` every 60 seconds and restored on Pod restart, so a kpulse restart does NOT re-fire every existing condition.

## Digests

Low-severity alerts (`info`, `warning` by default) are batched into a single "digest" message every `dedupe.digest.interval`. Critical alerts bypass the digest and fire immediately.

```yaml
dedupe:
  digest:
    enabled: true
    interval: 10m
    severities: [info, warning]
```

Turn off if you prefer one alert per event: `enabled: false`. Or raise to `1h` to be even quieter.

## Reloading config

There is no live reload. After editing the ConfigMap or Secret:

```bash
kubectl -n kpulse rollout restart deploy/kpulse
```

## Next

- [App Configuration](app-configuration.md) &mdash; routing rules and alert semantics
- [Channels](../channels/index.md) &mdash; wire up Slack, email, etc.
- [Monitors](../monitors/index.md) &mdash; tune individual monitors
