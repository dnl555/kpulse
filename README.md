# kpulse

Event-driven Kubernetes monitoring for developers and startups on their first cluster.

kpulse is **not** a Prometheus replacement and does not store metrics. It watches the cluster, catches the failure modes that wake teams up at night (pod crashes, PVC full, certs expiring, rollouts stuck), and pings Slack / email / webhook. One Pod, one ConfigMap, ~64 Mi of memory. Outgrow it later by adding Prometheus alongside, not instead.

## Why kpulse

- **Day-1 ready.** Install, paste one Slack webhook, you have alerts on the 12 most common failure modes.
- **No time-series stack required.** No Prometheus, no Grafana, no Alertmanager, no PVCs.
- **Sane defaults.** All 12 monitors enabled, thresholds tuned to be quiet on a healthy cluster.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/dnl555/kpulse/main/deploy/install.sh | bash
```

Then configure a channel:

```bash
kubectl -n kpulse edit configmap kpulse-config   # set cluster.name, enable a channel
kubectl -n kpulse edit secret kpulse-secrets     # add e.g. SLACK_WEBHOOK_URL
kubectl -n kpulse rollout restart deploy/kpulse
```

Test it:

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=slack'
```

## What it watches out of the box

| Monitor | Triggers | Severity |
|---|---|---|
| pod_crashes | CrashLoopBackOff, OOMKilled, ImagePullBackOff, etc. | critical |
| pod_restarts | > 5 restarts in 15 min | warning |
| warning_events | Warning-type k8s Events (with noisy reasons filtered) | info |
| pvc_usage | PVC > 80% warn, > 90% crit | warn/crit |
| node_conditions | DiskPressure, MemoryPressure, PIDPressure, NotReady | critical |
| node_disk | node rootfs/imagefs > 85% warn, > 92% crit | warn/crit |
| tls_cert_expiry | TLS Secret expiring in < 14 d warn, < 3 d crit | warn/crit |
| rollout_stuck | Deployment/StatefulSet rolling for > 15 min | warning |
| job_failed | Job condition `Failed=True` | warning |
| cronjob_missed | > 2 missed schedules | warning |
| hpa_at_max | HPA pinned at maxReplicas for > 30 min | warning |
| daemonset_unscheduled | desired != ready for > 10 min | warning |

Full details and how to deliberately trigger each one: [docs/monitors.md](docs/monitors.md).

## Channels

Slack, SMTP email, generic webhook, Discord, Microsoft Teams. Pick any subset. Each goes through the same dedupe + digest engine.

See [docs/channels.md](docs/channels.md) for configuration and routing.

## What kpulse is NOT

- **Not a metrics store.** No time series, no PromQL, no historical graphs.
- **Not a dashboard.** No UI in v1.
- **Not Alertmanager.** No silencing rules, no on-call schedules, no acknowledgements.

If you need any of those, run Prometheus + Grafana + Alertmanager. kpulse covers the gap before you're ready for that stack, and keeps doing the noisy "did Kubernetes break again" work after.

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE) (kwatch is acknowledged as inspiration; no kwatch code is included).
