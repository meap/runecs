## Project Overview

RunECS is a Go CLI (Cobra + AWS SDK v2) for managing AWS ECS services and one-off tasks: deploy, scale, run, logs, prune.

## Build & Lint

```bash
make build        # bin/runecs
make lint         # golangci-lint (.golangci.yml, "all" with exclusions)
go test ./...     # no tests yet
```

## Architecture

**Two layers**, one file per command in each:
- `cmd/<command>.go` — Cobra command, flag parsing, output formatting.
- `internal/ecs/<command>.go` — AWS calls via shared `AWSClients`, returns typed result from `internal/ecs/types.go`.

**Conventions:**
- Persistent flags `--service cluster/service` and `--profile` live on `rootCmd` (`cmd/main.go`); read via `rootCmd.Flag(...).Value.String()`. `--service` required except for `completion`, `help`, `list`, `version` (enforced in `PersistentPreRunE`).
- Commands call `parseServiceFlag()`, then `ecs.NewAWSClients(ctx, profile)` which builds ECS, CloudWatch Logs, and STS clients together (10 retries).
- Parsing/slice helpers in `internal/utils/`.
- `main.go` injects version/commit/buildTime via ldflags (see `.goreleaser.yaml`) and calls `cmd.Execute()`.

## Releases

GoReleaser (`.goreleaser.yaml`) cross-compiles linux/darwin/windows. Homebrew tap: `meap/homebrew-runecs`.
