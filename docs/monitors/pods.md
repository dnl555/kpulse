# Pod Crashes & Events

Three monitors cover pod-level health.

## pod_crashes

Fires `critical` whenever a container's waiting/terminated reason matches the configured list.

**Default reasons:** `CrashLoopBackOff`, `OOMKilled`, `ImagePullBackOff`, `ErrImagePull`, `CreateContainerConfigError`, `FailedScheduling`, `FailedMount`, `Evicted`.

```yaml
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
    include_recent_logs: true   # attach last 50 log lines (when supported by the notifier)
    max_log_lines: 50
```

**Trigger it on purpose:**

```bash
kubectl run oom --image=polinux/stress --restart=Never \
  --requests='memory=64Mi' --limits='memory=64Mi' -- \
  stress --vm 1 --vm-bytes 100M --vm-hang 30
```

The pod should OOMKill within seconds and kpulse should fire.

## pod_restarts

Fires `warning` when a container restarts more than `threshold` times within `window`.

```yaml
monitors:
  pod_restarts:
    enabled: true
    threshold: 5
    window: 15m
```

kpulse keeps a sliding window of restart counts per container. Useful for "this pod is flapping" cases that don't trip CrashLoopBackOff (e.g. liveness probe restarts).

## warning_events

Fires `info` for every `Warning`-type Event whose `reason` is not in `reasons_ignore`. The "everything else weird" catch-all.

```yaml
monitors:
  warning_events:
    enabled: true
    reasons_ignore:
      - FailedGracefulShutdown
      - Unhealthy
```

The default ignore list filters out two reasons that fire constantly during normal probe flaps. If you see other recurring noise, add the reason here.

Example reasons you might choose to ignore: `BackOff` (covered by `pod_crashes`), `FailedKillPod`, `Killing`.
