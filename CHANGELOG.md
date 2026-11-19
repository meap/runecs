# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- `run --wait` no longer fails when ECS does not yet return a task it has just started. ECS is eventually consistent, so `DescribeTasks` can briefly report the new task as missing; runecs now re-checks the same task with a growing delay (1 s up to 5 s) for up to 2 minutes before giving up. Previously it stopped waiting at once, while the task kept running.

## [0.11.1] - 2026-11-19

### Fixed
- `run --wait` now prints the task's logs when the task exits with a non-zero code. Previously the output was dropped and only the exit code was reported, so a failed command (e.g. a database migration in CI) could not be diagnosed. runecs still exits with status 1.
- macOS no longer blocks `runecs` installed via Homebrew with "Apple could not verify “runecs” is free of malware". The Homebrew cask now removes the quarantine flag after install.

## [0.11.0] - 2026-11-19

### New
- `restart` now accepts `--wait` (`-w`) to block until the new deployment of the service (or of every service with `--cluster`) has completed. Fails on rollout failure or circuit-breaker rollback, and times out after 30 minutes. Not supported with `--kill` (a warning is printed and the command does not wait).

### Fixed
- `go install github.com/meap/runecs@latest` now works. The Go module path changed from `runecs.io/v1` to `github.com/meap/runecs`. Releases before this one cannot be installed with `go install`; use the release binaries or Homebrew for those.

## [0.10.0] - 2026-04-19

### New
- `run` now accepts `--cpu` and `--memory` flags to override task sizing on a per-run basis without editing the task definition.

### Fixed
- `run` no longer returns truncated logs. Previously Live Tail could miss events emitted before the tail attached or still in flight when the task stopped; now runecs waits for the task to finish, gives CloudWatch a moment to flush, and fetches the complete log.
- Error messages from AWS/ECS calls now include context so failures are easier to diagnose.

### Docs
- Added `AWS_REGION` guidance for profiles that don't set a region.
- Expanded direnv examples for AWS authentication setups.

### Under the hood
- Added a `make lint` target.
- Internal cleanup of prune defaults.

[0.11.1]: https://github.com/meap/runecs/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/meap/runecs/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/meap/runecs/compare/v0.9...v0.10.0
