package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmcdonald/codebak/internal/adapters/tuisvc"
	"github.com/jmcdonald/codebak/internal/backup"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/ports"
)

// View represents the current view state
type View int

const (
	ProjectsView View = iota
	VersionsView
	DiffSelectView // Selecting versions to compare
	DiffResultView // Showing diff results (file list)
	FileDiffView   // Showing actual file content diff
	SettingsView    // Settings/configuration view
	MoveInputView   // Folder picker for move path
	MoveConfirmView // Confirmation before moving
)

// ProjectItem represents a project in the list
type ProjectItem struct {
	Name        string
	Path        string
	SourceLabel string
	SourceIcon  string
	Versions    int
	LastBackup  time.Time
	TotalSize   int64
}

// VersionItem represents a backup version
type VersionItem struct {
	File      string
	Size      int64
	FileCount int
	GitHead   string
	CreatedAt time.Time
}

// Model is the main TUI model
type Model struct {
	config   *config.Config
	service  ports.TUIService // Injected service for testability
	version  string           // Application version
	view     View
	width    int
	height   int
	quitting bool

	// Projects view
	projects        []ProjectItem
	projectCursor   int
	selectedProject string

	// Versions view
	versions      []VersionItem
	versionCursor int

	// Diff view
	diffSelections []int       // Indices of selected versions for diff
	diffResult     *DiffResult // Result of diff comparison
	diffCursor     int         // Cursor in diff result view

	// File diff view
	fileDiffResult *FileDiffResult // Line-by-line diff of selected file
	fileDiffScroll int             // Scroll offset in file diff view
	diffSwapped    bool            // Whether versions are swapped (v2 on left)

	// Settings view
	settingsCursor int
	prevView       View // View to return to after settings

	// Move input view (folder picker)
	folderPicker       filepicker.Model
	folderPickerHist   []string // History of visited directories
	folderPickerTyping bool     // True when typing path directly
	pathInput          textinput.Model
	pendingMovePath    string // Path selected, awaiting confirmation

	// Status message
	statusMsg string
	statusErr bool
}

// Key bindings
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Run     key.Binding
	Verify  key.Binding
	Recover key.Binding
	Diff    key.Binding
	Select  key.Binding
	Swap    key.Binding
	Quit     key.Binding
	Settings key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Run: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "run backup"),
	),
	Verify: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "verify"),
	),
	Recover: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "recover"),
	),
	Diff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff"),
	),
	Select: key.NewBinding(
		key.WithKeys(" ", "tab"),
		key.WithHelp("space", "select"),
	),
	Swap: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "swap"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Settings: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "settings"),
	),
}

// NewModel creates a new TUI model with default service.
func NewModel(version string) (*Model, error) {
	return NewModelWithService(version, tuisvc.New())
}

// NewModelWithService creates a new TUI model with a custom service.
// This allows dependency injection for testing.
func NewModelWithService(version string, svc ports.TUIService) (*Model, error) {
	cfg, err := svc.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	m := &Model{
		config:       cfg,
		service:      svc,
		version:      version,
		view:         ProjectsView,
		folderPicker: newFolderPicker(),
		pathInput:    newPathInput(),
	}

	if err := m.loadProjects(); err != nil {
		return nil, err
	}

	return m, nil
}

// newFolderPicker creates a folder picker for directory selection
func newFolderPicker() filepicker.Model {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.CurrentDirectory, _ = os.UserHomeDir()
	fp.Height = 12
	fp.AutoHeight = false
	// Customize empty directory message (this is a valid destination!)
	fp.Styles.EmptyDirectory = fp.Styles.EmptyDirectory.SetString("  (empty folder - press 's' to select)")
	return fp
}

// newPathInput creates a text input for typing paths directly
func newPathInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/folder or ~/folder"
	ti.CharLimit = 256
	ti.Width = 50
	return ti
}

// NewModelWithConfig creates a new TUI model with a provided config and service.
// This is useful for testing with pre-configured state.
func NewModelWithConfig(cfg *config.Config, svc ports.TUIService) *Model {
	return &Model{
		config:       cfg,
		service:      svc,
		version:      "test",
		view:         ProjectsView,
		folderPicker: newFolderPicker(),
		pathInput:    newPathInput(),
	}
}

