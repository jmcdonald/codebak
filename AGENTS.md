# AGENTS.md - codebak Build Instructions

## Language & Framework
- Go 1.21+
- Bubble Tea TUI framework
- golangci-lint for linting

## Build Commands
```bash
go build -o codebak ./cmd/codebak
```

## Test Commands
```bash
go test -v ./...
```

## Lint Commands
```bash
golangci-lint run ./...
```

## Validation Order
Always run in this order:
1. `golangci-lint run ./...` (lint)
2. `go build -o codebak ./cmd/codebak` (build)
3. `go test -v ./...` (test)

## Git Convention
Commit message format:
```
ralph: <task description>
```

## Important Directories
- `cmd/codebak/` - Main entry point
- `internal/` - Core packages
  - `internal/backup/` - Backup logic
  - `internal/config/` - Configuration
  - `internal/git/` - Git operations
  - `internal/manifest/` - Manifest management
  - `internal/recovery/` - Recovery operations
  - `internal/tui/` - Terminal UI

## Project Notes
- This is a Go-based TUI backup application
- Uses restic for encrypted backups
- Uses git bundles for incremental backups
- Configuration is YAML-based
