# codebak

Incremental code backup tool with TUI for macOS/Linux.

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/mcdonaldj/codebak/main/install.sh | bash
```

Or with Homebrew (coming soon):
```bash
brew install mcdonaldj/tap/codebak
```

## Quick Start

```bash
# Initialize configuration
codebak init

# Run backup (all changed projects)
codebak run

# Launch interactive TUI
codebak
```

## Features

- **Incremental backups**: Only backs up projects that have changed (git HEAD or file mtime)
- **Interactive TUI**: Browse projects, versions, and diffs with keyboard navigation
- **Integrity verification**: SHA256 checksums for all backups
- **Retention policy**: Automatic pruning of old backups
- **Recovery**: Restore any version with optional wipe or archive of current
- **Scheduling**: macOS launchd integration for daily backups

## Configuration

Config file: `~/.codebak/config.yaml`

```yaml
source_dir: ~/code           # Directory containing projects
backup_dir: ~/backups        # Where to store backups
schedule: daily              # Backup schedule
time: "03:00"                # Time for scheduled backups

exclude:                     # Patterns to exclude
  - node_modules
  - .venv
  - __pycache__
  - .git
  - "*.pyc"
  - .DS_Store
  - .idea
  - .vscode
  - target
  - dist
  - build

retention:
  keep_last: 30              # Keep last N backups per project
```

## Commands

| Command | Description |
|---------|-------------|
| `codebak` | Launch interactive TUI |
| `codebak run [project]` | Backup all changed projects (or specific project) |
| `codebak list <project>` | List all backup versions |
| `codebak verify <project> [version]` | Verify backup integrity |
| `codebak recover <project> [options]` | Recover from backup |
| `codebak install` | Install daily launchd schedule (3am) |
| `codebak uninstall` | Remove launchd schedule |
| `codebak status` | Show configuration and launchd status |
| `codebak init` | Create default config file |
| `codebak version` | Show version |

### Recovery Options

```bash
# Recover latest version (fails if project exists)
codebak recover myproject

# Recover and delete current project
codebak recover myproject --wipe

# Recover and archive current project
codebak recover myproject --archive

# Recover specific version
codebak recover myproject --version=20241215-100000
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate list |
| `Enter` | Select project/version |
| `Backspace` | Go back |
| `d` | Diff mode (select 2 versions) |
| `Space` | Toggle selection (in diff mode) |
| `v` | Verify selected backup |
| `r` | Recover selected version |
| `?` | Toggle help |
| `q` | Quit |

## Building from Source

```bash
# Clone repository
git clone https://github.com/mcdonaldj/codebak.git
cd codebak

# Build
make build

# Install to ~/go/bin
make install

# Run tests
make test

# Cross-compile for all platforms
make build-all
```

## How It Works

1. **Change Detection**: For git repos, compares HEAD commit. Otherwise, checks file modification times.
2. **Backup Creation**: Creates timestamped zip archives with excluded patterns filtered out.
3. **Manifest Tracking**: Each project has a `manifest.json` with backup history, checksums, and metadata.
4. **Retention**: Automatically prunes old backups exceeding the `keep_last` limit.

## Backup Structure

```
~/backups/
├── project-a/
│   ├── manifest.json
│   ├── 20241215-100000.zip
│   ├── 20241216-100000.zip
│   └── 20241217-100000.zip
└── project-b/
    ├── manifest.json
    └── 20241217-120000.zip
```

## License

MIT
