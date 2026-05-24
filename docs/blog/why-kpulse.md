---
title: Why kpulse exists
description: The gap between "I just kubectl applied my first thing" and "I have Prometheus, Grafana, and Alertmanager on call".
---

# Why kpulse exists

There is a strange period in a startup's life. You have a Kubernetes cluster. You have one, maybe two services on it. Things mostly work. And there is no real monitoring.

You know you should fix this. You also know the standard answer: Prometheus, Grafana, Loki, Alertmanager, an exporter for each thing you care about, a few hundred lines of YAML, and a couple of evenings setting it all up. You will get there eventually. You will not get there today.

In the meantime, your cluster is silently telling you things. The frontend Pod has been crashing every six minutes for an hour. A PVC is at 96% full. The cert on your staging ingress expires tomorrow. None of this reaches you. It will reach you in the form of a customer message, or a coworker's screenshot, or your own confused 3am pager that turns out to have been preventable.

kpulse exists to cover that period. It is not a Prometheus replacement. It is what you install before you are ready for Prometheus, and what you keep running alongside Prometheus after.

## What it actually does

kpulse is a single Go binary that runs in a single Pod. It watches the Kubernetes API and runs a handful of periodic probes. When something looks bad, it sends a human-readable message to a channel of your choice: Slack, email, generic webhook, Discord, Microsoft Teams. When the thing stops looking bad, it sends a `[RESOLVED]` message.

No time series. No PromQL. No dashboards. No silencing rules. The whole thing fits in your head.

Install:

```bash
curl -fsSL https://kpulse.io/install.sh | bash
kubectl -n kpulse edit secret kpulse-secrets   # add SLACK_WEBHOOK_URL
kubectl -n kpulse edit configmap kpulse-config # enable slack, set cluster.name
kubectl -n kpulse rollout restart deploy/kpulse
```

That is the whole onboarding. Out of the box, twelve monitors are already enabled and tuned to be quiet on a healthy cluster: pod crashes, restart storms, PVC pressure, node disk and conditions, TLS certificate expiry, stuck rollouts, failed Jobs, missed CronJobs, HPAs pinned at max, DaemonSets that can not schedule, k8s Warning events. None of these need configuration for a small cluster.

## Why event-driven, not metrics

The reason kpulse is not a metrics tool is that storing metrics is a hard, well-solved problem. Prometheus already does it. The pretty default Grafana dashboards already exist. There is no value in re-implementing a worse Prometheus.

What is missing in the early-cluster window is not metric storage. It is *signals you actually act on*. Kubernetes already produces those signals, for free, every second of every day. Pod state, Node conditions, certificate `NotAfter` fields, CronJob `lastScheduleTime`. They sit in `kubectl describe` waiting for a human to look at them. kpulse just watches and shouts when one of them looks bad.

This means kpulse starts being useful the instant it is installed. There is no scrape config. There is no need to deploy exporters into your workloads. There is no PVC to provision for time-series storage. The whole image is around 14 MB. The Pod requests 25 millicores and 64 Mi of memory.

## What kpulse is NOT

It is worth being explicit:

- It does not store time series, so you can not graph throughput over time.
- It does not have a dashboard. There is nothing to log into.
- It does not have a silence UI, on-call rotations, or escalation policies. There is no acknowledgement model.
- It does not replace Alertmanager. If you have a real on-call rotation, you want Alertmanager.

The intended path is: install kpulse on day one. Live with it for months. When you get to the point where the engineering team is big enough that on-call has to be a thing, install Prometheus + Alertmanager *alongside* kpulse. Use Prometheus for SLO-grade alerts and graphs. Keep kpulse running for the daily noise it covers well: a CronJob that quietly stopped scheduling, a Secret that is about to expire, a Node that flipped to `NotReady` at 4am.

## When kpulse is the wrong answer

If you already have Prometheus and Alertmanager, kpulse is at best a duplicate of work you have already done. The pod-restart alert, the PVC-pressure alert, the cert-expiry alert &mdash; all of these have well-known PromQL idioms and Alertmanager routes.

If you operate dozens of clusters and need centralised alerting, kpulse is a per-cluster process and does not aggregate. There is no plan to add that. The right answer there is to push everything into a central tool you already operate.

If your alerting needs are "page me at 3am only if my checkout error rate is above 0.5% sustained for 10 minutes" &mdash; kpulse can not express that. It alerts on Kubernetes-level signals, not application metrics. That is what Prometheus is for.

## Where it goes next

The roadmap is intentionally narrow. The next pieces are: PagerDuty and Opsgenie bridges (built on top of the existing webhook contract), a small browser UI for inspecting recent alerts (read-only, no acknowledgements), and Helm chart polish. There is no plan to add metric scraping, dashboards, or anything that would push kpulse toward being a partial Prometheus.

The goal is to stay small, stay useful from day one, and stay the kind of thing you can `tilt up` and read end-to-end in an afternoon.

[See it in action: installation &rarr;](../getting-started/installation.md)
