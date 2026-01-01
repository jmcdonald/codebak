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

# Navigate down through projects
Down
Sleep 500ms
Down
Sleep 500ms

# Select a project to see versions
Enter
Sleep 2s
Screenshot docs/screenshots/versions-view.png

# Navigate versions
Down
Sleep 500ms
Down
Sleep 500ms

# Run backup on current project
Type "r"
Sleep 3s
Screenshot docs/screenshots/backup-complete.png

# Go back to projects
Escape
Sleep 1s

# Verify a backup
Type "v"
Sleep 2s
Screenshot docs/screenshots/verify-result.png

# Quit
Type "q"
Sleep 500ms
