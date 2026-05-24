# Contributing to kpulse

Thanks for considering a contribution. kpulse is a small project and contributions of any size are welcome.

## Ways to contribute

There are many ways to contribute:

- **Suggest new features to be implemented** &mdash; open an issue with the `enhancement` label and describe the use case before any code.
- **Report issues** &mdash; bugs, confusing docs, surprising defaults. Include the kpulse version, Kubernetes version, and steps to reproduce.
- **Fixing issues** &mdash; pick anything labelled `good first issue` or `help wanted`. Comment on the issue first so we don't duplicate work.
- **Improve documentation** &mdash; README, [docs/monitors.md](docs/monitors.md), [docs/channels.md](docs/channels.md), or the Helm chart README. Typo fixes, clearer examples, or "I tried X and it didn't work, here's what I had to do" notes are all valuable.

If you're not sure where to start, open an issue and ask.

## Development setup

Requirements: Go 1.24+, Docker (for image builds), `kubectl`, and a cluster you don't mind breaking (kind or k3d work great).

```bash
git clone https://github.com/dnl555/kpulse.git
cd kpulse
go test ./...
make build              # builds bin/kpulse
make image VERSION=dev  # builds the container image
```

Run kpulse against a kubeconfig context (outside the cluster):

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

## Pull request checklist

- `go test ./...` passes.
- New behavior has a test.
- `gofmt -w .` clean.
- Commit message: short imperative subject (e.g. `Add Mattermost notifier`), optional 1-2 line body. No AI attribution lines.
- PR title under 70 chars. PR body: 1-3 bullets in a `## Summary` section. No need for "Test plan" / "Context" sections unless asked.
- Keep PRs small and focused. Big refactors are best discussed in an issue first.

## Coding conventions

- One responsibility per package. New monitors live in `internal/monitors/<name>.go` with a matching `_test.go`.
- Use the existing `Submitter` and `Notifier` interfaces; don't introduce new abstractions for one-off cases.
- Default to writing no code comments. Add one only when the *why* is non-obvious.
- No external services or telemetry. kpulse must remain self-contained.

## Adding a new monitor

1. Decide: informer-driven (reacts to k8s events) or periodic (polls every N).
2. Create `internal/monitors/<name>.go` implementing `monitors.Monitor`.
3. Add config knobs to `internal/config/config.go` (`Monitors` struct + defaults).
4. Wire it into `cmd/kpulse/main.go` `buildMonitors`.
5. Add a section to [docs/monitors.md](docs/monitors.md) including how to deliberately trigger it on a test cluster.
6. Test against a `fake.NewSimpleClientset`; see `pod_crashes_test.go` for the pattern.

## Adding a new channel

1. Implement the `notifiers.Notifier` interface in `internal/notifiers/<name>.go`.
2. Add a config stanza under `Channels` in `internal/config/config.go`.
3. Register it in `internal/notifiers/build.go`.
4. Add a `Helm chart values` entry under `channels.<name>` in `charts/kpulse/values.yaml` and the configmap template.
5. Document the channel in [docs/channels.md](docs/channels.md).

## Releases

Releases are tag-driven. The maintainer cuts releases by pushing a `vX.Y.Z` tag, which triggers `.github/workflows/release.yml` to build the image, render `kpulse.yaml`, and create a GitHub Release. Contributors don't need to do anything release-related.

## License

By contributing, you agree your contributions are licensed under the project's [MIT License](LICENSE).

## Code of Conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md). By participating you agree to its terms.
