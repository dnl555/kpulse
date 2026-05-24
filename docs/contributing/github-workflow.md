# GitHub Workflow

How code flows from your branch to merged.

## Branch + PR

1. Fork [dnl555/kpulse](https://github.com/dnl555/kpulse) on GitHub.
2. Clone your fork, create a branch:
   ```bash
   git checkout -b feat/my-thing
   ```
3. Make your changes. Run `go test ./...` and `gofmt -w .` before committing.
4. Commit. Short imperative subject (under 70 chars), optional 1-2 line body. No AI attribution lines.
   ```bash
   git commit -m "Add Mattermost notifier"
   ```
5. Push and open a PR against `main`:
   ```bash
   git push origin feat/my-thing
   gh pr create
   ```

## PR title and body

- Title under 70 chars
- Body: 1-3 bullets under a `## Summary` section
- No need for "Context" / "Test plan" sections unless asked

Example:

```
## Summary
- Add Mattermost notifier with native message formatting
- Wire it in `notifiers.Build` when `channels.mattermost.enabled` is set
- Tests cover happy path and 4xx error
```

## CI

Two workflows run automatically:

- **`ci.yml`** on every push and PR: `go test ./... -race`, `golangci-lint run`, `docker build`
- **`release.yml`** on tags `v*`: build + push image to `ghcr.io/dnl555/kpulse:<tag>`, render `kpulse.yaml`, create a GitHub Release

A green `ci` check is required before merge.

## Adding a new monitor

1. `internal/monitors/<name>.go` implementing `monitors.Monitor`.
2. Add config knobs to `internal/config/config.go` (`Monitors` struct + defaults in `defaults()`).
3. Wire it into `cmd/kpulse/main.go` `buildMonitors`.
4. Add a section to the [Monitors](../monitors/index.md) docs including how to trigger it on a test cluster.
5. Test against a `fake.NewSimpleClientset`; see [`pod_crashes_test.go`](https://github.com/dnl555/kpulse/blob/main/internal/monitors/pod_crashes_test.go) for the pattern.

## Adding a new channel

1. Implement the `notifiers.Notifier` interface in `internal/notifiers/<name>.go`.
2. Add a config stanza under `Channels` in `internal/config/config.go`.
3. Register it in `internal/notifiers/build.go`.
4. Add a `channels.<name>` block in `charts/kpulse/values.yaml` and wire it in `charts/kpulse/templates/configmap.yaml`.
5. Document the channel under [Channels](../channels/index.md).

## Conventions

- One responsibility per package.
- Use existing interfaces (`Submitter`, `Notifier`); don't introduce new abstractions for one-off cases.
- Default to no code comments. Add one only when the *why* is non-obvious.
- No external services or telemetry. kpulse must remain self-contained.

## Releases

Tag-driven; the maintainer cuts releases. Contributors don't need to do anything release-related.
