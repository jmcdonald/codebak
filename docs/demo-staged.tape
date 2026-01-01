# codebak demo - Staged data for compelling screenshots
#
# Prerequisites: Run ./scripts/stage-demo.sh first
#
# Usage:
#   ./scripts/stage-demo.sh
#   cp ~/.codebak/config.yaml ~/.codebak/config.yaml.bak  # backup real config
#   cp /tmp/codebak-demo/.codebak/config.yaml ~/.codebak/config.yaml  # use demo
#   vhs docs/demo-staged.tape
#   mv ~/.codebak/config.yaml.bak ~/.codebak/config.yaml  # restore

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

# Navigate to show variety
Down
Sleep 300ms

# Start diff mode
Type "d"
Sleep 1s

# Screenshot diff selection
Screenshot docs/screenshots/diff-select.png

# Select first version (newest with changes)
Sleep 500ms
Space
Sleep 500ms

# Move to second version
Down
Sleep 300ms

# Select second version
Space
Sleep 2s

# Screenshot diff result showing file changes
Screenshot docs/screenshots/diff-result.png

# Go back
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
