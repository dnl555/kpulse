# Discord

Standard Discord channel webhook.

## Get a webhook URL

In your Discord server: **Server Settings -> Integrations -> Webhooks -> New Webhook**. Pick a channel, copy the URL.

## Configure

ConfigMap:

```yaml
channels:
  discord:
    webhook_url_from_secret: DISCORD_WEBHOOK_URL
```

Secret:

```yaml
stringData:
  DISCORD_WEBHOOK_URL: https://discord.com/api/webhooks/.../...
```

## Message format

```
[!!] **[prod-eks-1]** `checkout/pod/api-7d9f` OOMKilled on api-7d9f/server
Container server in pod checkout/api-7d9f is in state OOMKilled
```

Discord caps content at 2000 chars; kpulse truncates at 1900 and appends `(truncated)`.

## Test

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=discord'
```

## Helm

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values \
  --set channels.discord.enabled=true \
  --set-string channels.secrets.DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
```
