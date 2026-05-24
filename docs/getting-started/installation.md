# Installation

kpulse runs as a single Deployment in its own namespace. Pick one install method.

## Option 1: one-line script (recommended)

```bash
curl -fsSL https://kpulse.io/install.sh | bash
```

The script `kubectl apply`s the latest release manifest. After it finishes, follow [General Configuration](general-configuration.md) to set your cluster name and pick a channel.

## Option 2: raw manifest

```bash
kubectl apply -f https://github.com/dnl555/kpulse/releases/latest/download/kpulse.yaml
```

Pin to a specific version:

```bash
kubectl apply -f https://github.com/dnl555/kpulse/releases/download/v0.1.0/kpulse.yaml
```

## Option 3: Helm

```bash
helm install kpulse oci://ghcr.io/dnl555/charts/kpulse \
  --namespace kpulse --create-namespace \
  --set clusterName=prod-eks-1 \
  --set channels.slack.enabled=true \
  --set-string channels.secrets.SLACK_WEBHOOK_URL="https://hooks.slack.com/services/T0/B0/xxx"
```

See the [chart README](https://github.com/dnl555/kpulse/blob/main/charts/kpulse/README.md) for the full values reference.

## Option 4: build from source

```bash
git clone https://github.com/dnl555/kpulse.git
cd kpulse
make build              # bin/kpulse
make image VERSION=dev  # kpulse:dev local image
```

## What gets installed

The single manifest creates:

| Resource | Purpose |
|---|---|
| `Namespace/kpulse` | Where kpulse lives |
| `ServiceAccount/kpulse` | Identity used by the Pod |
| `ClusterRole/kpulse` | Read-only access to Pods, Events, Nodes, Deployments, etc. |
| `ClusterRoleBinding/kpulse` | Binds the ClusterRole to the ServiceAccount |
| `Role/kpulse-state` + binding | Allows writing the `kpulse-state` ConfigMap (only) |
| `ConfigMap/kpulse-config` | All tuning knobs (heavily commented) |
| `Secret/kpulse-secrets` | Channel credentials |
| `Service/kpulse` | Cluster-internal access to `/healthz`, `/test-channel`, etc. |
| `Deployment/kpulse` | 1 replica, runs the binary |

## Verifying

```bash
kubectl -n kpulse get pods
kubectl -n kpulse logs deploy/kpulse
```

You should see:

```
kpulse v0.1.0 starting
warn: no channels configured; alerts will be dropped. ...
starting 12 monitors
kpulse ready, watching cluster "REPLACE_ME"
```

The warning is expected on a fresh install. Next step: [General Configuration](general-configuration.md).

## Uninstall

```bash
kubectl delete -f https://github.com/dnl555/kpulse/releases/latest/download/kpulse.yaml
```

Or for Helm: `helm uninstall kpulse -n kpulse && kubectl delete ns kpulse`.
