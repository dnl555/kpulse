# Channels

kpulse v1 ships with five channels. Each one is opt-in: add a stanza under `channels:` in the ConfigMap and the matching key(s) under `stringData:` in the Secret.

| Channel | Page | Best for |
|---|---|---|
| Slack | [Slack](slack.md) | small teams, real-time |
| Email (SMTP) | [Email](email.md) | personal alerts, audit trail |
| Generic Webhook | [Generic Webhook](webhook.md) | piping into anything (Mattermost, custom endpoint) |
| Discord | [Discord](discord.md) | gaming/indie teams already on Discord |
| Microsoft Teams | [Microsoft Teams](teams.md) | enterprise on M365 |

## Default channel

If `channels.slack.default: true` and Slack is registered, Slack is the default. Otherwise the first registered notifier wins. The default channel is used whenever no [routing rule](../getting-started/app-configuration.md#routing-rules) matches.

## Multiple channels

You can enable as many as you like. Use [routing rules](../getting-started/app-configuration.md#routing-rules) to send specific alerts to specific channels. Example: critical to Slack + email, cert reminders to email only, everything else to Slack.

## Testing

Once a channel is configured, send a synthetic alert:

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=slack'
```

A test message should appear in your channel within a second.
