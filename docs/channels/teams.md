# Microsoft Teams

Teams Incoming Webhook (one per channel).

## Get a webhook URL

In Teams: **... menu on the target channel -> Workflows -> Post to a channel when a webhook request is received** (Power Automate flow) OR the classic **Incoming Webhook** connector if still available in your tenant. Both produce a URL kpulse can POST to.

## Configure

ConfigMap:

```yaml
channels:
  teams:
    webhook_url_from_secret: TEAMS_WEBHOOK_URL
```

Secret:

```yaml
stringData:
  TEAMS_WEBHOOK_URL: https://outlook.office.com/webhook/...
```

## Message format

kpulse sends a `MessageCard` with:

- Severity-colored title bar (red / orange / blue)
- The alert title
- A facts table: Namespace, Object, Reason, Severity
- The body text

## Test

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=teams'
```

## Helm

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values \
  --set channels.teams.enabled=true \
  --set-string channels.secrets.TEAMS_WEBHOOK_URL="https://outlook.office.com/webhook/..."
```
