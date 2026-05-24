---
title: "Resolved alerts: the missing half of alerting"
description: An alerting tool that tells you when something broke but not when it cleared is half a feature.
---

# Resolved alerts: the missing half of alerting

Until v0.2, kpulse only fired one way. Something broke, you got a message. Something fixed itself, you got nothing. Quietly broken alerts are the norm in the industry &mdash; pagers fire, on-call acks, the underlying problem self-recovers, the next pager fires from a different cause, and nobody ever knows that the first one is over.

It is not just a UX problem. It is a coverage problem. A monitoring tool that tells you "a thing is on fire" and never tells you "the fire is out" puts the entire burden of state on you. Your mental cache fills up with possibly-still-burning fires. You stop trusting the silence.

v0.2 fixed this. By default kpulse now sends a `[RESOLVED]` notification when a previously firing alert clears.

## What it looks like

In Slack, a fire alert is red:

> :rotating_light: *[prod-eks-1]* `checkout/pod/api-7d9f` OOMKilled on api-7d9f/server
> Container server in pod checkout/api-7d9f is in state OOMKilled

The resolution, when the same Pod gets back to Running (or is replaced by a healthy one), is green:

> :white_check_mark: *[RESOLVED] [prod-eks-1]* `checkout/pod/api-7d9f` api-7d9f/server is back to Running
> Container server in pod checkout/api-7d9f is now Running.

The dedupe key is the same on both sides &mdash; `(monitor, namespace, kind, name, reason)`. The resolution literally says "the thing I told you about is over."

In email, the entire banner flips from red to green and the subject gets a `[RESOLVED]` prefix:

```
Subject: [prod-eks-1] [RESOLVED] PVC checkout/data-api-0 back below threshold
X-Kpulse-State: resolved
```

(The `X-Kpulse-State` header is filter-friendly: a Gmail filter on `"X-Kpulse-State: resolved"` is a clean way to route resolutions out of your inbox without losing the fire alerts.)

## Which monitors resolve, and why

Not every alert has a meaningful resolution. A `Warning` event is a point-in-time fact &mdash; it does not "stop being true" later. A failed Job does not unfail. A discrete restart storm is over the moment the window passes; there is no second event to bind to.

So kpulse divides its monitors into two groups:

**Resolvable (8 monitors):** `pod_crashes`, `pvc_usage`, `node_disk`, `node_conditions`, `tls_cert_expiry`, `rollout_stuck`, `hpa_at_max`, `daemonset_unscheduled`, `cronjob_missed`. For each, "the condition no longer holds" is well-defined: the PVC dropped below 80%, the cert was renewed, the Pod transitioned to Running, the DaemonSet is fully scheduled. When kpulse detects the clearing, it emits `[RESOLVED]`.

**Not resolvable (3 monitors):** `warning_events` (events are point-in-time), `pod_restarts` (a restart burst is discrete), `job_failed` (Jobs do not unfail). These keep their v0.1 behavior &mdash; fire once, dedupe by window, never resolve.

There is a config knob: `resolution.enabled: true` is the default; set it to false to return to v0.1 behavior. The knob exists for two real cases. First, teams that pipe kpulse into Alertmanager already have state tracking in Alertmanager and do not want duplicate resolutions. Second, audit-heavy environments that prefer strict fire-only semantics.

## The implementation

For periodic monitors (PVC, node disk, TLS), the implementation is the obvious one. The monitor builds a complete set of currently-firing alerts on every scan and hands it to the engine via `Reconcile(monitor, firing[])`. The engine compares against its in-memory active set, sends the diff as resolutions, and updates the active set.

```go
func (p *PVCUsage) scan(ctx context.Context, sub Submitter) {
    var firing []alert.Alert
    for ref, u := range pvc {
        pct := float64(u.used) / float64(u.cap) * 100
        switch {
        case pct >= p.cfg.CritAt: firing = append(firing, ...)
        case pct >= p.cfg.WarnAt: firing = append(firing, ...)
        }
    }
    sub.Reconcile(p.Name(), firing)  // engine handles dedupe + resolution diff
}
```

For event-based monitors (`pod_crashes`, `rollout_stuck`, `node_conditions`), the monitor tracks its own "was firing" state per object key. When an informer update arrives, the monitor checks the current state and explicitly calls `sub.Resolve(...)` when an object was firing and no longer is.

The most interesting case is `pod_crashes` because Pods are mortal. A Pod might recover by becoming Running again (rare in CrashLoopBackOff cases) but more commonly the Deployment rolls and a fresh Pod replaces it. kpulse handles both: it resolves on Running transitions, and on Pod deletion it resolves any active alert keyed to that Pod's UID.

```go
func (m *PodCrashes) handleDelete(obj any, sub Submitter) {
    pod := obj.(*corev1.Pod)
    prefix := string(pod.UID) + "|"
    for k, v := range m.seen {
        if hasPrefix(k, prefix) {
            sub.Resolve(alert.Alert{...})  // pod is gone; alert is gone
            delete(m.seen, k)
        }
    }
}
```

## One honest limitation

The active-alert set is in-memory. If kpulse restarts, it forgets what was firing. It will not send `[RESOLVED]` messages for alerts that pre-date the restart. The next informer sync or periodic scan re-establishes state, and a new `firing` alert goes out if the condition still holds &mdash; so the world re-converges, just with a one-cycle gap.

You could ask: why not persist the active set to the same ConfigMap that backs dedupe state? It is a reasonable thing to do, and we will probably add it in a later release. The reason it is not there today is that we wanted to ship a clean v0.2 and live with the behavior for a while before adding write-amplification to the snapshot loop.

If your kpulse restarts often enough that this gap matters, that is a different bug.

## Why this is a default, not a flag

The historical pattern with alerting tools is to ship fire-only and let the operator opt into resolution. We picked the opposite: ship resolution-on, let the operator opt out.

The reasoning is that operators who *want* resolutions are people running operations, not people writing the tool. Asking them to discover a config knob to get a basic feature is the wrong default. People who explicitly do not want resolutions (because they have Alertmanager downstream) have a single line to add to their ConfigMap. The friction lives in the right place.

[Configuration reference &rarr;](../getting-started/resolved-alerts.md)
