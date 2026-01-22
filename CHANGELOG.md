# Changelog

All notable changes to codebak will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-01-22

### Added

- **Sensitive Path Protection**: Encrypted backups using [restic](https://restic.net/) for sensitive dotfiles and config directories
  - AES-256 encryption at rest
  - Incremental, deduplicated backups across all sensitive sources
  - Automatic repository initialization on first backup
  - Configure with `type: sensitive` in sources

- **Multi-Source Support**: Configure multiple source directories with different backup strategies
  - `type: git` - Traditional zip-based backups (default)
  - `type: sensitive` - Restic encrypted backups
  - Custom labels and icons per source

- **TUI Enhancements**:
  - 🔒 icon for sensitive sources in project list
  - Snapshot count display for restic-backed sources
  - Snapshot list view when selecting sensitive sources

### Changed

- **Breaking**: Config format updated to use `sources:` array instead of single `source_dir:`
  - Old configs with `source_dir:` still work (auto-migrated)
  - New `sources:` format supports multiple directories with types

### Migration Guide

**Old format (still works):**
```yaml
source_dir: ~/code
```

**New format (recommended):**
```yaml
sources:
  - path: ~/code
    type: git
  - path: ~/.ssh
    type: sensitive
    label: SSH Keys
```

## [0.6.0] - 2024-12-15

### Added
- Settings view with backup directory migration
- Folder picker with vim-style navigation
- Interactive folder selection shortcuts

## [0.5.0] - 2024-12-01

### Added
- Version comparison and diff viewing
- Line-by-line file diff with syntax highlighting
- Diff navigation between versions

## [0.4.0] - 2024-11-15

### Added
- Backup verification with SHA256 checksums
- Recovery options (archive, wipe)
- Retention policy with automatic pruning

## [0.3.0] - 2024-11-01

### Added
- Interactive TUI with vim-style keybindings
- Project and version list views
- Smart change detection via git HEAD

## [0.2.0] - 2024-10-15

### Added
- CLI commands: run, list, verify, recover
- Launchd scheduling support
- Exclusion patterns

## [0.1.0] - 2024-10-01

### Added
- Initial release
- Zip-based project backups
- Manifest tracking with checksums
