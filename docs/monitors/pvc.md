# PVC Monitoring

Watches PersistentVolumeClaim usage by polling kubelet's `/stats/summary` endpoint on every node.

```yaml
monitors:
  pvc_usage:
    enabled: true
    warn_at: 80         # percent
    crit_at: 90         # percent
    interval: 10m
```

## How it works

Every `interval`:

1. List all Nodes.
2. For each node, GET `/api/v1/nodes/<name>/proxy/stats/summary`.
3. Walk `pods[*].volume[*]` collecting `pvcRef`, `usedBytes`, `capacityBytes`.
4. For each PVC compute `pct = used / capacity * 100`.
5. Fire `warning` if `>= warn_at`, `critical` if `>= crit_at`.

Namespace filtering applies (`namespaces.exclude` won't be alerted on).

## Requirements

The kpulse ServiceAccount needs `get` on `nodes/proxy`. The bundled ClusterRole grants this. Some hardened clusters block the kubelet proxy; if so, PVC monitoring won't have data and will silently produce no alerts.

## Tuning

| Symptom | Fix |
|---|---|
| Too noisy at 80% on slowly-growing PVCs | Raise `warn_at` to 85 or 90 |
| Alerts arrive too late to act | Lower `warn_at` to 70 |
| Want critical only | Set `warn_at: 100` (effectively disables warn tier) |
| Once an hour is enough | `interval: 1h` |

## Trigger it on purpose

```bash
kubectl exec -it <pod-with-pvc> -- sh -c 'dd if=/dev/zero of=/data/big.bin bs=1M count=900'
```

(Adjust count to push past your `warn_at` threshold relative to the PVC capacity.)
