# Workload Health

Five monitors cover higher-level workload objects: rollouts, jobs, cronjobs, autoscalers, daemonsets.

## rollout_stuck

`warning` when a Deployment's `Progressing` condition is not `True` and its `LastUpdateTime` is older than `threshold`. Also flags StatefulSets whose `readyReplicas < replicas` for longer than `threshold`.

```yaml
monitors:
  rollout_stuck:
    enabled: true
    threshold: 15m
```

Common causes: image pull failure, failing readiness probe, insufficient cluster capacity.

## job_failed

`warning` whenever a batch Job hits the `Failed=True` condition.

```yaml
monitors:
  job_failed:
    enabled: true
```

Fires once per Job (dedupe key includes the Job name). Helm hook jobs, migration jobs, scheduled batch all covered.

## cronjob_missed

Periodic scan (every 1m) that parses each CronJob's schedule with the standard cron parser and counts how many expected runs were skipped since `lastScheduleTime`.

```yaml
monitors:
  cronjob_missed:
    enabled: true
    miss_threshold: 2
```

Suspended CronJobs (`.spec.suspend: true`) are skipped.

Fires `warning` when missed runs reach `miss_threshold`. For a `*/5 * * * *` job with `miss_threshold: 2`, you'll be alerted ~10 minutes after the job stops running.

## hpa_at_max

Tracks per-HPA: when `currentReplicas == maxReplicas` continuously for >= `duration`, fires `warning`. Clears (resets the timer) when the HPA scales down.

```yaml
monitors:
  hpa_at_max:
    enabled: true
    duration: 30m
```

Signal that you probably need to raise `maxReplicas` or scale the underlying nodes.

## daemonset_unscheduled

Tracks per-DaemonSet: when `desiredNumberScheduled != numberReady` for longer than `threshold`, fires `warning`.

```yaml
monitors:
  daemonset_unscheduled:
    enabled: true
    threshold: 10m
```

Common causes: taint mismatch, pull failure, node selector mistake. The 10-minute floor avoids false positives during rolling node upgrades.
