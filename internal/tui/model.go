package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mcdonaldj/codebak/internal/backup"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
)

// View represents the current view state
type View int

const (
	ProjectsView View = iota
	VersionsView
	DiffSelectView // Selecting versions to compare
	DiffResultView // Showing diff results
)

// ProjectItem represents a project in the list
type ProjectItem struct {
	Name       string
	Path       string
	Versions   int
	LastBackup time.Time
	TotalSize  int64
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
	view     View
	width    int
	height   int
	quitting bool

	// Projects view
	projects       []ProjectItem
	projectCursor  int
	selectedProject string

	// Versions view
	versions      []VersionItem
	versionCursor int

	// Diff view
	diffSelections []int       // Indices of selected versions for diff
	diffResult     *DiffResult // Result of diff comparison
	diffCursor     int         // Cursor in diff result view

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
	Quit    key.Binding
	Help    key.Binding
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
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// NewModel creates a new TUI model
func NewModel() (*Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	m := &Model{
		config: cfg,
		view:   ProjectsView,
	}

	if err := m.loadProjects(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadProjects loads all projects with their backup info
func (m *Model) loadProjects() error {
	sourceDir := config.ExpandPath(m.config.SourceDir)
	backupDir := config.ExpandPath(m.config.BackupDir)

	projects, err := backup.ListProjects(sourceDir)
	if err != nil {
		return err
	}

	m.projects = nil
	for _, name := range projects {
		item := ProjectItem{
			Name: name,
			Path: filepath.Join(sourceDir, name),
		}

		// Load manifest if exists
		mf, err := manifest.Load(backupDir, name)
		if err == nil && len(mf.Backups) > 0 {
			item.Versions = len(mf.Backups)
			latest := mf.LatestBackup()
			if latest != nil {
				item.LastBackup = latest.CreatedAt
			}
			for _, b := range mf.Backups {
				item.TotalSize += b.SizeBytes
			}
		}

		m.projects = append(m.projects, item)
	}

	return nil
}

// loadVersions loads backup versions for the selected project
func (m *Model) loadVersions() error {
	backupDir := config.ExpandPath(m.config.BackupDir)

	mf, err := manifest.Load(backupDir, m.selectedProject)
	if err != nil {
		return err
	}

	m.versions = nil
	for _, b := range mf.Backups {
		m.versions = append(m.versions, VersionItem{
			File:      b.File,
			Size:      b.SizeBytes,
			FileCount: b.FileCount,
			GitHead:   b.GitHead,
			CreatedAt: b.CreatedAt,
		})
	}

	// Reverse so newest is first
	for i, j := 0, len(m.versions)-1; i < j; i, j = i+1, j-1 {
		m.versions[i], m.versions[j] = m.versions[j], m.versions[i]
	}

	return nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		result := backup.BackupProject(m.config, project)
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

		// Verify using recovery package
		backupDir := config.ExpandPath(m.config.BackupDir)
		mf, err := manifest.Load(backupDir, project)
		if err != nil || len(mf.Backups) == 0 {
			return statusMsg{err: true, msg: "No backups to verify"}
		}

		latest := mf.LatestBackup()
		zipPath := filepath.Join(backupDir, project, latest.File)
		actualChecksum, err := manifest.ComputeSHA256(zipPath)
		if err != nil {
			return statusMsg{err: true, msg: fmt.Sprintf("Verify failed: %v", err)}
		}

		if actualChecksum != latest.SHA256 {
			return statusMsg{err: true, msg: "✗ Checksum mismatch!"}
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

		line := fmt.Sprintf("%s%-28s %8s %12s %s",
			cursor, truncate(p.Name, 28), versions, size, lastBackup)
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

	// Help
	help := "[↑/↓] navigate  [enter] versions  [r] backup  [v] verify  [q] quit"
	b.WriteString(helpStyle.Render(help))

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
	b.WriteString(helpStyle.Render(help))

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
	selected := len(m.diffSelections)
	if selected == 0 {
		b.WriteString(dimStyle.Render("Select first version..."))
	} else if selected == 1 {
		b.WriteString(dimStyle.Render("Select second version..."))
	}
	b.WriteString("\n")

	// Help
	help := "[↑/↓] navigate  [space] select  [esc] cancel"
	b.WriteString(helpStyle.Render(help))

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
	help := "[↑/↓] navigate  [esc] back  [q] quit"
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// Run starts the TUI
func Run() error {
	m, err := NewModel()
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

// Ensure we handle status messages
func (m *Model) handleStatusMsg(msg statusMsg) {
	m.statusMsg = msg.msg
	m.statusErr = msg.err
	// Reload projects to reflect changes
	_ = m.loadProjects()
	if m.view == VersionsView {
		_ = m.loadVersions()
	}
}
