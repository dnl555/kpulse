---
title: "Local-only dev with Tilt: changing kpulse without a release"
description: How the contributor loop works, why we didn't pick a registry pattern, and what we learned hooking Tilt to k3d.
---

# Local-only dev with Tilt: changing kpulse without a release

The first time I had to iterate on a kpulse feature against a real cluster, the loop was painful. Make a change. Commit. Push a tag. Wait for the GitHub release workflow to build the image and push to GHCR. Re-install the cluster. Watch.

This is fine for "I have a working release and I want to validate the published artifact." It is awful for "I am writing code and I want to know if it works." A 90-second pipeline between every keystroke and every test is a tax on momentum.

So we wired Tilt to k3d for v0.2.2. The local dev loop is now around twelve seconds. Edit a Go file, save, look at the cluster, the new binary is running with the changes.

## Prereqs (one-time)

```bash
brew install tilt-dev/tap/tilt k3d
k3d cluster create kpulse-test
kubectl config use-context k3d-kpulse-test
```

## Run

```bash
tilt up
```

The first build takes about thirty seconds. It compiles the binary, builds an alpine-based dev image (the production image is distroless, more on that below), imports it into k3d, applies the dev manifest, and ports the kpulse HTTP server to `localhost:8080`. It also deploys a small webhook echo sink at `localhost:8081` so you can see exactly what kpulse would send to your real channel.

After that, every Go file change triggers a rebuild loop that takes about twelve seconds end to end:

```
go-build (3s) -> docker build (1s) -> k3d image import (3s) -> deployment roll (5s)
```

If you want to exercise alerts immediately, there is a sidecar set of deliberately broken Pods:

```bash
tilt up -- triggers
```

This deploys a `triggers` namespace with a Pod that CrashLoopBackOffs, a Pod with an image that does not exist, a Pod that OOMKills itself, and a Job that always fails. Within thirty seconds you should see all of these arrive at the webhook sink. Delete any of them to test the resolve path.

## What we did not do (and why)

The internet's recommended pattern for Tilt-with-k3d is a local registry. You stand up a Docker registry on `localhost:5000`, configure k3d to trust it, point Tilt at it as a `default_registry`, and let Tilt do its content-addressed-tag thing pushing to the registry.

That pattern works. It is also more setup, more moving parts, and more failure modes (registry up? credentials right? k3d config baked the registry in?). We tried it. It worked. It also added a step every new contributor had to do correctly.

So we took the simpler path: build a Docker image locally, run `k3d image import` to copy it into the k3d node's containerd, refer to the image with `imagePullPolicy: Never`. The trick is that `k3d image import` is fast enough (about three seconds) that we do not need the registry's caching at all. The loop ends up the same speed in practice and the setup is one less thing.

The Tiltfile has the entire pipeline:

```python
local_resource(
    'go-build',
    cmd='CGO_ENABLED=0 GOOS=linux GOARCH=...' +
        ' go build -o ./bin/kpulse ./cmd/kpulse',
    deps=['cmd', 'internal', 'go.mod', 'go.sum'],
)

custom_build(
    ref='kpulse',
    command=('docker build -f Dockerfile.dev -t $EXPECTED_REF . && ' +
             'k3d image import $EXPECTED_REF -c kpulse-test'),
    deps=['bin/kpulse', 'Dockerfile.dev'],
    tag='dev',
    disable_push=True,
)
```

## Why we did not use live_update

Tilt's `live_update` step can sync a file into a running container and restart the process without doing a fresh image build. For a Go binary this should be the fastest possible loop, since the binary is already built locally.

It does not work cleanly with our distroless production image because distroless has no shell, and the canonical `restart_container()` call is deprecated for `k8s_resource`. The supported path is the `restart_process` extension, which works fine but requires the image to include the tilt-restart-wrapper binary.

We weighed it: another binary in the dev image, another concept in the Tiltfile, another thing for contributors to understand &mdash; against a five-to-eight-second loop saving. We decided the simpler image rebuild loop was worth the extra seconds. If someone wants to add `restart_process` later in a PR, we are happy to look at it.

## The Secret-wipe hazard we ran into

One trap that bit us during dev and ended up being a real bug in the published artifact: the dev manifest used to include a placeholder `Secret/kpulse-secrets`. Every time someone (or Tilt) ran `kubectl apply -f dev/kpulse.dev.yaml`, that placeholder Secret wiped any real credentials a developer had set with `kubectl patch`.

The fix was to remove the Secret from the manifest entirely. Tilt has a one-shot `local_resource` that creates the Secret only if it does not already exist:

```python
local_resource(
    'kpulse-secrets-bootstrap',
    cmd=("kubectl get ns kpulse >/dev/null 2>&1 || kubectl create ns kpulse; " +
         "kubectl -n kpulse get secret kpulse-secrets >/dev/null 2>&1 || " +
         "kubectl -n kpulse create secret generic kpulse-secrets " +
         "--from-literal=PLACEHOLDER=set-real-creds-with-kubectl-patch"),
)
```

The same trap was in the *production* install. v0.2.2 fixes both: the manifest no longer ships a Secret, the install.sh creates a stub only if missing, and the Deployment's Secret volume is `optional: true` so a missing Secret does not block the Pod from starting. Operators can now `kubectl patch secret kpulse-secrets` to add real credentials, and re-running `kubectl apply -f kpulse.yaml` (or `helm upgrade`) never touches their creds.

## What this enables

The whole point of the Tilt loop is to make small changes cheap. A typical session now looks like: open the Tilt UI in the browser, edit `internal/notifiers/slack.go`, save, see the new behavior in a real Slack channel three seconds later (Tilt's port-forward of `kpulse:8080` makes the `/test-channel?name=slack` endpoint trivial to hit). When I added the email rewrite, I went through about fifteen iterations of "render this in Gmail, look at it, change the HTML, look again" in maybe twenty minutes total.

For contributors, the goal is that the README and the [Cloning and Building](../contributing/cloning-and-building.md) page are enough to get from `git clone` to a working dev loop without reading the source. If you try this and it does not work, that is a doc bug worth filing.
