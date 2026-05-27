# Slack

Standard Slack incoming webhook. The lowest-friction channel to set up.

## Get a webhook URL

1. Go to [api.slack.com/messaging/webhooks](https://api.slack.com/messaging/webhooks).
2. Create or pick a Slack app, enable Incoming Webhooks, add one for the target channel.
3. Copy the URL: `https://hooks.slack.com/services/T0XXX/B0XXX/xxxxxx`.

## Configure

ConfigMap:

```yaml
channels:
  slack:
    webhook_url_from_secret: SLACK_WEBHOOK_URL
    default: true
```

Secret:

```bash
kubectl -n kpulse edit secret kpulse-secrets
# under stringData add:
#   SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0/B0/xxx
```

Apply:

```bash
kubectl -n kpulse rollout restart deploy/kpulse
```

## Test

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=slack'
```

## Message format

What kpulse sends to Slack (raw):

```
:rotating_light: *[prod-eks-1]* `checkout/pod/api-7d9f` OOMKilled on api-7d9f/server
Container server in pod checkout/api-7d9f is in state OOMKilled
```

What Slack renders:

> 🚨 **[prod-eks-1]** `checkout/pod/api-7d9f` OOMKilled on api-7d9f/server
> Container server in pod checkout/api-7d9f is in state OOMKilled

Severity emoji:

| Severity | Slack code | Renders as |
|---|---|---|
| info | `:information_source:` | ℹ️ |
| warning | `:warning:` | ⚠️ |
| critical | `:rotating_light:` | 🚨 |

## Helm

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values \
  --set channels.slack.enabled=true \
  --set-string channels.secrets.SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
```
