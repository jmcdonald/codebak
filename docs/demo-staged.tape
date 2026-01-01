# codebak demo - Staged data for compelling screenshots
#
# Prerequisites: Run ./scripts/stage-demo.sh first
#
# Usage:
#   ./scripts/run-demo-capture.sh

Output docs/codebak-demo.gif
Set Width 900
Set Height 600
Set FontSize 14
Set Theme "Dracula"
Set Padding 20

# Launch codebak TUI
Type "codebak"
Enter
Sleep 3s

# Screenshot projects view (5 projects with varied states)
Screenshot docs/screenshots/projects-view.png

# Navigate to website-redesign (last in alphabetical list, 5 versions)
# Projects: cli-toolkit, ml-pipeline, mobile-app, new-project, website-redesign
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 300ms

# Select website-redesign to see its 5 versions
Enter
Sleep 2s

# Screenshot versions view (5 versions)
Screenshot docs/screenshots/versions-view.png

# Start diff mode
Type "d"
Sleep 1s

# Screenshot diff selection mode
Screenshot docs/screenshots/diff-select.png

# Select first version (newest - 20260101-140000)
Space
Sleep 500ms

# Move down to older version (20251231-030000 has different content)
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 300ms

# Select second version - triggers diff computation
Space
Sleep 3s

# Screenshot diff result showing file changes
Screenshot docs/screenshots/diff-result.png

# Enter a file to see line-by-line diff (select first modified file)
Down
Sleep 300ms
Enter
Sleep 2s

# Screenshot file diff view
Screenshot docs/screenshots/file-diff.png

# Go back to diff result
Escape
Sleep 500ms

# Go back to versions
Escape
Sleep 1s

# Go back to projects
Escape
Sleep 1s

# Run backup on a project
Type "r"
Sleep 3s

# Screenshot backup result
Screenshot docs/screenshots/backup-complete.png

# Verify a backup
Type "v"
Sleep 2s

# Screenshot verify result
Screenshot docs/screenshots/verify-result.png

# Quit
Type "q"
Sleep 500ms
