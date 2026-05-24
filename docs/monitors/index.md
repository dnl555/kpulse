# Monitors

kpulse ships 12 monitors, all enabled by default. Each is a key under `monitors:` in the ConfigMap. Set `enabled: false` to turn one off.

| Monitor | Triggers | Severity | Page |
|---|---|---|---|
| `pod_crashes` | CrashLoopBackOff, OOMKilled, ImagePullBackOff, etc. | critical | [Pod Crashes & Events](pods.md) |
| `pod_restarts` | > 5 restarts in 15 min | warning | [Pod Crashes & Events](pods.md) |
| `warning_events` | Warning-type k8s Events | info | [Pod Crashes & Events](pods.md) |
| `pvc_usage` | PVC > 80% warn, > 90% crit | warn/crit | [PVC Monitoring](pvc.md) |
| `node_conditions` | DiskPressure, MemoryPressure, PIDPressure, NotReady | critical | [Node Monitoring](node.md) |
| `node_disk` | rootfs/imagefs > 85% warn, > 92% crit | warn/crit | [Node Monitoring](node.md) |
| `tls_cert_expiry` | < 14 d warn, < 3 d crit | warn/crit | [TLS Cert Expiry](tls.md) |
| `rollout_stuck` | Deployment/StatefulSet stuck > 15 min | warning | [Workload Health](workload.md) |
| `job_failed` | Job condition `Failed=True` | warning | [Workload Health](workload.md) |
| `cronjob_missed` | > 2 missed schedules | warning | [Workload Health](workload.md) |
| `hpa_at_max` | HPA at maxReplicas > 30 min | warning | [Workload Health](workload.md) |
| `daemonset_unscheduled` | desired != ready > 10 min | warning | [Workload Health](workload.md) |

## Tuning rule of thumb

The defaults are calibrated to be quiet on a healthy cluster.

- **Getting spammed?** Raise the threshold, or raise `dedupe.window`, or add the noisy `reason` to `warning_events.reasons_ignore`.
- **Something broke and kpulse stayed silent?** Lower the threshold. Check if the namespace is in `namespaces.exclude`.
- **Want everything in one daily summary?** Set `dedupe.digest.interval: 24h` and add `critical` to `dedupe.digest.severities` (default excludes it so critical bypasses the digest).

## Disabling a monitor

```yaml
monitors:
  warning_events:
    enabled: false
```

Then `kubectl -n kpulse rollout restart deploy/kpulse`.
