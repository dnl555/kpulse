# Cloning and Building

## Prerequisites

- Go 1.24+
- Docker (for image builds)
- `kubectl` and a cluster (kind or k3d work great)
- Optional: `make`, `helm`

## Clone

```bash
git clone https://github.com/dnl555/kpulse.git
cd kpulse
```

## Run tests

```bash
go test ./...
```

All packages should pass. The full suite takes a few seconds (it uses `client-go/kubernetes/fake` for monitor tests, no real cluster needed).

## Build the binary

```bash
make build
./bin/kpulse --help
```

Output:

```
Usage of ./bin/kpulse:
  -config string
        ConfigMap-mounted config path (default "/etc/kpulse/config.yaml")
  -http string
        HTTP listen addr (default ":8080")
  -namespace string
        kpulse namespace (state ConfigMap location) (default "kpulse")
  -secrets string
        directory containing per-key secret files (default "/etc/kpulse/secrets")
```

## Build the container image

```bash
make image VERSION=dev
docker run --rm kpulse:dev --help
```

## Run against a real cluster (without deploying)

```bash
mkdir -p /tmp/kpulse-secrets
echo -n "$SLACK_WEBHOOK_URL" > /tmp/kpulse-secrets/SLACK_WEBHOOK_URL
cat > /tmp/kpulse-config.yaml <<YAML
cluster: { name: dev }
channels:
  slack:
    webhook_url_from_secret: SLACK_WEBHOOK_URL
    default: true
YAML

KUBECONFIG=$HOME/.kube/config ./bin/kpulse \
  --config=/tmp/kpulse-config.yaml \
  --secrets=/tmp/kpulse-secrets \
  --namespace=kpulse
```

kpulse will use the current kubeconfig context. Ctrl+C to stop.

## Build the docs locally

```bash
pip install mkdocs-material
mkdocs serve
# open http://127.0.0.1:8000
```

## Render the deploy manifest

```bash
VERSION=v0.1.0 ./deploy/render.sh > kpulse.yaml
```

## Run lint

```bash
# install once
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.0

golangci-lint run
```

## Live-reload dev loop with Tilt

The repo ships a `Tiltfile` that compiles, deploys, and watches the source on a local k3d cluster. Loop time is ~10-15 s per change.

Prereqs (one-time):

```bash
brew install tilt-dev/tap/tilt k3d
k3d cluster create kpulse-test
kubectl config use-context k3d-kpulse-test
```

Run:

```bash
tilt up                   # builds, deploys, watches; opens Tilt UI
tilt up -- triggers       # also deploy intentionally-broken workloads
tilt down                 # tear it all back down
```

What you get:

- **kpulse** running with the in-cluster generic webhook channel wired to a tiny echo sink. Open `http://localhost:8080/test-channel?name=webhook` to send a synthetic alert; open `http://localhost:8081/` to inspect the sink's request log.
- A `demo` namespace with the **sink** Deployment.
- Optional `triggers` namespace with deliberately broken Pods (CrashLoopBackOff, ImagePullBackOff, OOMKilled, failing Job) to validate the alert path.

To test a real channel (Slack, email, etc.), patch the Secret out of band; `kubectl apply` of the dev manifest never touches the Secret:

```bash
kubectl -n kpulse patch secret kpulse-secrets --type=merge \
  -p '{"stringData":{"SLACK_WEBHOOK_URL":"https://hooks.slack.com/services/..."}}'
kubectl -n kpulse rollout restart deploy/kpulse
```
