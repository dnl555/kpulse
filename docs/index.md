---
title: kpulse
description: Monitor and detect failures in your Kubernetes cluster instantly.
---

# Monitor and detect failures in your Kubernetes cluster instantly

**kpulse** is event-driven Kubernetes monitoring for developers and startups on their first cluster. One Pod, one ConfigMap, ~64 Mi of memory. Slack / email / webhook out of the box.

It is **not** a Prometheus replacement and does not store metrics. It watches the cluster, catches the failure modes that wake teams up at night (pod crashes, PVC full, certs expiring, rollouts stuck), and pings the channel of your choice. Outgrow it later by adding Prometheus alongside, not instead.

## 30-second install

```bash
curl -fsSL https://kpulse.io/install.sh | bash
kubectl -n kpulse edit secret kpulse-secrets    # add SLACK_WEBHOOK_URL
kubectl -n kpulse edit configmap kpulse-config  # set cluster.name, uncomment slack
kubectl -n kpulse rollout restart deploy/kpulse
```

That's it. 12 monitors are enabled by default and start firing alerts to Slack.

[Get started :material-arrow-right:](getting-started/installation.md){ .md-button .md-button--primary }
[Browse monitors :material-arrow-right:](monitors/index.md){ .md-button }

## Why kpulse

- **Day-1 ready.** Install, paste one Slack webhook, you have alerts on the 12 most common failure modes.
- **No time-series stack required.** No Prometheus, no Grafana, no Alertmanager, no PVCs.
- **Sane defaults.** All 12 monitors enabled, thresholds tuned to be quiet on a healthy cluster.
- **Tiny.** Single Go binary, distroless image, ~64 Mi memory request.
- **MIT licensed.** Use it, fork it, ship it.

## What kpulse is NOT

- Not a metrics store (no time series, no PromQL)
- Not a dashboard (no UI in v1)
- Not an Alertmanager replacement (no silencing rules, no on-call rotations)

If you need any of those, run Prometheus + Grafana + Alertmanager. kpulse covers the gap before you're ready for that stack and keeps doing the noisy "did Kubernetes break again" work after.

## Contact

Questions, bug reports off-GitHub, or anything else: [contact@kpulse.io](mailto:contact@kpulse.io).
