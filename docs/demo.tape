# codebak demo - TUI walkthrough
# Run with: vhs docs/demo.tape

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

# Screenshot the projects view
Screenshot docs/screenshots/projects-view.png

# Navigate to website-redesign (first project)
Enter
Sleep 2s
Screenshot docs/screenshots/versions-view.png

# Navigate to diff view - press 'd' for diff
Type "d"
Sleep 1s
Screenshot docs/screenshots/diff-select.png

# Select versions and view diff result
Down
Sleep 500ms
Enter
Sleep 2s
Screenshot docs/screenshots/diff-result.png

# Drill into a file to see line-by-line diff
Down
Sleep 500ms
Enter
Sleep 2s
Screenshot docs/screenshots/file-diff.png

# Go back
Escape
Sleep 500ms
Escape
Sleep 500ms
Escape
Sleep 1s

# Run backup on current project
Type "r"
Sleep 3s
Screenshot docs/screenshots/backup-complete.png

# Verify a backup
Type "v"
Sleep 2s
Screenshot docs/screenshots/verify-result.png

# Quit
Type "q"
Sleep 500ms