// loadProjects loads all projects with their backup info
func (m *Model) loadProjects() error {
	projects, err := m.service.ListProjects(m.config)
	if err != nil {
		return err
	}

	m.projects = nil
	for _, p := range projects {
		m.projects = append(m.projects, ProjectItem{
			Name:        p.Name,
			Path:        p.Path,
			SourceLabel: p.SourceLabel,
			SourceIcon:  p.SourceIcon,
			Versions:    p.Versions,
			LastBackup:  p.LastBackup,
			TotalSize:   p.TotalSize,
		})
	}

	return nil
}

// loadVersions loads backup versions for the selected project
func (m *Model) loadVersions() error {
	versions, err := m.service.ListVersions(m.config, m.selectedProject)
	if err != nil {
		return err
	}

	m.versions = nil
	for _, v := range versions {
		m.versions = append(m.versions, VersionItem{
			File:      v.File,
			Size:      v.Size,
			FileCount: v.FileCount,
			GitHead:   v.GitHead,
			CreatedAt: v.CreatedAt,
		})
	}

	return nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle folder picker view for ALL message types (it needs readDirMsg etc)
	if m.view == MoveInputView {
		return m.handleFolderPicker(msg)
	}

	// Handle confirmation dialog
	if m.view == MoveConfirmView {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return m.handleMoveConfirm(keyMsg)
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case statusMsg:
		m.statusMsg = msg.msg
		m.statusErr = msg.err
		// Reload data to reflect changes
		_ = m.loadProjects()
		if m.view == VersionsView {
			_ = m.loadVersions()
		}
		return m, nil

	case diffMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Diff failed: %v", msg.err)
			m.statusErr = true
			m.view = VersionsView
			m.diffSelections = nil
		} else {
			m.diffResult = msg.result
			m.diffCursor = 0
			m.view = DiffResultView
			m.statusMsg = ""
		}
		return m, nil

	case fileDiffMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("File diff failed: %v", msg.err)
			m.statusErr = true
		} else {
			m.fileDiffResult = msg.result
			m.fileDiffScroll = 0
			m.view = FileDiffView
			m.statusMsg = ""
		}
		return m, nil

	case tea.KeyMsg:
		// Clear status on any key
		m.statusMsg = ""
		m.statusErr = false

		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Up):
			m.moveCursor(-1)

		case key.Matches(msg, keys.Down):
			m.moveCursor(1)

		case key.Matches(msg, keys.Enter):
			if m.view == ProjectsView && len(m.projects) > 0 {
				m.selectedProject = m.projects[m.projectCursor].Name
				if err := m.loadVersions(); err != nil {
					m.statusMsg = fmt.Sprintf("Error: %v", err)
					m.statusErr = true
				} else {
					m.view = VersionsView
					m.versionCursor = 0
				}
			} else if m.view == DiffResultView && m.diffResult != nil && len(m.diffResult.Changes) > 0 {
				// Drill into file diff
				change := m.diffResult.Changes[m.diffCursor]
				return m, m.computeFileDiff(change)
			} else if m.view == SettingsView {
				if cmd := m.handleSettingsSelect(); cmd != nil {
					return m, cmd
				}
			}

		case key.Matches(msg, keys.Back):
			switch m.view {
			case VersionsView:
				m.view = ProjectsView
				m.versions = nil
			case DiffSelectView:
				m.view = VersionsView
				m.diffSelections = nil
			case DiffResultView:
				m.view = VersionsView
				m.diffResult = nil
				m.diffCursor = 0
			case FileDiffView:
				m.view = DiffResultView
				m.fileDiffResult = nil
				m.fileDiffScroll = 0
			case SettingsView:
				m.view = m.prevView
			case MoveInputView:
				m.view = SettingsView
			}

		case key.Matches(msg, keys.Run):
			return m, m.runBackup()

		case key.Matches(msg, keys.Verify):
			return m, m.runVerify()

		case key.Matches(msg, keys.Diff):
			if m.view == VersionsView && len(m.versions) >= 2 {
				m.view = DiffSelectView
				m.diffSelections = nil
				m.statusMsg = "Select 2 versions to compare (space to select)"
			}

		case key.Matches(msg, keys.Select):
			if m.view == DiffSelectView {
				return m, m.toggleDiffSelection()
			}

		case key.Matches(msg, keys.Swap):
			if m.view == FileDiffView && m.fileDiffResult != nil {
				m.diffSwapped = !m.diffSwapped
			}

		case key.Matches(msg, keys.Settings):
			if m.view != SettingsView {
				m.prevView = m.view
				m.view = SettingsView
				m.settingsCursor = 0
			}
		}
	}

	return m, nil
}

