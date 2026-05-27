# Email (SMTP)

SMTP with PLAIN auth. Tested with Fastmail, Gmail (app password), AWS SES. Emails are sent as `multipart/alternative` (HTML + plain text), with a severity-colored top bar and a metadata table that renders cleanly in Gmail, Apple Mail, and Outlook.

## Configure

ConfigMap:

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

## Gmail with app password

1. Enable 2-step verification on your Google account.
2. Create an app password at [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords).
3. Use `smtp.gmail.com:587`, your Gmail address as `from` and `SMTP_USER`, the app password as `SMTP_PASS`.

## AWS SES

```yaml
smtp_host: email-smtp.us-east-1.amazonaws.com
smtp_port: 587
```

Use SMTP credentials from SES (not your IAM keys).

## Message format

Subject template:

```
[<cluster>] <prefix> <title>
```

`<prefix>` is the severity / state marker:

| Severity | Prefix | Banner color |
|---|---|---|
| info | `[i]` | blue (#2563eb) |
| warning | `[!]` | orange (#d97706) |
| critical | `[!!]` | red (#dc2626) |
| resolved (any prior severity) | `[RESOLVED]` | green (#10b981) |

The HTML body renders a colored banner matching the table above, followed by a metadata grid (cluster, namespace, object, reason, severity, fired-at) and the body text. A plain-text alternative is included so CLI mail clients fall back gracefully.

## Headers added

Each email includes:

- `Date`, `Message-ID`
- `List-Id: kpulse-<cluster> <kpulse.<cluster>.local>` (use for Gmail filters)
- `Auto-Submitted: auto-generated`
- `X-Kpulse-Cluster`, `X-Kpulse-Monitor`, `X-Kpulse-Severity`, `X-Kpulse-State`

A Gmail filter like `list:kpulse-prod-eks-1` matches every kpulse alert for that cluster. To route resolved messages to a separate label, add a filter on `"X-Kpulse-State: resolved"` (Gmail's "Has the words" search matches headers verbatim).

## Test

```bash
kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
curl 'http://localhost:8080/test-channel?name=email'
```

## Helm

```bash
helm upgrade kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --reuse-values \
  --set channels.email.enabled=true \
  --set channels.email.smtpHost=smtp.fastmail.com \
  --set channels.email.smtpPort=587 \
  --set channels.email.from=alerts@example.com \
  --set 'channels.email.to={oncall@example.com}' \
  --set-string channels.secrets.SMTP_USER=alerts@example.com \
  --set-string channels.secrets.SMTP_PASS=hunter2
```
