# kpulse Helm chart

Event-driven Kubernetes monitoring. See the [project README](https://github.com/dnl555/kpulse) for what it does and why.

## Install

```bash
helm install kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --namespace kpulse --create-namespace \
  --set clusterName=prod-eks-1 \
  --set channels.slack.enabled=true \
  --set-string channels.secrets.SLACK_WEBHOOK_URL="https://hooks.slack.com/services/T0/B0/xxx"
```

## Common configurations

### Slack only

```yaml
clusterName: prod-eks-1
channels:
  slack:
    enabled: true
  secrets:
    SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0/B0/xxx
```

### Email (SMTP) only

```yaml
clusterName: prod-eks-1
channels:
  email:
    enabled: true
    smtpHost: smtp.fastmail.com
    smtpPort: 587
    from: alerts@example.com
    to: [oncall@example.com]
  secrets:
    SMTP_USER: alerts@example.com
    SMTP_PASS: hunter2
```

### Slack + Email with routing

```yaml
clusterName: prod-eks-1
channels:
  slack:
    enabled: true
    default: true
  email:
    enabled: true
    smtpHost: smtp.fastmail.com
    smtpPort: 587
    from: alerts@example.com
    to: [oncall@example.com]
  secrets:
    SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0/B0/xxx
    SMTP_USER: alerts@example.com
    SMTP_PASS: hunter2
routing:
  - match: { severity: critical }
    channels: [slack, email]
  - match: { monitor: tls_cert_expiry }
    channels: [email]
```

### Tighten a noisy monitor

```yaml
monitors:
  warning_events:
    enabled: false
  pod_restarts:
    threshold: 20
    window: 30m
```

## Values reference

See `values.yaml` for every knob (heavily commented). Selected highlights:

| Key | Default | Notes |
|---|---|---|
| `clusterName` | _required_ | Shown in every alert title |
| `image.repository` | `ghcr.io/dnl555/kpulse` | |
| `image.tag` | _chart appVersion_ | |
| `replicaCount` | `1` | Run a single instance; engine state is in-memory + ConfigMap |
| `channels.*.enabled` | `false` | Opt-in per channel |
| `channels.secrets` | `{}` | Map of secret name -> value, becomes Secret/kpulse-secrets |
| `monitors.*.enabled` | `true` | All 12 monitors on by default |
| `dedupe.window` | `30m` | Per-key suppression window |
| `dedupe.digest.enabled` | `true` | Batch info/warning into one message |
| `routing` | `[]` | First-match routing rules |

## Uninstall

```bash
helm uninstall kpulse -n kpulse
kubectl delete namespace kpulse
```
