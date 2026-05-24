---
title: The 12 default monitors, and why those thresholds
description: A reasoned tour of every default kpulse ships with, with the rationale behind each threshold.
---

# The 12 default monitors, and why those thresholds

Every alerting tool ships with defaults, and most of the time nobody reads the documentation for them. They are either too loud (every healthy cluster lights up the chat) or too quiet (real problems pass silently). kpulse tries hard to ship defaults that are useful on day one without needing tuning.

Here is what each of the 12 enabled-by-default monitors does, and the reasoning behind every threshold.

## pod_crashes (critical)

Fires when a container's waiting or terminated reason is in a specific list: `CrashLoopBackOff`, `OOMKilled`, `ImagePullBackOff`, `ErrImagePull`, `CreateContainerConfigError`, `FailedScheduling`, `FailedMount`, `Evicted`.

These are not arbitrary. They are the eight reasons that, in practice, account for the overwhelming majority of "this Pod is broken, not just slow" cases. They are also the reasons where the *next* action is almost always a human looking at logs. `Pending` is not in the list because Pending is usually transient. `BackOff` is not in the list because it is owned by the same root cause as `CrashLoopBackOff` and `pod_crashes` would double-fire.

## pod_restarts (warning, > 5 in 15 minutes)

Counts container restarts over a sliding window. The 5/15m threshold catches a Pod that is flapping fast enough to disrupt traffic but not fast enough to trip `CrashLoopBackOff`. A pod with three restarts an hour is probably fine. Five in fifteen minutes is not.

This monitor sits between `pod_crashes` (which catches hard breakage) and `warning_events` (which catches everything else weird). It is the monitor that catches things like a liveness probe that occasionally fails under load.

## warning_events (info, k8s Warning events)

Forwards every Kubernetes `Warning`-type Event whose `reason` is not in an ignore list. This is the "everything else weird" channel.

Default ignore list:

- `FailedGracefulShutdown` and `Unhealthy` (probe flap noise)
- `Failed`, `BackOff`, `BackoffLimitExceeded` (already covered by `pod_crashes` and `job_failed`; including them here would double-fire)
- `InvalidDiskCapacity` (k3d / Docker Desktop quirk where the kubelet can not stat the host-backed image filesystem)

If you run kpulse on a real cloud cluster the InvalidDiskCapacity exclusion is harmless. If you run it on k3d locally, removing it would mean a constant info alert about your kubelet's confused view of disk capacity.

## pvc_usage (warn at 80%, crit at 90%, every 10 minutes)

Walks every Node's `/stats/summary` endpoint and computes used/capacity per PersistentVolumeClaim. The 80/90 thresholds are conservative. You probably do not want to wait until 95% to know something is filling up, because by then you might already be in a corner you can not get out of without resizing the volume.

The 10-minute scan interval is a trade-off. Faster scans give you earlier alerts but produce more load on the kubelet proxy endpoint, which is not designed to be hit constantly. Ten minutes is enough lead time for any reasonable PVC growth rate.

## node_conditions (critical)

Fires when a Node condition you care about flips to True (or to False, in the case of Ready). Defaults: `DiskPressure`, `MemoryPressure`, `PIDPressure`, `NotReady`.

These four are the conditions the kubelet itself decides are serious. There is no threshold to tune here &mdash; the kubelet has already made the decision. kpulse just forwards it. You will hear about this within seconds of it happening, because it is informer-based, not periodic.

## node_disk (warn 85%, crit 92%, every 10 minutes)

Same data source as `pvc_usage`, but for the Node's own filesystem and the container runtime's image filesystem. The thresholds are slightly higher than PVC because Node disk usage usually fluctuates more (image pulls, log rotation) and 80% on a node is often normal.

92% as critical is chosen to fire before the kubelet itself decides to start evicting Pods, which it does around `nodefs.available<10%` by default. You want the warning before the eviction.

## tls_cert_expiry (warn 14 days, crit 3 days, every 6 hours)

Lists every Secret of type `kubernetes.io/tls`, parses the leaf certificate, and computes days until `NotAfter`. The 14-day warning is two weeks &mdash; enough time to renew a cert-manager certificate before anything user-visible happens. The 3-day critical is the "if cert-manager is broken, you have to act today" tier.

Six hours as the scan interval is a compromise. Faster scans add load and there is no value in finding out about cert expiry every minute &mdash; the situation does not change second-to-second.

## rollout_stuck (warning, > 15 minutes)

Fires when a Deployment's `Progressing` condition has been not-True for 15 minutes, or when a StatefulSet's `readyReplicas < replicas` for the same duration. Fifteen minutes is long enough to filter out normal rolling updates (which typically complete in 1-3 minutes) and short enough to catch a real problem before someone notices it manually.

## job_failed (warning)

Fires when a batch Job hits `Failed=True`. There is no threshold &mdash; if the Job's `backoffLimit` has been exceeded, you want to know. This includes Helm hook Jobs, migration Jobs, scheduled batch.

## cronjob_missed (warning, > 2 missed schedules)

Scans every CronJob every minute, parses the schedule expression, and counts how many runs were skipped since the last successful schedule. Two missed runs is the threshold because one missed run is sometimes legitimate (controller restart, brief unavailability) and two is usually not.

For a `*/5 * * * *` job, this means you find out about a stuck CronJob within about ten minutes. For an `@daily` job, you find out within about two days. The threshold is in *runs missed*, not wall-clock time, which is the right shape for this.

## hpa_at_max (warning, > 30 minutes)

Tracks per-HPA: when `currentReplicas == maxReplicas` continuously for thirty minutes or more, fires. Thirty minutes filters out legitimate burst traffic that scales to max for a few minutes. Sustained max usage means you almost certainly need to raise `maxReplicas`, scale the underlying nodes, or have a bug.

## daemonset_unscheduled (warning, > 10 minutes)

Per-DaemonSet: when `desiredNumberScheduled != numberReady` for ten minutes, fires. The ten-minute floor avoids false positives during rolling node upgrades, which routinely produce a brief mismatch. Persistent mismatch usually means a taint, a node selector mistake, or a pull failure.

## What we explicitly do not monitor

- **Pending pods.** Pending is usually transient. `FailedScheduling` (in `pod_crashes`) covers the real case.
- **Resource quotas.** kpulse does not know what your quota policy is.
- **Application-level health.** That is what your application's own monitoring should cover; kpulse is cluster-level only.
- **Cost.** No mechanism for it, and the cloud bill is not a cluster signal.

## Tuning the noise

The single best lever for "kpulse is too loud" is `dedupe.window`, which defaults to 30 minutes. Same alert within that window is suppressed. If you find yourself getting the same five alerts every half hour, raise it to an hour or two.

The second best lever is the per-monitor `enabled: false`. If you do not have HPAs, turn off `hpa_at_max`. If you do not use CronJobs, turn off `cronjob_missed`.

Finally, `dedupe.digest` batches all `info` and `warning` alerts into a single message every ten minutes (critical bypasses the digest). If your chat channel feels noisy on a busy day, the digest is doing most of the protecting.

[Full monitor reference &rarr;](../monitors/index.md)
