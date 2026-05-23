# kpulse

Event-driven Kubernetes monitoring for developers and startups on their first cluster.

kpulse is not a Prometheus replacement and does not store metrics. It watches the cluster, catches the failure modes that wake teams up at night (pod crashes, PVC full, certs expiring, rollouts stuck), and pings Slack / email / webhook. One Pod, one ConfigMap, ~64 Mi of memory. Outgrow it later by adding Prometheus alongside, not instead.

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

Full docs: see [docs/monitors.md](docs/monitors.md) and [docs/channels.md](docs/channels.md).

## What kpulse is NOT

- Not a metrics store (no time series, no PromQL)
- Not a dashboard (no UI)
- Not an Alertmanager replacement (no silencing rules, no on-call rotations)

If you need any of those, run Prometheus + Grafana + Alertmanager. kpulse covers the gap before you're ready for that stack.

## License

MIT. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
