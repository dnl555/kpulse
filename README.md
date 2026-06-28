# kpulse

[![ci](https://github.com/dnl555/kpulse/actions/workflows/ci.yml/badge.svg)](https://github.com/dnl555/kpulse/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/dnl555/kpulse?sort=semver)](https://github.com/dnl555/kpulse/releases)
[![license](https://img.shields.io/github/license/dnl555/kpulse)](LICENSE)

> Event-driven Kubernetes monitoring for developers and startups on their first cluster.
> One Pod, one ConfigMap, ~64 Mi of memory. Slack / email / webhook out of the box.
> **Not** a Prometheus replacement. Web: [kpulse.io](https://kpulse.io)

kpulse watches the cluster, catches the failure modes that wake teams up at night (pod crashes, PVC full, certs expiring, rollouts stuck), and pings the channel of your choice. Outgrow it later by adding Prometheus alongside, not instead.

## TL;DR

```bash
curl -fsSL https://kpulse.io/install.sh | bash
kubectl -n kpulse edit secret kpulse-secrets   # add SLACK_WEBHOOK_URL
kubectl -n kpulse edit configmap kpulse-config # uncomment slack stanza, set cluster.name
kubectl -n kpulse rollout restart deploy/kpulse
```

You now have alerts on 12 common cluster failure modes flowing to Slack.

## Why kpulse

- **Day-1 ready.** Install, paste one Slack webhook, you have alerts on the 12 most common failure modes.
- **No time-series stack required.** No Prometheus, no Grafana, no Alertmanager, no PVCs.
- **Sane defaults.** All 12 monitors enabled, thresholds tuned to be quiet on a healthy cluster.

## Install

```bash
curl -fsSL https://kpulse.io/install.sh | bash
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

## Install options

| Method | Command |
|---|---|
| One-line script | `curl -fsSL https://kpulse.io/install.sh \| bash` |
| Raw manifest | `kubectl apply -f https://github.com/dnl555/kpulse/releases/latest/download/kpulse.yaml` |
| Helm | `helm install kpulse oci://ghcr.io/dnl555/charts/kpulse --namespace kpulse --create-namespace` |
| Local build | `git clone ... && make build image` |

## Contributing

Issues and PRs welcome. The codebase is intentionally small (~1500 LOC Go, 7 packages):

```
internal/
  alert/        Alert struct + Severity
  config/       ConfigMap parser + Secret resolver
  notifiers/    Slack / SMTP / webhook / Discord / Teams
  engine/       dedupe, routing, digest
  state/        ConfigMap-backed persistence
  monitors/     12 monitors (informer-based + periodic)
  httpsrv/      /healthz, /readyz, /metrics, /test-channel
cmd/kpulse/     main wiring
```

Run `make test` to test, `make build` to build the binary, `make image VERSION=dev` to build the image.

## Channels

Slack, SMTP email, generic webhook, Discord, Microsoft Teams. Pick any subset. Each goes through the same dedupe + digest engine.

See [docs/channels.md](docs/channels.md) for configuration and routing.

## Browser UI (opt-in)

v0.3.0 ships an embedded read-only UI. Off by default. Flip `ui.enabled: true` in `kpulse-config`, port-forward, and open it:

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080
# open http://localhost:8080
```

Active alerts, recent activity, per-monitor counters, channel test buttons. ~25 KB embedded in the binary, zero runtime dependencies. See [docs/getting-started/ui.md](docs/getting-started/ui.md).

## What kpulse is NOT

- **Not a metrics store.** No time series, no PromQL, no historical graphs.
- **Not a dashboard.** No UI in v1.
- **Not Alertmanager.** No silencing rules, no on-call schedules, no acknowledgements.

If you need any of those, run Prometheus + Grafana + Alertmanager. kpulse covers the gap before you're ready for that stack, and keeps doing the noisy "did Kubernetes break again" work after.

## Community

- [CONTRIBUTING.md](CONTRIBUTING.md) &mdash; how to file issues, propose features, send PRs, add monitors or channels.
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) &mdash; expectations for community interaction.
- Contact: [contact@kpulse.io](mailto:contact@kpulse.io)

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE) (kwatch is acknowledged as inspiration; no kwatch code is included).
