# Channels

kpulse v1 supports five channels. Each one is enabled by adding a stanza under `channels:` in `kpulse-config` and the matching secret key(s) to `Secret/kpulse-secrets`.

The default channel is whichever has `default: true` (Slack is the only one with this flag in the template). If none is marked default, the first registered notifier is used. Override per-alert via `routing`.

Add credentials with:

```bash
kubectl -n kpulse edit secret kpulse-secrets
# under stringData add e.g.:
#   SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0000/B0000/xxx
kubectl -n kpulse rollout restart deploy/kpulse
```

Test a channel without waiting for a real alert:

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=slack'
```

## slack

Standard Slack incoming webhook. Get the URL from `https://api.slack.com/messaging/webhooks`.

```yaml
channels:
  slack:
    webhook_url_from_secret: SLACK_WEBHOOK_URL
    default: true
```

Secret:

```yaml
stringData:
  SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T0000/B0000/xxx
```

## email

Plain SMTP with `PLAIN` auth over the host you point to. Tested with Fastmail, Gmail (app password), AWS SES.

```yaml
channels:
  email:
    smtp_host: smtp.fastmail.com
    smtp_port: 587
    from: alerts@example.com
    to: [oncall@example.com]
    user_from_secret: SMTP_USER
    pass_from_secret: SMTP_PASS
```

Secret:

```yaml
stringData:
  SMTP_USER: alerts@example.com
  SMTP_PASS: hunter2
```

## webhook

Generic JSON POST. Body shape:

```json
{
  "monitor": "pod_crashes",
  "severity": "critical",
  "cluster": "prod-eks-1",
  "namespace": "default",
  "object": "pod/foo",
  "reason": "OOMKilled",
  "title": "OOMKilled on foo/app",
  "body": "Container app in pod default/foo is in state OOMKilled",
  "fired_at": "2026-05-23T18:42:10Z"
}
```

Headers may include secret references via `$TOKEN` expansion.

```yaml
channels:
  webhook:
    url: https://hooks.example.com/kpulse
    headers:
      Authorization: "Bearer $WEBHOOK_TOKEN"
      X-Source: kpulse
```

Secret:

```yaml
stringData:
  WEBHOOK_TOKEN: very-long-token
```

## discord

Standard Discord channel webhook (Server Settings -> Integrations -> Webhooks).

```yaml
channels:
  discord:
    webhook_url_from_secret: DISCORD_WEBHOOK_URL
```

Messages over 1900 chars are truncated to fit Discord's 2000-char content limit.

## teams

Microsoft Teams Incoming Webhook (configured per channel in Teams). Body is a MessageCard with severity-colored theme.

```yaml
channels:
  teams:
    webhook_url_from_secret: TEAMS_WEBHOOK_URL
```

## Routing

Add a `routing` list to send specific alerts to specific channels. Rules are first-match.

```yaml
routing:
  - match: { severity: critical }
    channels: [slack, email]      # critical goes to both
  - match: { monitor: tls_cert_expiry }
    channels: [email]             # cert reminders to email only
```

Without any routing rules, every alert goes to the default channel.