func (m *Model) moveCursor(delta int) {
	switch m.view {
	case ProjectsView:
		m.projectCursor += delta
		if m.projectCursor < 0 {
			m.projectCursor = 0
		}
		if m.projectCursor >= len(m.projects) {
			m.projectCursor = len(m.projects) - 1
		}
	case VersionsView, DiffSelectView:
		m.versionCursor += delta
		if m.versionCursor < 0 {
			m.versionCursor = 0
		}
		if m.versionCursor >= len(m.versions) {
			m.versionCursor = len(m.versions) - 1
		}
	case DiffResultView:
		if m.diffResult != nil {
			m.diffCursor += delta
			if m.diffCursor < 0 {
				m.diffCursor = 0
			}
			if m.diffCursor >= len(m.diffResult.Changes) {
				m.diffCursor = len(m.diffResult.Changes) - 1
			}
		}
	case FileDiffView:
		if m.fileDiffResult != nil {
			m.fileDiffScroll += delta
			if m.fileDiffScroll < 0 {
				m.fileDiffScroll = 0
			}
			maxScroll := len(m.fileDiffResult.Lines) - (m.height - 10)
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.fileDiffScroll > maxScroll {
				m.fileDiffScroll = maxScroll
			}
		}
	case SettingsView:
		settingsCount := 4 // Number of settings options
		m.settingsCursor += delta
		if m.settingsCursor < 0 {
			m.settingsCursor = 0
		}
		if m.settingsCursor >= settingsCount {
			m.settingsCursor = settingsCount - 1
		}
	}
}

// handleSettingsSelect handles Enter key press in SettingsView
func (m *Model) handleSettingsSelect() tea.Cmd {
	switch m.settingsCursor {
	case 0: // Backup Directory - calculate size async
		m.statusMsg = fmt.Sprintf("📁 %s (calculating...)", m.config.BackupDir)
		return m.calculateBackupDirSize()
	case 1: // Color Theme
		m.statusMsg = "🎨 Theme: purple (default) — more themes coming in future release"
	case 2: // Migrate Backups
		m.folderPicker = newFolderPicker() // Reset picker
		m.folderPickerHist = nil           // Reset history
		m.folderPickerTyping = false       // Reset typing mode
		m.view = MoveInputView
		return m.folderPicker.Init()
	case 3: // About
		m.statusMsg = fmt.Sprintf("codebak v%s — Incremental Code Backup Tool", m.version)
	}
	return nil
}

// calculateBackupDirSize returns a Cmd that calculates size asynchronously
func (m *Model) calculateBackupDirSize() tea.Cmd {
	backupDir := m.config.BackupDir
	return func() tea.Msg {
		var totalSize int64
		_ = filepath.Walk(backupDir, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors, continue walking
			}
			if !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})
		return statusMsg{
			msg: fmt.Sprintf("📁 %s (%s used)", backupDir, backup.FormatSize(totalSize)),
			err: false,
		}
	}
}

