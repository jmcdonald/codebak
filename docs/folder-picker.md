# Folder Picker - Migrate Backups

The folder picker provides a production-quality interface for migrating your backup directory to a new location.

## Accessing the Folder Picker

1. Press `?` from any view to open **Settings**
2. Navigate to **Migrate Backups** and press `Enter`
3. The folder picker opens, starting at your home directory

## Features

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Enter` | Open selected directory |
| `s` or `Space` | **Select current directory** as new backup location |
| `q` or `Ctrl+C` | Cancel and return to Settings |

### Quick Navigation Shortcuts

| Key | Action |
|-----|--------|
| `~` | Jump to home directory |
| `.` | Jump to current backup directory |
| `-` | Go back to previous directory (history) |

### Hybrid Type Mode

For power users who know the exact path:

| Key | Action |
|-----|--------|
| `/` or `g` | Enter typing mode |
| `Enter` | Navigate to typed path |
| `Esc` | Cancel typing mode |

**Typing mode supports:**
- Tilde expansion: `~/backups` expands to `/Users/you/backups`
- Path validation: Only valid directories are accepted
- Error feedback: Invalid paths show an error message

## Confirmation Dialog

After selecting a destination, a confirmation dialog appears showing:

```
⚠️  Confirm Move

  From:
    /old/backup/path

  To:
    /new/backup/path

  Projects: 5
  Total size: 1.2 GB

  This will move all backups to the new location.

  [y] Confirm  [n] Cancel
```

| Key | Action |
|-----|--------|
| `y` or `Y` | Confirm and execute the move |
| `n`, `N`, `Esc`, or `q` | Cancel and return to folder picker |

## What Happens During Move

1. **Validation**: Checks that source and destination are different
2. **Directory Creation**: Creates destination directory if needed
3. **Move Operation**: Uses `os.Rename` for atomic moves on same filesystem
4. **Config Update**: Updates `~/.codebak/config.yaml` with new path
5. **Feedback**: Shows success or error message in the TUI

## Technical Details

### State Management

- **History Stack**: Navigation history is tracked for the `-` (back) feature
- **State Reset**: History and typing mode are reset each time you enter the picker
- **Cursor Blink**: Typing mode properly initializes cursor blinking via `Focus()` Cmd

### File Structure

```
internal/tui/model.go
├── handleFolderPicker()     # Main folder picker handler
├── handlePathInput()        # Typing mode handler
├── handleMoveConfirm()      # Confirmation dialog handler
├── renderMoveInputView()    # Folder picker UI
├── renderMoveConfirmView()  # Confirmation dialog UI
└── executeMoveBackups()     # Actual move operation
```

### Test Coverage

21 functional tests cover:
- State reset on entry
- All keyboard shortcuts (`s`, `space`, `~`, `.`, `-`, `/`, `g`, `q`)
- Typing mode (Enter, Esc, empty input)
- Confirmation dialog (y, n, esc)
- Render output for both views
