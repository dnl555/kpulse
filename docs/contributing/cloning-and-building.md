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