// getBackupDirSize calculates total size of backup directory (sync, for confirmation view)
func (m *Model) getBackupDirSize() string {
	var totalSize int64
	_ = filepath.Walk(m.config.BackupDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return backup.FormatSize(totalSize)
}

// handleFolderPicker handles messages in MoveInputView (folder picker)
func (m *Model) handleFolderPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle typing mode separately
	if m.folderPickerTyping {
		return m.handlePathInput(msg)
	}

	// Check for special keys first
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "q", "ctrl+c":
			m.view = SettingsView
			return m, nil
		case "s", " ": // Select current directory
			m.pendingMovePath = m.folderPicker.CurrentDirectory
			m.view = MoveConfirmView
			return m, nil
		case "~": // Jump to home
			m.folderPickerHist = append(m.folderPickerHist, m.folderPicker.CurrentDirectory)
			m.folderPicker.CurrentDirectory, _ = os.UserHomeDir()
			return m, m.folderPicker.Init()
		case ".": // Jump to current backup dir
			m.folderPickerHist = append(m.folderPickerHist, m.folderPicker.CurrentDirectory)
			m.folderPicker.CurrentDirectory = m.config.BackupDir
			return m, m.folderPicker.Init()
		case "-": // Go back in history
			if len(m.folderPickerHist) > 0 {
				prev := m.folderPickerHist[len(m.folderPickerHist)-1]
				m.folderPickerHist = m.folderPickerHist[:len(m.folderPickerHist)-1]
				m.folderPicker.CurrentDirectory = prev
				return m, m.folderPicker.Init()
			}
		case "/", "g": // Enter typing mode
			m.folderPickerTyping = true
			m.pathInput.SetValue("")
			return m, m.pathInput.Focus() // Focus returns Cmd for cursor blink
		}
	}

	// Update folder picker
	var cmd tea.Cmd
	m.folderPicker, cmd = m.folderPicker.Update(msg)

	// Check if user selected a directory (by pressing enter on it)
	if didSelect, path := m.folderPicker.DidSelectFile(msg); didSelect {
		m.pendingMovePath = path
		m.view = MoveConfirmView
		return m, nil
	}

	return m, cmd
}

// handlePathInput handles typing mode in folder picker
func (m *Model) handlePathInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEsc:
			m.folderPickerTyping = false
			m.pathInput.Blur()
			return m, nil
		case tea.KeyEnter:
			path := m.pathInput.Value()
			if path == "" {
				m.folderPickerTyping = false
				m.pathInput.Blur()
				return m, nil
			}
			// Expand ~ if present
			if len(path) > 0 && path[0] == '~' {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			// Validate path exists and is directory
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				m.statusMsg = "Invalid path or not a directory"
				m.statusErr = true
				return m, nil
			}
			// Navigate to the path
			m.folderPickerHist = append(m.folderPickerHist, m.folderPicker.CurrentDirectory)
			m.folderPicker.CurrentDirectory = path
			m.folderPickerTyping = false
			m.pathInput.Blur()
			return m, m.folderPicker.Init()
		}
	}

	// Update text input
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

// handleMoveConfirm handles the confirmation dialog
func (m *Model) handleMoveConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.view = SettingsView
		return m, m.executeMoveBackups(m.pendingMovePath)
	case "n", "N", "esc", "q":
		m.view = MoveInputView
		return m, nil
	}
	return m, nil
}

// executeMoveBackups moves backups to new location
func (m *Model) executeMoveBackups(newPath string) tea.Cmd {
	return func() tea.Msg {
		// Expand ~ if present
		if len(newPath) > 0 && newPath[0] == '~' {
			home, err := os.UserHomeDir()
			if err != nil {
				return statusMsg{err: true, msg: fmt.Sprintf("Cannot expand ~: %v", err)}
			}
			newPath = filepath.Join(home, newPath[1:])
		}

		// Make absolute
		absPath, err := filepath.Abs(newPath)
		if err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Invalid path: %v", err)}
		}

		oldPath := m.config.BackupDir
		if oldPath == absPath {
			return statusMsg{msg: "Same path - nothing to move"}
		}

		// Check if old path exists
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			// No backups to move, just update config
			m.config.BackupDir = absPath
			if err := m.config.Save(); err != nil {
				return statusMsg{err: true, msg: fmt.Sprintf("Error saving config: %v", err)}
			}
			return statusMsg{msg: fmt.Sprintf("✓ Backup dir set to %s", absPath)}
		}

		// Create new directory
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Cannot create dir: %v", err)}
		}

		// Move all project directories
		entries, err := os.ReadDir(oldPath)
		if err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Cannot read source: %v", err)}
		}

		moved := 0
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			oldProjectPath := filepath.Join(oldPath, entry.Name())
			newProjectPath := filepath.Join(absPath, entry.Name())
			if err := os.Rename(oldProjectPath, newProjectPath); err != nil {
				return statusMsg{err: true, msg: fmt.Sprintf("Failed moving %s: %v", entry.Name(), err)}
			}
			moved++
		}

		// Update config
		m.config.BackupDir = absPath
		if err := m.config.Save(); err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Error saving config: %v", err)}
		}

		// Try to remove old empty directory
		_ = os.Remove(oldPath)

		return statusMsg{msg: fmt.Sprintf("✓ Moved %d projects to %s", moved, absPath)}
	}
}

