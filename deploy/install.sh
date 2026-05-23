#!/usr/bin/env sh
set -e
VERSION="${KPULSE_VERSION:-latest}"
OWNER="${KPULSE_OWNER:-dnl555}"
REPO="${KPULSE_REPO:-kpulse}"

if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${OWNER}/${REPO}/releases/latest/download/kpulse.yaml"
else
  URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/kpulse.yaml"
fi

echo "Installing kpulse (${VERSION}) from ${URL}"
kubectl apply -f "$URL"

cat <<'EOF'

kpulse installed in namespace 'kpulse'.

Next steps:
  1. Set your cluster name and pick channels:
       kubectl -n kpulse edit configmap kpulse-config
  2. Add channel credentials:
       kubectl -n kpulse edit secret kpulse-secrets
  3. Apply changes:
       kubectl -n kpulse rollout restart deploy/kpulse
  4. Test a channel:
       kubectl -n kpulse port-forward svc/kpulse 8080:8080 &
       curl 'http://localhost:8080/test-channel?name=slack'
EOF
