# Generic Webhook

POSTs each alert as JSON to a URL you control. Use this to pipe alerts into Mattermost, a custom endpoint, a Lambda, or whatever you want.

## Configure

ConfigMap:

```yaml
channels:
  webhook:
    url: https://hooks.example.com/kpulse
    headers:
      Authorization: "Bearer $WEBHOOK_TOKEN"   # $TOKEN expands from the Secret
      X-Source: kpulse
```

Secret (only if you use `$TOKEN` expansion):

```yaml
stringData:
  WEBHOOK_TOKEN: very-long-token
```

## Payload

```json
{
  "monitor": "pod_crashes",
  "severity": "critical",
  "cluster": "prod-eks-1",
  "namespace": "checkout",
  "object": "pod/api-7d9f",
  "reason": "OOMKilled",
  "title": "OOMKilled on api-7d9f/server",
  "body": "Container server in pod checkout/api-7d9f is in state OOMKilled",
  "fired_at": "2026-05-24T08:59:00Z"
}
```

Content-Type is `application/json`. Any non-2xx response is treated as a failure and counted in `kpulse_notifier_failures_total`.

## Mattermost

Mattermost incoming webhooks accept `{"text": "..."}`. The generic webhook doesn't reshape to that format. If you want Mattermost-native styling, file an issue and we'll add a `mattermost` channel.

For now you can run a tiny shim that translates the JSON to the Mattermost shape, or open a PR.

## Helm

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values \
  --set channels.webhook.enabled=true \
  --set channels.webhook.url=https://hooks.example.com/kpulse \
  --set channels.webhook.headers.Authorization='Bearer $WEBHOOK_TOKEN' \
  --set-string channels.secrets.WEBHOOK_TOKEN=very-long-token
```