func (m *Model) runBackup() tea.Cmd {
	return func() tea.Msg {
		var project string
		if m.view == ProjectsView && len(m.projects) > 0 {
			project = m.projects[m.projectCursor].Name
		} else if m.view == VersionsView {
			project = m.selectedProject
		}

		if project == "" {
			return statusMsg{err: true, msg: "No project selected"}
		}

		result := m.service.RunBackup(m.config, project)
		if result.Error != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Backup failed: %v", result.Error)}
		}
		if result.Skipped {
			return statusMsg{msg: fmt.Sprintf("%s: %s", project, result.Reason)}
		}
		return statusMsg{msg: fmt.Sprintf("✓ Backed up %s (%s)", project, backup.FormatSize(result.Size))}
	}
}

func (m *Model) runVerify() tea.Cmd {
	return func() tea.Msg {
		var project string
		if m.view == ProjectsView && len(m.projects) > 0 {
			project = m.projects[m.projectCursor].Name
		} else if m.view == VersionsView {
			project = m.selectedProject
		}

		if project == "" {
			return statusMsg{err: true, msg: "No project selected"}
		}

		if err := m.service.VerifyBackup(m.config, project); err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("✗ %v", err)}
		}

		return statusMsg{msg: fmt.Sprintf("✓ %s verified", project)}
	}
}

type statusMsg struct {
	msg string
	err bool
}

type diffMsg struct {
	result *DiffResult
	err    error
}

type fileDiffMsg struct {
	result *FileDiffResult
	err    error
}

func (m *Model) toggleDiffSelection() tea.Cmd {
	// Toggle selection for current version
	idx := m.versionCursor
	found := -1
	for i, sel := range m.diffSelections {
		if sel == idx {
			found = i
			break
		}
	}

	if found >= 0 {
		// Deselect
		m.diffSelections = append(m.diffSelections[:found], m.diffSelections[found+1:]...)
	} else {
		// Select (max 2)
		if len(m.diffSelections) < 2 {
			m.diffSelections = append(m.diffSelections, idx)
		}
	}

	// If we have 2 selections, compute diff
	if len(m.diffSelections) == 2 {
		v1 := m.versions[m.diffSelections[0]].File
		v2 := m.versions[m.diffSelections[1]].File
		return func() tea.Msg {
			result, err := ComputeDiff(m.config, m.selectedProject, v1, v2)
			return diffMsg{result: result, err: err}
		}
	}

	return nil
}

// View renders the UI
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	var content string
	switch m.view {
	case ProjectsView:
		content = m.renderProjectsView()
	case VersionsView:
		content = m.renderVersionsView()
	case DiffSelectView:
		content = m.renderDiffSelectView()
	case DiffResultView:
		content = m.renderDiffResultView()
	case FileDiffView:
		content = m.renderFileDiffView()
	case SettingsView:
		content = m.renderSettingsView()
	case MoveInputView:
		content = m.renderMoveInputView()
	case MoveConfirmView:
		content = m.renderMoveConfirmView()
	}

	return appStyle.Render(content)
}

func (m *Model) renderProjectsView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(" 📦 codebak ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("  %-28s %8s %12s %s",
		"PROJECT", "VERSIONS", "SIZE", "LAST BACKUP")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", 70)))
	b.WriteString("\n")

	// List items
	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := 0
	if m.projectCursor >= visibleHeight {
		start = m.projectCursor - visibleHeight + 1
	}

	for i := start; i < len(m.projects) && i < start+visibleHeight; i++ {
		p := m.projects[i]
		cursor := "  "
		style := normalStyle
		if i == m.projectCursor {
			cursor = "▸ "
			style = selectedStyle
		}

		versions := fmt.Sprintf("%d", p.Versions)
		if p.Versions == 0 {
			versions = "-"
		}

		size := backup.FormatSize(p.TotalSize)
		if p.TotalSize == 0 {
			size = "-"
		}

		lastBackup := "-"
		if !p.LastBackup.IsZero() {
			lastBackup = relativeTime(p.LastBackup)
		}

		// Show source icon if available
		icon := ""
		if p.SourceIcon != "" {
			icon = p.SourceIcon + " "
		}

		line := fmt.Sprintf("%s%s%-26s %8s %12s %s",
			cursor, icon, truncate(p.Name, 26), versions, size, lastBackup)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	// Pad to fixed height
	for i := len(m.projects); i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Status
	b.WriteString("\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
	}
	b.WriteString("\n")

	// Help (split footer with settings on right)
	help := "[↑/↓] navigate  [enter] versions  [r] backup  [v] verify  [q] quit"
	b.WriteString(renderSplitFooter(help, m.width))

	return b.String()
}

