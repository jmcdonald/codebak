# Implementation Plan: Restic Integration (67-codebak-0vz)

This plan implements restic integration for sensitive path backups in codebak.

## Context
Epic: 67-codebak-0vz - Restic integration for sensitive path backups
Goal: Add encrypted incremental backups for ~/.ssh, ~/.aws, ~/.claude, etc.

## Tasks

### Phase 1: Foundation (No Dependencies)

- [x] [P2] 67-codebak-63a: Config schema for sensitive source type
  - Add SourceType field (git | sensitive) to config
  - Default sensitive paths list
  - Update config validation and tests

- [x] [P2] 67-codebak-5kp: Add restic port and adapter
  - Create ResticClient port interface
  - Implement exec-based adapter
  - Methods: Init, Backup, Snapshots, Restore, Forget
  - Unit tests for adapter

### Phase 2: Integration (Depends on Phase 1)

- [x] [P2] 67-codebak-e1k: Unified backup flow: git + restic by source type
  - Backup loop branches on source.Type
  - git sources → existing bundle flow
  - sensitive sources → restic backup
  - Integration tests for mixed configs

- [x] [P2] 67-codebak-s6f: TUI: Display sensitive sources with restic snapshots
  - Show 🔒 icon for sensitive sources
  - VERSION column shows snapshot count
  - New snapshot list view for sensitive sources
  - Unit tests for TUI changes

### Phase 3: Epic Complete

- [x] [P2] 67-codebak-0vz: Verify restic integration epic complete
  - All sub-tasks merged
  - End-to-end test: backup + restore sensitive paths
  - Update README with sensitive paths feature

## Validation Commands
From AGENTS.md:
1. `golangci-lint run ./...`
2. `go build -o codebak ./cmd/codebak`
3. `go test -v ./...`

## Git Convention
`ralph: <task description>`
