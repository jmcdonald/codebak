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

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