func (m *Model) renderVersionsView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(fmt.Sprintf(" 📦 %s ", m.selectedProject))
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(m.versions) == 0 {
		b.WriteString(dimStyle.Render("  No backups found"))
		b.WriteString("\n\n")
	} else {
		// Header
		header := fmt.Sprintf("  %-18s %10s %8s %10s",
			"VERSION", "SIZE", "FILES", "GIT HEAD")
		b.WriteString(dimStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(strings.Repeat("─", 60)))
		b.WriteString("\n")

		// List items
		visibleHeight := m.height - 10
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		start := 0
		if m.versionCursor >= visibleHeight {
			start = m.versionCursor - visibleHeight + 1
		}

		for i := start; i < len(m.versions) && i < start+visibleHeight; i++ {
			v := m.versions[i]
			cursor := "  "
			style := normalStyle
			if i == m.versionCursor {
				cursor = "▸ "
				style = selectedStyle
			}

			version := strings.TrimSuffix(v.File, ".zip")
			gitHead := v.GitHead
			if len(gitHead) > 7 {
				gitHead = gitHead[:7]
			}
			if gitHead == "" {
				gitHead = "-"
			}

			line := fmt.Sprintf("%s%-18s %10s %8d %10s",
				cursor, version, backup.FormatSize(v.Size), v.FileCount, gitHead)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	// Pad to fixed height
	visibleHeight := m.height - 10
	for i := len(m.versions); i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Status
	b.WriteString("\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] navigate  [d] diff  [esc] back  [r] backup  [v] verify  [q] quit"
	b.WriteString(renderSplitFooter(help, m.width))

	return b.String()
}

func (m *Model) renderDiffSelectView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(fmt.Sprintf(" 🔍 %s - Select versions to compare ", m.selectedProject))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("     %-18s %10s %8s %10s",
		"VERSION", "SIZE", "FILES", "GIT HEAD")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", 60)))
	b.WriteString("\n")

	// Check if index is selected
	isSelected := func(idx int) bool {
		for _, sel := range m.diffSelections {
			if sel == idx {
				return true
			}
		}
		return false
	}

	// List items
	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := 0
	if m.versionCursor >= visibleHeight {
		start = m.versionCursor - visibleHeight + 1
	}

	for i := start; i < len(m.versions) && i < start+visibleHeight; i++ {
		v := m.versions[i]
		cursor := "  "
		style := normalStyle
		checkbox := "[ ]"

		if i == m.versionCursor {
			cursor = "▸ "
			style = selectedStyle
		}
		if isSelected(i) {
			checkbox = "[✓]"
		}

		version := strings.TrimSuffix(v.File, ".zip")
		gitHead := v.GitHead
		if len(gitHead) > 7 {
			gitHead = gitHead[:7]
		}
		if gitHead == "" {
			gitHead = "-"
		}

		line := fmt.Sprintf("%s%s %-18s %10s %8d %10s",
			cursor, checkbox, version, backup.FormatSize(v.Size), v.FileCount, gitHead)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	// Pad to fixed height
	for i := len(m.versions); i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Status
	b.WriteString("\n")
	switch len(m.diffSelections) {
	case 0:
		b.WriteString(dimStyle.Render("Select first version..."))
	case 1:
		b.WriteString(dimStyle.Render("Select second version..."))
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] navigate  [space] select  [esc] cancel"
	b.WriteString(renderSplitFooter(help, m.width))

	return b.String()
}

func (m *Model) renderDiffResultView() string {
	var b strings.Builder

	if m.diffResult == nil {
		return "Loading..."
	}

	// Title
	title := titleStyle.Render(fmt.Sprintf(" 📊 Diff: %s vs %s ",
		m.diffResult.Version1, m.diffResult.Version2))
	b.WriteString(title)
	b.WriteString("\n\n")

	// Summary
	summary := fmt.Sprintf("  Modified: %d   Added: %d   Deleted: %d",
		m.diffResult.Modified, m.diffResult.Added, m.diffResult.Deleted)
	b.WriteString(dimStyle.Render(summary))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", 70)))
	b.WriteString("\n")

	if len(m.diffResult.Changes) == 0 {
		b.WriteString(dimStyle.Render("  No differences found"))
		b.WriteString("\n")
	} else {
		// List changes
		visibleHeight := m.height - 10
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		start := 0
		if m.diffCursor >= visibleHeight {
			start = m.diffCursor - visibleHeight + 1
		}

		for i := start; i < len(m.diffResult.Changes) && i < start+visibleHeight; i++ {
			c := m.diffResult.Changes[i]
			cursor := "  "
			style := normalStyle
			if i == m.diffCursor {
				cursor = "▸ "
				style = selectedStyle
			}

			var statusIcon string
			switch c.Status {
			case 'M':
				statusIcon = "M"
			case 'A':
				statusIcon = "A"
			case 'D':
				statusIcon = "D"
			}

			line := fmt.Sprintf("%s%s %s", cursor, statusIcon, c.Path)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
	}

	// Pad to fixed height
	visibleHeight := m.height - 10
	for i := len(m.diffResult.Changes); i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Status
	b.WriteString("\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] navigate  [enter] view diff  [esc] back  [q] quit"
	b.WriteString(renderSplitFooter(help, m.width))

	return b.String()
}

func (m *Model) computeFileDiff(change FileChange) tea.Cmd {
	return func() tea.Msg {
		result, err := ComputeFileDiff(
			m.config,
			m.selectedProject,
			m.diffResult.Version1,
			m.diffResult.Version2,
			change.Path,
			change.Status,
		)
		return fileDiffMsg{result: result, err: err}
	}
}

func (m *Model) renderFileDiffView() string {
	var b strings.Builder

	if m.fileDiffResult == nil {
		return "Loading..."
	}

	// Title with file path
	v1, v2 := m.fileDiffResult.Version1, m.fileDiffResult.Version2
	if m.diffSwapped {
		v1, v2 = v2, v1
	}
	title := titleStyle.Render(fmt.Sprintf(" 📄 %s ", m.fileDiffResult.Path))
	b.WriteString(title)
	b.WriteString("\n")

	// Version headers
	header := fmt.Sprintf("  %-35s │ %-35s", v1, v2)
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", 75)))
	b.WriteString("\n")

	// Handle special cases
	if m.fileDiffResult.Error != "" {
		b.WriteString(errorBadge.Render(m.fileDiffResult.Error))
		b.WriteString("\n")
	} else if m.fileDiffResult.IsBinary {
		b.WriteString(dimStyle.Render("  Binary file - content diff not available"))
		b.WriteString("\n")
	} else if len(m.fileDiffResult.Lines) == 0 {
		b.WriteString(dimStyle.Render("  No differences"))
		b.WriteString("\n")
	} else {
		// Render diff lines
		visibleHeight := m.height - 12
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		endIdx := m.fileDiffScroll + visibleHeight
		if endIdx > len(m.fileDiffResult.Lines) {
			endIdx = len(m.fileDiffResult.Lines)
		}

		for i := m.fileDiffScroll; i < endIdx; i++ {
			line := m.fileDiffResult.Lines[i]

			// Format line numbers
			ln1 := "   "
			ln2 := "   "
			if line.LineNum1 > 0 {
				ln1 = fmt.Sprintf("%3d", line.LineNum1)
			}
			if line.LineNum2 > 0 {
				ln2 = fmt.Sprintf("%3d", line.LineNum2)
			}

			// Swap if needed
			if m.diffSwapped {
				ln1, ln2 = ln2, ln1
			}

			// Truncate content for display
			content := line.Content
			maxWidth := 60
			if len(content) > maxWidth {
				content = content[:maxWidth-3] + "..."
			}

			// Style based on change type
			var lineStr string
			switch line.Type {
			case '+':
				lineStr = fmt.Sprintf("%s  + │ %s  + %s", ln1, ln2, content)
				b.WriteString(addedStyle.Render(lineStr))
			case '-':
				lineStr = fmt.Sprintf("%s  - │ %s  - %s", ln1, ln2, content)
				b.WriteString(deletedStyle.Render(lineStr))
			default:
				lineStr = fmt.Sprintf("%s    │ %s    %s", ln1, ln2, content)
				b.WriteString(dimStyle.Render(lineStr))
			}
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(m.fileDiffResult.Lines) > visibleHeight {
			scrollInfo := fmt.Sprintf("  Lines %d-%d of %d",
				m.fileDiffScroll+1, endIdx, len(m.fileDiffResult.Lines))
			b.WriteString(dimStyle.Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	// Status
	b.WriteString("\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] scroll  [s] swap sides  [esc] back  [q] quit"
	b.WriteString(renderSplitFooter(help, m.width))

	return b.String()
}

func (m *Model) renderSettingsView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(" ⚙ Settings ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Settings options with dynamic values
	backupDirValue := truncatePath(m.config.BackupDir, 35)
	settings := []struct {
		name        string
		description string
		value       string
	}{
		{"Backup Directory", "Where backups are stored", backupDirValue},
		{"Color Theme", "UI color scheme", "purple (default)"},
		{"Migrate Backups", "Move backups to new location", "codebak move <path>"},
		{"About", "Version and info", fmt.Sprintf("v%s", m.version)},
	}

	for i, s := range settings {
		style := normalStyle
		prefix := "  "
		if i == m.settingsCursor {
			style = selectedStyle
			prefix = "▸ "
		}

		line := fmt.Sprintf("%s%-20s %s", prefix, s.name, dimStyle.Render(s.value))
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("    %s", s.description)))
		b.WriteString("\n\n")
	}

	// Status message area
	b.WriteString("\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] navigate  [enter] select  [esc] back"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m *Model) renderMoveInputView() string {
	var b strings.Builder

	// Title with current path
	title := titleStyle.Render(" 📁 Select New Backup Location ")
	b.WriteString(title)
	b.WriteString("\n")

	// Current info
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Current: %s", truncatePath(m.config.BackupDir, 50))))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Browsing: %s", m.folderPicker.CurrentDirectory)))
	b.WriteString("\n\n")

	// Path input (typing mode) or folder picker
	if m.folderPickerTyping {
		b.WriteString("  ")
		b.WriteString(m.pathInput.View())
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("  Press Enter to navigate, Esc to cancel"))
		b.WriteString("\n\n")
	} else {
		// Folder picker
		b.WriteString(m.folderPicker.View())
	}

	// Status message area
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString(errorBadge.Render(m.statusMsg))
		} else {
			b.WriteString(successBadge.Render(m.statusMsg))
		}
		b.WriteString("\n")
	}

	// Help
	var help string
	if m.folderPickerTyping {
		help = "[enter] go  [esc] cancel"
	} else {
		help = "[↑/↓] navigate  [enter] open  [s] select  [/] type path  [~] home  [.] backups  [-] prev  [q] cancel"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m *Model) renderMoveConfirmView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(" ⚠️  Confirm Move ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// From/To paths
	b.WriteString(dimStyle.Render("  From:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("    %s\n", m.config.BackupDir))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  To:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("    %s\n", m.pendingMovePath))
	b.WriteString("\n")

	// Stats
	projectCount := len(m.projects)
	totalSize := m.getBackupDirSize()
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Projects: %d", projectCount)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  Total size: %s", totalSize)))
	b.WriteString("\n\n")

	// Warning
	b.WriteString("  This will move all backups to the new location.\n")
	b.WriteString("\n")

	// Confirmation prompt
	b.WriteString("  ")
	b.WriteString(selectedStyle.Render("[y] Confirm"))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render("[n] Cancel"))
	b.WriteString("\n")

	return b.String()
}

// truncatePath shortens a path for display by replacing middle with ...
func truncatePath(path string, max int) string {
	if len(path) <= max {
		return path
	}
	// Keep first ~10 and last portion
	if max > 15 {
		keep := (max - 3) / 2
		return path[:keep] + "..." + path[len(path)-keep:]
	}
	return path[:max-3] + "..."
}

// Run starts the TUI
func Run(version string) error {
	m, err := NewModel(version)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// Helper functions
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func relativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
