# Node Monitoring

Two monitors cover node-level health: `node_conditions` (event-driven) and `node_disk` (periodic).

## node_conditions

Fires `critical` when a Node condition you care about flips. Default watch list: `DiskPressure`, `MemoryPressure`, `PIDPressure`, `NotReady`.

```yaml
monitors:
  node_conditions:
    enabled: true
    alert_on:
      - DiskPressure
      - MemoryPressure
      - PIDPressure
      - NotReady
```

Note: `NotReady` is matched by checking that the standard `Ready` condition is not `True`. All other names in `alert_on` map to a condition that is `True`.

Triggers from the kubelet are immediate; alerts fire as soon as the condition flips.

## node_disk

Periodic scan via the same kubelet `stats/summary` endpoint used by [pvc_usage](pvc.md). Looks at:

- `node.fs` &mdash; the node's root filesystem
- `node.runtime.imageFs` &mdash; the container runtime's image filesystem (often a separate volume on EKS / GKE)

```yaml
monitors:
  node_disk:
    enabled: true
    warn_at: 85         # percent
    crit_at: 92         # percent
    interval: 10m
```

Alert title format: `Node ip-10-0-1-23 rootfs at 88.4%`.

## Why both

`DiskPressure` (from `node_conditions`) tells you the kubelet has decided a node is in trouble. `node_disk` warns you before that point so you can act early (clear unused images, expand the volume).

## Requirements

Same as [pvc_usage](pvc.md): `get` on `nodes/proxy`. Granted by the bundled ClusterRole.
