# TLS Certificate Expiry

Periodic scan that finds every `Secret` of type `kubernetes.io/tls`, parses the leaf certificate, and alerts when it's about to expire.

```yaml
monitors:
  tls_cert_expiry:
    enabled: true
    warn_days: 14
    crit_days: 3
    interval: 6h
```

## Scope

Any `Secret` with `type: kubernetes.io/tls` and a parseable `tls.crt` field is checked. This covers:

- Ingress TLS Secrets (cert-manager, manually issued, etc.)
- mTLS Secrets used by service meshes (Istio, Linkerd) IF stored as standard TLS Secrets
- Any custom TLS Secret you create

Namespace filtering applies.

## Severities

- `<= warn_days` -> `warning`
- `<= crit_days` -> `critical`

The defaults (14 / 3) give you two weeks to act on a typical 90-day Let's Encrypt cert and a final scream if cert-manager forgot to renew.

## Trigger it on purpose

Create a short-lived self-signed cert:

```bash
openssl req -x509 -newkey rsa:2048 -days 1 -nodes -keyout tls.key -out tls.crt -subj "/CN=test"
kubectl create secret tls expiring-soon --cert=tls.crt --key=tls.key
```

Within 6 hours (or sooner if you change `interval`), kpulse will fire `critical` since 1 day is less than `crit_days: 3`.

## Quiet a noisy chain

If a cert intentionally has a 90-day cycle and cert-manager always renews at day 30, the warning fires repeatedly during the renewal window. Options:

- Raise `warn_days` to 30 so it only fires once cert-manager is past its expected renewal point
- Disable warning tier: `warn_days: 3, crit_days: 3` (both fire only at 3 days, severity escalates internally)
