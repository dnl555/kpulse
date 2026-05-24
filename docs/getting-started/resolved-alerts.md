# Resolved Alerts

By default, kpulse sends a second notification when a previously firing alert clears. The message is prefixed `[RESOLVED]`, uses a green color, and reuses the same dedupe key as the original alert.

This applies to **state-based monitors** &mdash; the ones where "the condition no longer holds" is meaningful. Discrete one-shot monitors (warning events, pod restarts, job failed) do not resolve and are unaffected.

## Toggle

```yaml
resolution:
  enabled: true   # default
```

Set to `false` to keep the v0.1 behavior (fire only, never resolve).

## Which monitors resolve

| Monitor | Resolves on |
|---|---|
| `pod_crashes` | Container transitions to Running, or Pod deleted |
| `pvc_usage` | Usage drops back below `warn_at` |
| `node_disk` | Usage drops back below `warn_at` |
| `node_conditions` | Condition no longer reported True (or Ready=True for NotReady) |
| `tls_cert_expiry` | Cert renewed (NotAfter pushed past warn window) |
| `rollout_stuck` | Deployment `Progressing=True`, or StatefulSet ready==replicas |
| `hpa_at_max` | HPA scales below maxReplicas |
| `daemonset_unscheduled` | All desired pods ready |
| `cronjob_missed` | Next scheduled run lands |

## Which monitors do NOT resolve

- `warning_events` &mdash; events are point-in-time; "stopped firing" isn't a concept
- `pod_restarts` &mdash; a restart burst is a discrete event
- `job_failed` &mdash; Jobs don't unfail (a re-run would be a new Job)

## What a resolved alert looks like

| Channel | Resolved format |
|---|---|
| Slack | `:white_check_mark: *[RESOLVED]* [cluster] ...` |
| Email | green top bar, `[RESOLVED]` in subject, `X-Kpulse-State: resolved` header |
| Webhook | `"state": "resolved"` in JSON body |
| Discord | `[OK] **[RESOLVED]** [cluster] ...` |
| Teams | green theme color, `[RESOLVED]` in title |

## Behavior notes

- Resolved alerts **bypass the dedupe window** (always send) and **bypass the digest** (always immediate).
- The dedupe key matches the original alert: `(monitor, namespace, kind, name, reason)`. If the original alert never fired (or was suppressed by dedupe before reaching the engine), no resolved message is sent.
- The active-alert set is in-memory. If kpulse restarts, it won't know about alerts that were firing before the restart, so it won't send resolved messages for them. (The next scan from a periodic monitor will re-establish state and send a fresh `firing` alert if the condition still holds.)

## When to turn it off

- You forward everything into Alertmanager / PagerDuty and *those* tools already track alert state.
- You want strict "fire and forget" semantics for compliance.
