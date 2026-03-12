# tko

This file provides guidance to Agents when working with code in this repository.

## What is tko?

tko is a rootless, daemonless OCI container image builder. It builds container images from pre-compiled artifacts without requiring Docker, DinD, or elevated privileges. It uses `google/go-containerregistry` for image manipulation.

## Build & Test Commands

```bash
# Test
go test -v ./...

# Run a single test
go test -v ./... -run TestName

# Autoformatter
go fmt ./...
```

## Architecture

**Entry point**: `main.go` — initializes CLI via Kong framework with YAML config loading (`.tko.yml`).

**Packages**:

- **pkg/cmd** — CLI command definitions and execution. `cli.go` defines the Kong command structure (version, build). `build.go` contains BuildCmd with ~20 flags (all prefixed `TKO_*` as env vars). `git.go` extracts commit/tag info. `simpleKeychain.go` handles basic auth.

- **pkg/build** — Core image building logic:
  - `root.go` — Main `Build()` orchestration: fetch base → create layer → mutate config → publish
  - `base.go` — Base image resolution and platform selection
  - `layers.go` — TAR layer creation with reproducible timestamps (unix epoch)
  - `publish.go` — Three output targets: REMOTE (registry), LOCAL_DAEMON (Docker), LOCAL_FILE
  - `ExitCleanupWatcher.go` — Signal-based cleanup of temp files
  - `temp.go` — Temp file management

**Build flow**: CLI parses args → sets up multi-keychain auth (Docker config, GitHub, Google, simple) → creates BuildSpec → `build.Build()` fetches base image, creates artifact layer, mutates image config (entrypoint, env, labels, annotations), publishes to target.

## Key Conventions

- Reproducible builds: all TAR entries use unix epoch timestamps, stripped PAX records
- OCI labels follow `org.opencontainers.image.*` convention
- GoReleaser handles releases with ldflags for version/commit/date injection
- CI dogfoods tko to build its own container image
- Sane defaults, minimal configuration
- As simple as is possible, while maintaining needed functionality
