# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.10.0]: https://github.com/meap/runecs/compare/v0.9...v0.10.0
