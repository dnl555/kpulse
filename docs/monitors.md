# Monitors

kpulse ships 12 monitors, all enabled by default. Each one is a key under `monitors:` in `kpulse-config`. Set `enabled: false` to turn one off.

## Tuning rule of thumb

The defaults are calibrated to be quiet on a healthy cluster. If you get spammed, raise the threshold (or increase `dedupe.window`). If something broke and kpulse stayed silent, you probably want a lower threshold.

## The 12 monitors

### pod_crashes

Fires `critical` whenever a container's waiting/terminated reason matches the configured list.

Default reasons: `CrashLoopBackOff`, `OOMKilled`, `ImagePullBackOff`, `ErrImagePull`, `CreateContainerConfigError`, `FailedScheduling`, `FailedMount`, `Evicted`.

To trigger: `kubectl run oom --image=polinux/stress -- stress --vm 1 --vm-bytes 1G --vm-hang 1 --timeout 10s` with a 64Mi limit.

### pod_restarts

Fires `warning` when a container restarts more than `threshold` times within `window`.

Defaults: `threshold: 5`, `window: 15m`.

### warning_events

Fires `info` for every `Warning`-type Event whose `reason` is not in `reasons_ignore`. This is your "everything else weird" catch-all.

Default ignore list: `FailedGracefulShutdown`, `Unhealthy` (these fire constantly in healthy clusters with probe flaps).

### pvc_usage

Periodic scan (default every 10m) that pulls kubelet `stats/summary` for every node and computes used/capacity per PVC.

Defaults: `warn_at: 80`, `crit_at: 90`. Note: this requires the cluster to expose `nodes/proxy` (most managed clusters do).

### node_conditions

Fires `critical` when a Node condition you care about flips. Default watch list: `DiskPressure`, `MemoryPressure`, `PIDPressure`, `NotReady`.

### node_disk

Periodic scan (default every 10m). Same data source as pvc_usage. Fires when `rootfs` or `imagefs` cross threshold.

Defaults: `warn_at: 85`, `crit_at: 92`.

### tls_cert_expiry

Periodic scan (default every 6h). Lists all Secrets of type `kubernetes.io/tls`, parses `tls.crt`, computes days until `NotAfter`.

Defaults: `warn_days: 14`, `crit_days: 3`.

To trigger: create a TLS Secret with a cert expiring in <3 days (use `openssl req -x509 -newkey rsa:2048 -days 1 ...`).

### rollout_stuck

Fires `warning` when a Deployment `Progressing` condition is not `True` and its `LastUpdateTime` is older than `threshold`. Also flags StatefulSets whose `readyReplicas < replicas` for longer than `threshold`.

Default: `threshold: 15m`.

### job_failed

Fires `warning` whenever a Job hits `Failed=True`.

### cronjob_missed

Periodic scan (every 1m). Parses each CronJob's schedule with the standard cron parser and counts how many expected runs were skipped since `lastScheduleTime`.

Default: `miss_threshold: 2`.

### hpa_at_max

Tracks per-HPA: when `currentReplicas == maxReplicas` continuously for >= `duration`, fires `warning`. Clears when it scales down.

Default: `duration: 30m`.

### daemonset_unscheduled

Tracks per-DaemonSet: when `desiredNumberScheduled != numberReady` for longer than `threshold`, fires `warning`.

Default: `threshold: 10m`.

## Dedupe and digest

Every alert is keyed by `(monitor, namespace, kind, name, reason)` and suppressed if it fired within `dedupe.window` (default 30m). Low-severity alerts (`info`, `warning` by default) are batched into a single digest message every `dedupe.digest.interval` (default 10m). Critical alerts bypass the digest and fire immediately.

If you want a critical alert to retry sooner, lower `dedupe.window`. If digests feel too chatty, raise `dedupe.digest.interval` to 30m or 1h.
