# kpulse local dev with Tilt.
#
# Prereqs (one-time):
#   1. Install Tilt:    brew install tilt-dev/tap/tilt
#   2. Create a k3d cluster (any name; Tilt uses the current kubectl context):
#         k3d cluster create kpulse-test
#         kubectl config use-context k3d-kpulse-test
#
# Usage:
#   tilt up                       # builds, deploys, watches; opens Tilt UI
#   tilt up -- triggers           # also deploy deliberately-broken workloads
#   tilt down                     # tear it all back down
#
# Loop:
#   - Edit any *.go file under cmd/ or internal/
#   - Tilt recompiles the binary, rebuilds the image, imports into k3d,
#     and rolls the Deployment. ~10-15s per change.
#   - Edit dev/kpulse.dev.yaml (ConfigMap) and the deployment rolls.
#
# See alerts arrive:
#   - kpulse: http://localhost:8080  (test-channel, healthz, readyz)
#   - sink:   http://localhost:8081  (browse received webhook requests)

allow_k8s_contexts(k8s_context())
ctx = k8s_context()
if not ctx.startswith('k3d-'):
    fail('Tiltfile only intended for k3d. Current context: ' + ctx)
cluster_name = ctx[len('k3d-'):]

goarch = str(local('uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/"', quiet=True)).strip()

# 1) Compile kpulse on every Go source change.
local_resource(
    'go-build',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=' + goarch +
        ' go build -ldflags "-s -w -X main.version=dev" -o ./bin/kpulse ./cmd/kpulse',
    deps=['cmd', 'internal', 'go.mod', 'go.sum'],
    labels=['kpulse'],
)

# 2) Build the dev image, import into k3d, and bump the Deployment so the
#    new image is picked up. Static tag kpulse:dev keeps the manifest simple.
local_resource(
    'image-import',
    cmd='\n'.join([
        'docker build -q -f Dockerfile.dev -t kpulse:dev . >/dev/null',
        'k3d image import kpulse:dev -c ' + cluster_name + ' >/dev/null',
        ('kubectl -n kpulse set env deploy/kpulse ' +
         'KPULSE_DEV_BUILD="$(date +%s)" --overwrite >/dev/null 2>&1 || true'),
    ]),
    deps=['bin/kpulse', 'Dockerfile.dev'],
    resource_deps=['go-build'],
    labels=['kpulse'],
)

# 2b) Ensure the kpulse-secrets Secret exists (without overwriting real
#     credentials a user may have set via `kubectl -n kpulse patch secret ...`).
local_resource(
    'kpulse-secrets-bootstrap',
    cmd=("kubectl get ns kpulse >/dev/null 2>&1 || kubectl create ns kpulse; " +
         "kubectl -n kpulse get secret kpulse-secrets >/dev/null 2>&1 || " +
         "kubectl -n kpulse create secret generic kpulse-secrets " +
         "--from-literal=PLACEHOLDER=set-real-creds-with-kubectl-patch"),
    labels=['kpulse'],
)

# 3) Apply the dev manifest. image=kpulse:dev, imagePullPolicy=Never.
k8s_yaml('dev/kpulse.dev.yaml')
k8s_resource(
    'kpulse',
    port_forwards=['8080:8080'],
    resource_deps=['image-import', 'kpulse-secrets-bootstrap'],
    labels=['kpulse'],
)

# 4) In-cluster webhook sink (URLs in kpulse-config point at sink.demo.svc).
k8s_yaml('dev/sink.yaml')
k8s_resource(
    'sink',
    port_forwards=['8081:8080'],
    labels=['demo'],
    new_name='webhook-sink',
)

# 5) Optional: deliberately broken workloads. Enable with: tilt up -- triggers
config.define_string_list('to-run', args=True)
cfg = config.parse()
if 'triggers' in cfg.get('to-run', []):
    k8s_yaml('dev/triggers.yaml')
    for n in ['crash', 'badimage', 'oom']:
        k8s_resource(new_name=n, objects=[n + ':pod:triggers'], labels=['triggers'])
    k8s_resource(new_name='fail-job', objects=['fail-job:job:triggers'], labels=['triggers'])
