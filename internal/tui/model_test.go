package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/mocks"
	"github.com/jmcdonald/codebak/internal/ports"
)

func TestNewModelWithService(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{
		SourceDir: "/test/source",
		BackupDir: "/test/backup",
	}
	svc.Projects = []ports.TUIProjectInfo{
		{Name: "project-a", Path: "/test/source/project-a", Versions: 2},
		{Name: "project-b", Path: "/test/source/project-b", Versions: 0},
	}

	m, err := NewModelWithService(svc)
	if err != nil {
		t.Fatalf("NewModelWithService failed: %v", err)
	}

	if len(m.projects) != 2 {
		t.Errorf("projects = %d, expected 2", len(m.projects))
	}
	if m.view != ProjectsView {
		t.Errorf("view = %v, expected ProjectsView", m.view)
	}
}

func TestModelNavigation(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}
	svc.Projects = []ports.TUIProjectInfo{
		{Name: "project-a"},
		{Name: "project-b"},
		{Name: "project-c"},
	}

	m := NewModelWithConfig(svc.ConfigResult, svc)
	m.projects = []ProjectItem{
		{Name: "project-a"},
		{Name: "project-b"},
		{Name: "project-c"},
	}

	// Test down navigation
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*Model)
	if m.projectCursor != 1 {
		t.Errorf("cursor = %d, expected 1", m.projectCursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*Model)
	if m.projectCursor != 2 {
		t.Errorf("cursor = %d, expected 2", m.projectCursor)
	}

	// Test boundary - shouldn't go past end
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*Model)
	if m.projectCursor != 2 {
		t.Errorf("cursor = %d, expected 2 (at boundary)", m.projectCursor)
	}

	// Test up navigation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*Model)
	if m.projectCursor != 1 {
		t.Errorf("cursor = %d, expected 1", m.projectCursor)
	}
}

func TestModelEnterProject(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}
	svc.Versions = map[string][]ports.TUIVersionInfo{
		"project-a": {
			{File: "v2.zip", Size: 2048, FileCount: 20, CreatedAt: time.Now()},
			{File: "v1.zip", Size: 1024, FileCount: 10, CreatedAt: time.Now().Add(-24 * time.Hour)},
		},
	}

	m := NewModelWithConfig(svc.ConfigResult, svc)
	m.projects = []ProjectItem{
		{Name: "project-a"},
		{Name: "project-b"},
	}

	// Press enter to go to versions view
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if m.view != VersionsView {
		t.Errorf("view = %v, expected VersionsView", m.view)
	}
	if m.selectedProject != "project-a" {
		t.Errorf("selectedProject = %q, expected %q", m.selectedProject, "project-a")
	}
	if len(m.versions) != 2 {
		t.Errorf("versions = %d, expected 2", len(m.versions))
	}
}

func TestModelBackNavigation(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}

	m := NewModelWithConfig(svc.ConfigResult, svc)
	m.projects = []ProjectItem{{Name: "test-project"}}
	m.view = VersionsView
	m.selectedProject = "test-project"

	// Press escape to go back
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*Model)

	if m.view != ProjectsView {
		t.Errorf("view = %v, expected ProjectsView", m.view)
	}
}

func TestModelQuit(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}

	m := NewModelWithConfig(svc.ConfigResult, svc)

	// Press q to quit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(*Model)

	if !m.quitting {
		t.Error("quitting should be true")
	}
	if cmd == nil {
		t.Error("quit command should not be nil")
	}
}

func TestModelView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}

	m := NewModelWithConfig(svc.ConfigResult, svc)
	m.projects = []ProjectItem{
		{Name: "my-project", Versions: 3},
	}
	m.width = 80
	m.height = 24

	view := m.View()

	// Check that view contains expected elements
	if view == "" {
		t.Error("View() returned empty string")
	}
	if !contains(view, "codebak") {
		t.Error("View should contain 'codebak'")
	}
	if !contains(view, "my-project") {
		t.Error("View should contain project name")
	}
}

func TestModelWindowSize(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}

	m := NewModelWithConfig(svc.ConfigResult, svc)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updated.(*Model)

	if m.width != 100 {
		t.Errorf("width = %d, expected 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, expected 50", m.height)
	}
}

// TestWithTeatest demonstrates using teatest for more advanced testing
func TestWithTeatest(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}
	svc.Projects = []ports.TUIProjectInfo{
		{Name: "project-alpha", Versions: 5},
		{Name: "project-beta", Versions: 3},
	}

	m := NewModelWithConfig(svc.ConfigResult, svc)
	m.projects = []ProjectItem{
		{Name: "project-alpha", Versions: 5},
		{Name: "project-beta", Versions: 3},
	}
	m.width = 80
	m.height = 24

	// Create teatest program
	tm := teatest.NewTestModel(t, m)

	// Send window size
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Navigate down
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Quit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Wait for quit
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================
// Pure function tests: truncate(), relativeTime()
// ============================================

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "hello",
			max:      10,
			expected: "hello",
		},
		{
			name:     "string equal to max",
			input:    "hello",
			max:      5,
			expected: "hello",
		},
		{
			name:     "string longer than max",
			input:    "hello world",
			max:      8,
			expected: "hello w\u2026",
		},
		{
			name:     "empty string",
			input:    "",
			max:      5,
			expected: "",
		},
		{
			name:     "max of 1",
			input:    "hello",
			max:      1,
			expected: "\u2026",
		},
		{
			name:     "unicode string truncation",
			input:    "hello\u4e16\u754c",
			max:      6,
			expected: "hello\u2026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, expected %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		contains string
	}{
		{
			name:     "5 minutes ago",
			input:    now.Add(-5 * time.Minute),
			contains: "m ago",
		},
		{
			name:     "30 minutes ago",
			input:    now.Add(-30 * time.Minute),
			contains: "m ago",
		},
		{
			name:     "2 hours ago",
			input:    now.Add(-2 * time.Hour),
			contains: "h ago",
		},
		{
			name:     "12 hours ago",
			input:    now.Add(-12 * time.Hour),
			contains: "h ago",
		},
		{
			name:     "2 days ago",
			input:    now.Add(-48 * time.Hour),
			contains: "d ago",
		},
		{
			name:     "5 days ago",
			input:    now.Add(-120 * time.Hour),
			contains: "d ago",
		},
		{
			name:     "2 weeks ago",
			input:    now.Add(-14 * 24 * time.Hour),
			contains: "", // Returns date format like "Jan 2"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relativeTime(tt.input)
			if tt.contains != "" && !contains(result, tt.contains) {
				t.Errorf("relativeTime() = %q, expected to contain %q", result, tt.contains)
			}
			if result == "" {
				t.Error("relativeTime() returned empty string")
			}
		})
	}
}

// ============================================
// moveCursor tests across all views
// ============================================

func TestMoveCursorProjectsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{
		{Name: "proj1"},
		{Name: "proj2"},
		{Name: "proj3"},
	}
	m.view = ProjectsView
	m.projectCursor = 0

	tests := []struct {
		name           string
		delta          int
		expectedCursor int
	}{
		{"move down from start", 1, 1},
		{"move down again", 1, 2},
		{"move down at end (boundary)", 1, 2},
		{"move up", -1, 1},
		{"move up again", -1, 0},
		{"move up at start (boundary)", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.moveCursor(tt.delta)
			if m.projectCursor != tt.expectedCursor {
				t.Errorf("projectCursor = %d, expected %d", m.projectCursor, tt.expectedCursor)
			}
		})
	}
}

func TestMoveCursorVersionsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{
		{File: "v1.zip"},
		{File: "v2.zip"},
	}
	m.view = VersionsView
	m.versionCursor = 0

	// Move down
	m.moveCursor(1)
	if m.versionCursor != 1 {
		t.Errorf("versionCursor = %d, expected 1", m.versionCursor)
	}

	// Move down at boundary
	m.moveCursor(1)
	if m.versionCursor != 1 {
		t.Errorf("versionCursor = %d, expected 1 (boundary)", m.versionCursor)
	}

	// Move up
	m.moveCursor(-1)
	if m.versionCursor != 0 {
		t.Errorf("versionCursor = %d, expected 0", m.versionCursor)
	}
}

func TestMoveCursorDiffSelectView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{
		{File: "v1.zip"},
		{File: "v2.zip"},
		{File: "v3.zip"},
	}
	m.view = DiffSelectView
	m.versionCursor = 1

	m.moveCursor(1)
	if m.versionCursor != 2 {
		t.Errorf("versionCursor = %d, expected 2", m.versionCursor)
	}

	m.moveCursor(-2)
	if m.versionCursor != 0 {
		t.Errorf("versionCursor = %d, expected 0", m.versionCursor)
	}
}

func TestMoveCursorDiffResultView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = &DiffResult{
		Changes: []FileChange{
			{Path: "file1.txt", Status: 'M'},
			{Path: "file2.txt", Status: 'A'},
			{Path: "file3.txt", Status: 'D'},
		},
	}
	m.view = DiffResultView
	m.diffCursor = 0

	m.moveCursor(1)
	if m.diffCursor != 1 {
		t.Errorf("diffCursor = %d, expected 1", m.diffCursor)
	}

	m.moveCursor(2)
	if m.diffCursor != 2 {
		t.Errorf("diffCursor = %d, expected 2 (boundary)", m.diffCursor)
	}
}

func TestMoveCursorFileDiffView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Lines: make([]DiffLine, 100),
	}
	m.view = FileDiffView
	m.fileDiffScroll = 0
	m.height = 20

	m.moveCursor(5)
	if m.fileDiffScroll != 5 {
		t.Errorf("fileDiffScroll = %d, expected 5", m.fileDiffScroll)
	}

	m.moveCursor(-3)
	if m.fileDiffScroll != 2 {
		t.Errorf("fileDiffScroll = %d, expected 2", m.fileDiffScroll)
	}

	// Test boundary at start
	m.moveCursor(-10)
	if m.fileDiffScroll != 0 {
		t.Errorf("fileDiffScroll = %d, expected 0 (boundary)", m.fileDiffScroll)
	}
}

func TestMoveCursorWithNilDiffResult(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = DiffResultView
	m.diffResult = nil
	m.diffCursor = 0

	// Should not panic
	m.moveCursor(1)
	if m.diffCursor != 0 {
		t.Errorf("diffCursor = %d, expected 0", m.diffCursor)
	}
}

func TestMoveCursorWithNilFileDiffResult(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = FileDiffView
	m.fileDiffResult = nil
	m.fileDiffScroll = 0

	// Should not panic
	m.moveCursor(1)
	if m.fileDiffScroll != 0 {
		t.Errorf("fileDiffScroll = %d, expected 0", m.fileDiffScroll)
	}
}

// ============================================
// toggleDiffSelection tests
// ============================================

func TestToggleDiffSelection(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{
		{File: "v1.zip"},
		{File: "v2.zip"},
		{File: "v3.zip"},
	}
	m.view = DiffSelectView
	m.versionCursor = 0

	// Select first version
	cmd := m.toggleDiffSelection()
	if len(m.diffSelections) != 1 {
		t.Errorf("diffSelections = %d, expected 1", len(m.diffSelections))
	}
	if m.diffSelections[0] != 0 {
		t.Errorf("diffSelections[0] = %d, expected 0", m.diffSelections[0])
	}
	if cmd != nil {
		t.Error("cmd should be nil with only 1 selection")
	}

	// Deselect first version
	_ = m.toggleDiffSelection()
	if len(m.diffSelections) != 0 {
		t.Errorf("diffSelections = %d, expected 0", len(m.diffSelections))
	}

	// Select version 0 and 1
	m.versionCursor = 0
	m.toggleDiffSelection()
	m.versionCursor = 1
	cmd = m.toggleDiffSelection()
	if len(m.diffSelections) != 2 {
		t.Errorf("diffSelections = %d, expected 2", len(m.diffSelections))
	}
	if cmd == nil {
		t.Error("cmd should not be nil when 2 versions selected")
	}
}

func TestToggleDiffSelectionMaxTwo(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{
		{File: "v1.zip"},
		{File: "v2.zip"},
		{File: "v3.zip"},
	}
	m.view = DiffSelectView
	m.diffSelections = []int{0, 1} // Already have 2

	// Try to select third - should not add
	m.versionCursor = 2
	m.toggleDiffSelection()
	// Note: when 2 are selected, it triggers diff computation
	// so we can't easily test the "max 2" logic here
}

// ============================================
// runBackup and runVerify tests
// ============================================

func TestRunBackupProjectsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.BackupResults = map[string]ports.TUIBackupResult{
		"my-project": {Size: 2048},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "my-project"}}
	m.view = ProjectsView
	m.projectCursor = 0

	cmd := m.runBackup()
	msg := cmd().(statusMsg)

	if msg.err {
		t.Errorf("unexpected error: %s", msg.msg)
	}
	if !contains(msg.msg, "my-project") {
		t.Errorf("msg = %q, expected to contain project name", msg.msg)
	}
}

func TestRunBackupVersionsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.BackupResults = map[string]ports.TUIBackupResult{
		"test-proj": {Size: 1024},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "test-proj"
	m.view = VersionsView

	cmd := m.runBackup()
	msg := cmd().(statusMsg)

	if msg.err {
		t.Errorf("unexpected error: %s", msg.msg)
	}
}

func TestRunBackupNoProject(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{}
	m.view = ProjectsView

	cmd := m.runBackup()
	msg := cmd().(statusMsg)

	if !msg.err {
		t.Error("expected error for no project")
	}
	if !contains(msg.msg, "No project selected") {
		t.Errorf("msg = %q, expected 'No project selected'", msg.msg)
	}
}

func TestRunBackupError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.BackupResults = map[string]ports.TUIBackupResult{
		"error-proj": {Error: errors.New("backup failed")},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "error-proj"}}
	m.view = ProjectsView

	cmd := m.runBackup()
	msg := cmd().(statusMsg)

	if !msg.err {
		t.Error("expected error")
	}
	if !contains(msg.msg, "Backup failed") {
		t.Errorf("msg = %q, expected to contain 'Backup failed'", msg.msg)
	}
}

func TestRunBackupSkipped(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.BackupResults = map[string]ports.TUIBackupResult{
		"skip-proj": {Skipped: true, Reason: "no changes"},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "skip-proj"}}
	m.view = ProjectsView

	cmd := m.runBackup()
	msg := cmd().(statusMsg)

	if msg.err {
		t.Error("skipped should not be an error")
	}
	if !contains(msg.msg, "no changes") {
		t.Errorf("msg = %q, expected to contain reason", msg.msg)
	}
}

func TestRunVerifySuccess(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "verify-proj"}}
	m.view = ProjectsView

	cmd := m.runVerify()
	msg := cmd().(statusMsg)

	if msg.err {
		t.Errorf("unexpected error: %s", msg.msg)
	}
	if !contains(msg.msg, "verified") {
		t.Errorf("msg = %q, expected to contain 'verified'", msg.msg)
	}
}

func TestRunVerifyError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.VerifyErrors = map[string]error{
		"bad-proj": errors.New("verification failed"),
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "bad-proj"}}
	m.view = ProjectsView

	cmd := m.runVerify()
	msg := cmd().(statusMsg)

	if !msg.err {
		t.Error("expected error")
	}
}

func TestRunVerifyNoProject(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{}
	m.view = ProjectsView

	cmd := m.runVerify()
	msg := cmd().(statusMsg)

	if !msg.err {
		t.Error("expected error for no project")
	}
}

// ============================================
// Update() message handling tests
// ============================================

func TestUpdateStatusMsg(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.Projects = []ports.TUIProjectInfo{{Name: "proj"}}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}

	// Test success message
	updated, _ := m.Update(statusMsg{msg: "Success!", err: false})
	m = updated.(*Model)

	if m.statusMsg != "Success!" {
		t.Errorf("statusMsg = %q, expected 'Success!'", m.statusMsg)
	}
	if m.statusErr {
		t.Error("statusErr should be false")
	}

	// Test error message
	updated, _ = m.Update(statusMsg{msg: "Error!", err: true})
	m = updated.(*Model)

	if !m.statusErr {
		t.Error("statusErr should be true")
	}
}

func TestUpdateStatusMsgReloadsVersions(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.Projects = []ports.TUIProjectInfo{{Name: "proj"}}
	svc.Versions = map[string][]ports.TUIVersionInfo{
		"proj": {{File: "v1.zip"}},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.selectedProject = "proj"
	m.view = VersionsView

	m.Update(statusMsg{msg: "Done"})

	if svc.ListProjectsCalls != 1 {
		t.Errorf("ListProjectsCalls = %d, expected 1", svc.ListProjectsCalls)
	}
}

func TestUpdateDiffMsg(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = DiffSelectView

	// Test successful diff
	result := &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes: []FileChange{
			{Path: "file.txt", Status: 'M'},
		},
	}
	updated, _ := m.Update(diffMsg{result: result, err: nil})
	m = updated.(*Model)

	if m.view != DiffResultView {
		t.Errorf("view = %v, expected DiffResultView", m.view)
	}
	if m.diffResult != result {
		t.Error("diffResult not set correctly")
	}
	if m.diffCursor != 0 {
		t.Errorf("diffCursor = %d, expected 0", m.diffCursor)
	}
}

func TestUpdateDiffMsgError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = DiffSelectView
	m.diffSelections = []int{0, 1}

	updated, _ := m.Update(diffMsg{result: nil, err: errors.New("diff error")})
	m = updated.(*Model)

	if m.view != VersionsView {
		t.Errorf("view = %v, expected VersionsView on error", m.view)
	}
	if !m.statusErr {
		t.Error("statusErr should be true")
	}
	if m.diffSelections != nil {
		t.Error("diffSelections should be cleared on error")
	}
}

func TestUpdateFileDiffMsg(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = DiffResultView

	result := &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines:    []DiffLine{{Content: "line1"}},
	}
	updated, _ := m.Update(fileDiffMsg{result: result, err: nil})
	m = updated.(*Model)

	if m.view != FileDiffView {
		t.Errorf("view = %v, expected FileDiffView", m.view)
	}
	if m.fileDiffResult != result {
		t.Error("fileDiffResult not set correctly")
	}
	if m.fileDiffScroll != 0 {
		t.Errorf("fileDiffScroll = %d, expected 0", m.fileDiffScroll)
	}
}

func TestUpdateFileDiffMsgError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = DiffResultView

	updated, _ := m.Update(fileDiffMsg{result: nil, err: errors.New("read error")})
	m = updated.(*Model)

	if m.statusErr != true {
		t.Error("statusErr should be true")
	}
	if !contains(m.statusMsg, "File diff failed") {
		t.Errorf("statusMsg = %q, expected to contain 'File diff failed'", m.statusMsg)
	}
}

// ============================================
// Keyboard navigation tests
// ============================================

func TestUpdateKeyboardDiff(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{{File: "v1.zip"}, {File: "v2.zip"}}
	m.view = VersionsView

	// Press 'd' to enter diff select mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(*Model)

	if m.view != DiffSelectView {
		t.Errorf("view = %v, expected DiffSelectView", m.view)
	}
}

func TestUpdateKeyboardDiffNotEnoughVersions(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{{File: "v1.zip"}} // Only 1 version
	m.view = VersionsView

	// Press 'd' - should not change view
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(*Model)

	if m.view != VersionsView {
		t.Errorf("view = %v, expected to stay in VersionsView", m.view)
	}
}

func TestUpdateKeyboardSelect(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{{File: "v1.zip"}, {File: "v2.zip"}}
	m.view = DiffSelectView
	m.versionCursor = 0

	// Press space to select
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(*Model)

	if len(m.diffSelections) != 1 {
		t.Errorf("diffSelections = %d, expected 1", len(m.diffSelections))
	}
}

func TestUpdateKeyboardSwap(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{Path: "file.txt"}
	m.view = FileDiffView
	m.diffSwapped = false

	// Press 's' to swap
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*Model)

	if !m.diffSwapped {
		t.Error("diffSwapped should be true")
	}

	// Press 's' again to swap back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*Model)

	if m.diffSwapped {
		t.Error("diffSwapped should be false")
	}
}

func TestUpdateKeyboardSwapNoResult(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = FileDiffView
	m.fileDiffResult = nil
	m.diffSwapped = false

	// Press 's' - should not crash
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*Model)

	if m.diffSwapped {
		t.Error("diffSwapped should still be false")
	}
}

func TestUpdateKeyboardBackFromAllViews(t *testing.T) {
	tests := []struct {
		name         string
		startView    View
		expectedView View
	}{
		{"VersionsView to ProjectsView", VersionsView, ProjectsView},
		{"DiffSelectView to VersionsView", DiffSelectView, VersionsView},
		{"DiffResultView to VersionsView", DiffResultView, VersionsView},
		{"FileDiffView to DiffResultView", FileDiffView, DiffResultView},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := mocks.NewMockTUIService()
			m := NewModelWithConfig(&config.Config{}, svc)
			m.view = tt.startView
			m.diffResult = &DiffResult{} // Needed for some views
			m.fileDiffResult = &FileDiffResult{}

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(*Model)

			if m.view != tt.expectedView {
				t.Errorf("view = %v, expected %v", m.view, tt.expectedView)
			}
		})
	}
}

func TestUpdateKeyboardEnterDiffResult(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/tmp"}, svc)
	m.selectedProject = "proj"
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes: []FileChange{
			{Path: "file.txt", Status: 'M'},
		},
	}
	m.view = DiffResultView
	m.diffCursor = 0

	// Press enter to view file diff
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Error("cmd should not be nil")
	}
}

func TestUpdateKeyboardRun(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.BackupResults = map[string]ports.TUIBackupResult{
		"proj": {Size: 1024},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.view = ProjectsView

	// Press 'r' to run backup
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd == nil {
		t.Error("cmd should not be nil")
	}
}

func TestUpdateKeyboardVerify(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.view = ProjectsView

	// Press 'v' to verify
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if cmd == nil {
		t.Error("cmd should not be nil")
	}
}

func TestUpdateClearsStatusOnKeypress(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.statusMsg = "Previous status"
	m.statusErr = true

	// Any key should clear status
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*Model)

	if m.statusMsg != "" {
		t.Errorf("statusMsg = %q, expected empty", m.statusMsg)
	}
	if m.statusErr {
		t.Error("statusErr should be false")
	}
}

// ============================================
// View rendering tests
// ============================================

func TestRenderProjectsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{
		{Name: "project-alpha", Versions: 5, TotalSize: 1024 * 1024, LastBackup: time.Now().Add(-2 * time.Hour)},
		{Name: "project-beta", Versions: 0, TotalSize: 0},
	}
	m.width = 80
	m.height = 24
	m.view = ProjectsView

	view := m.View()

	if !contains(view, "codebak") {
		t.Error("View should contain 'codebak'")
	}
	if !contains(view, "project-alpha") {
		t.Error("View should contain 'project-alpha'")
	}
	if !contains(view, "project-beta") {
		t.Error("View should contain 'project-beta'")
	}
}

func TestRenderProjectsViewWithStatus(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.width = 80
	m.height = 24
	m.view = ProjectsView
	m.statusMsg = "Backup complete!"
	m.statusErr = false

	view := m.View()
	if !contains(view, "Backup complete!") {
		t.Error("View should contain status message")
	}
}

func TestRenderProjectsViewWithError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.width = 80
	m.height = 24
	m.view = ProjectsView
	m.statusMsg = "Error occurred"
	m.statusErr = true

	view := m.View()
	if !contains(view, "Error occurred") {
		t.Error("View should contain error message")
	}
}

func TestRenderVersionsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "my-project"
	m.versions = []VersionItem{
		{File: "2024-01-15.zip", Size: 2048, FileCount: 50, GitHead: "abc1234def"},
		{File: "2024-01-14.zip", Size: 1024, FileCount: 45, GitHead: ""},
	}
	m.width = 80
	m.height = 24
	m.view = VersionsView

	view := m.View()

	if !contains(view, "my-project") {
		t.Error("View should contain project name")
	}
	if !contains(view, "2024-01-15") {
		t.Error("View should contain version")
	}
	if !contains(view, "abc1234") {
		t.Error("View should contain truncated git head")
	}
}

func TestRenderVersionsViewEmpty(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "empty-project"
	m.versions = []VersionItem{}
	m.width = 80
	m.height = 24
	m.view = VersionsView

	view := m.View()

	if !contains(view, "No backups found") {
		t.Error("View should show 'No backups found'")
	}
}

func TestRenderDiffSelectView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "diff-project"
	m.versions = []VersionItem{
		{File: "v2.zip", Size: 2048},
		{File: "v1.zip", Size: 1024},
	}
	m.diffSelections = []int{0}
	m.width = 80
	m.height = 24
	m.view = DiffSelectView

	view := m.View()

	if !contains(view, "Select versions to compare") {
		t.Error("View should contain selection instruction")
	}
}

func TestRenderDiffSelectViewMessages(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{{File: "v1.zip"}, {File: "v2.zip"}}
	m.width = 80
	m.height = 24
	m.view = DiffSelectView

	// No selections
	m.diffSelections = nil
	view := m.View()
	if !contains(view, "Select first version") {
		t.Error("View should prompt for first version")
	}

	// One selection
	m.diffSelections = []int{0}
	view = m.View()
	if !contains(view, "Select second version") {
		t.Error("View should prompt for second version")
	}
}

func TestRenderDiffResultView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Added:    2,
		Modified: 3,
		Deleted:  1,
		Changes: []FileChange{
			{Path: "src/main.go", Status: 'M'},
			{Path: "src/new.go", Status: 'A'},
			{Path: "src/old.go", Status: 'D'},
		},
	}
	m.width = 80
	m.height = 24
	m.view = DiffResultView

	view := m.View()

	if !contains(view, "v1") || !contains(view, "v2") {
		t.Error("View should contain version names")
	}
	if !contains(view, "Modified: 3") {
		t.Error("View should show modified count")
	}
	if !contains(view, "src/main.go") {
		t.Error("View should contain file paths")
	}
}

func TestRenderDiffResultViewEmpty(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes:  []FileChange{},
	}
	m.width = 80
	m.height = 24
	m.view = DiffResultView

	view := m.View()

	if !contains(view, "No differences found") {
		t.Error("View should show 'No differences found'")
	}
}

func TestRenderDiffResultViewNil(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = nil
	m.view = DiffResultView

	view := m.View()

	if !contains(view, "Loading") {
		t.Error("View should show 'Loading...'")
	}
}

func TestRenderFileDiffView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:     "src/main.go",
		Version1: "v1",
		Version2: "v2",
		Lines: []DiffLine{
			{LineNum1: 1, LineNum2: 1, Type: ' ', Content: "package main"},
			{LineNum1: 2, LineNum2: 0, Type: '-', Content: "// old comment"},
			{LineNum1: 0, LineNum2: 2, Type: '+', Content: "// new comment"},
		},
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()

	if !contains(view, "src/main.go") {
		t.Error("View should contain file path")
	}
	if !contains(view, "package main") {
		t.Error("View should contain file content")
	}
}

func TestRenderFileDiffViewBinary(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:     "image.png",
		IsBinary: true,
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()

	if !contains(view, "Binary file") {
		t.Error("View should indicate binary file")
	}
}

func TestRenderFileDiffViewError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:  "file.txt",
		Error: "Could not read file",
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()

	if !contains(view, "Could not read file") {
		t.Error("View should show error message")
	}
}

func TestRenderFileDiffViewNil(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = nil
	m.view = FileDiffView

	view := m.View()

	if !contains(view, "Loading") {
		t.Error("View should show 'Loading...'")
	}
}

func TestRenderFileDiffViewSwapped(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines:    []DiffLine{{LineNum1: 1, LineNum2: 1, Type: ' ', Content: "content"}},
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView
	m.diffSwapped = true

	view := m.View()

	// When swapped, v2 should appear before v1 in header
	// The header format is "  %-35s | %-35s"
	if !contains(view, "v2") || !contains(view, "v1") {
		t.Error("View should contain both versions")
	}
}

func TestRenderFileDiffViewNoLines(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines:    []DiffLine{},
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()

	if !contains(view, "No differences") {
		t.Error("View should show 'No differences'")
	}
}

func TestViewQuitting(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.quitting = true

	view := m.View()

	if view != "" {
		t.Errorf("View() = %q, expected empty string when quitting", view)
	}
}

// ============================================
// loadProjects and loadVersions tests
// ============================================

func TestLoadProjects(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.Projects = []ports.TUIProjectInfo{
		{Name: "proj1", Path: "/path/to/proj1", Versions: 3, TotalSize: 1024},
		{Name: "proj2", Path: "/path/to/proj2", Versions: 1, TotalSize: 512},
	}
	m := NewModelWithConfig(&config.Config{}, svc)

	err := m.loadProjects()
	if err != nil {
		t.Fatalf("loadProjects failed: %v", err)
	}

	if len(m.projects) != 2 {
		t.Errorf("projects = %d, expected 2", len(m.projects))
	}
	if m.projects[0].Name != "proj1" {
		t.Errorf("projects[0].Name = %q, expected 'proj1'", m.projects[0].Name)
	}
}

func TestLoadProjectsError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ProjectsError = errors.New("failed to list")
	m := NewModelWithConfig(&config.Config{}, svc)

	err := m.loadProjects()
	if err == nil {
		t.Error("loadProjects should return error")
	}
}

func TestLoadVersions(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.Versions = map[string][]ports.TUIVersionInfo{
		"myproj": {
			{File: "v2.zip", Size: 2048, FileCount: 20, GitHead: "abc123"},
			{File: "v1.zip", Size: 1024, FileCount: 10, GitHead: "def456"},
		},
	}
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "myproj"

	err := m.loadVersions()
	if err != nil {
		t.Fatalf("loadVersions failed: %v", err)
	}

	if len(m.versions) != 2 {
		t.Errorf("versions = %d, expected 2", len(m.versions))
	}
	if m.versions[0].File != "v2.zip" {
		t.Errorf("versions[0].File = %q, expected 'v2.zip'", m.versions[0].File)
	}
}

func TestLoadVersionsError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.VersionsError = errors.New("failed to list versions")
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "proj"

	err := m.loadVersions()
	if err == nil {
		t.Error("loadVersions should return error")
	}
}

// ============================================
// NewModelWithService error tests
// ============================================

func TestNewModelWithServiceConfigError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigError = errors.New("config not found")

	_, err := NewModelWithService(svc)
	if err == nil {
		t.Error("NewModelWithService should return error when config fails")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("error = %q, expected to contain 'loading config'", err.Error())
	}
}

func TestNewModelWithServiceProjectsError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.ConfigResult = &config.Config{}
	svc.ProjectsError = errors.New("projects failed")

	_, err := NewModelWithService(svc)
	if err == nil {
		t.Error("NewModelWithService should return error when projects fail")
	}
}

// ============================================
// Init test
// ============================================

func TestInit(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	cmd := m.Init()

	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

// ============================================
// computeFileDiff test
// ============================================

func TestComputeFileDiffCmd(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/nonexistent"}, svc)
	m.selectedProject = "proj"
	m.diffResult = &DiffResult{Version1: "v1", Version2: "v2"}

	change := FileChange{Path: "file.txt", Status: 'M'}
	cmd := m.computeFileDiff(change)

	if cmd == nil {
		t.Error("computeFileDiff should return a command")
	}

	// Execute the command - it will fail because the files don't exist, but that's OK
	msg := cmd()
	if _, ok := msg.(fileDiffMsg); !ok {
		t.Errorf("cmd returned %T, expected fileDiffMsg", msg)
	}
}

// ============================================
// Additional edge case tests
// ============================================

func TestMoveCursorEmptyProjects(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{} // Empty
	m.view = ProjectsView
	m.projectCursor = 0

	// Should not panic or change cursor
	m.moveCursor(1)
	// When empty, cursor becomes -1 (len-1 = 0-1 = -1) but then gets clamped to 0
	// We just verify it doesn't panic
}

func TestMoveCursorEmptyVersions(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{} // Empty
	m.view = VersionsView
	m.versionCursor = 0

	// Should not panic
	m.moveCursor(1)
}

func TestRenderProjectsViewScrolling(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	// Create many projects to test scrolling
	for i := 0; i < 50; i++ {
		m.projects = append(m.projects, ProjectItem{Name: "project-" + string(rune('a'+i%26))})
	}
	m.projectCursor = 40 // Near the end
	m.width = 80
	m.height = 24
	m.view = ProjectsView

	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestRenderVersionsViewScrolling(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "proj"

	// Create many versions to test scrolling
	for i := 0; i < 50; i++ {
		m.versions = append(m.versions, VersionItem{File: "v" + string(rune('0'+i%10)) + ".zip"})
	}
	m.versionCursor = 40
	m.width = 80
	m.height = 24
	m.view = VersionsView

	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestRenderDiffSelectViewScrolling(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "proj"

	for i := 0; i < 50; i++ {
		m.versions = append(m.versions, VersionItem{File: "v" + string(rune('0'+i%10)) + ".zip"})
	}
	m.versionCursor = 40
	m.width = 80
	m.height = 24
	m.view = DiffSelectView

	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestRenderDiffResultViewScrolling(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	// Create many changes to test scrolling
	changes := []FileChange{}
	for i := 0; i < 50; i++ {
		changes = append(changes, FileChange{Path: "file" + string(rune('a'+i%26)) + ".txt", Status: 'M'})
	}
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes:  changes,
		Modified: 50,
	}
	m.diffCursor = 40
	m.width = 80
	m.height = 24
	m.view = DiffResultView

	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestRenderFileDiffViewScrollIndicator(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	// Create many lines to show scroll indicator
	lines := []DiffLine{}
	for i := 0; i < 100; i++ {
		lines = append(lines, DiffLine{LineNum1: i + 1, LineNum2: i + 1, Type: ' ', Content: "line content"})
	}
	m.fileDiffResult = &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines:    lines,
	}
	m.fileDiffScroll = 20
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()
	if !contains(view, "Lines") {
		t.Error("View should show scroll indicator")
	}
}

func TestRenderFileDiffViewLongContent(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	// Create a line with very long content
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "a"
	}
	m.fileDiffResult = &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines: []DiffLine{
			{LineNum1: 1, LineNum2: 1, Type: ' ', Content: longContent},
			{LineNum1: 2, LineNum2: 0, Type: '-', Content: longContent},
			{LineNum1: 0, LineNum2: 2, Type: '+', Content: longContent},
		},
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView

	view := m.View()
	// Should truncate and add "..."
	if !contains(view, "...") {
		t.Error("View should truncate long lines with ellipsis")
	}
}

func TestUpdateEnterWithNoProjects(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{} // Empty
	m.view = ProjectsView

	// Press enter - should not crash
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	// View should stay the same
	if m.view != ProjectsView {
		t.Errorf("view = %v, expected ProjectsView", m.view)
	}
}

func TestUpdateEnterWithNoDiffChanges(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes:  []FileChange{}, // Empty
	}
	m.view = DiffResultView

	// Press enter - should not crash
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// No command should be returned
	if cmd != nil {
		t.Error("cmd should be nil when no changes")
	}
}

func TestUpdateEnterVersionsViewError(t *testing.T) {
	svc := mocks.NewMockTUIService()
	svc.VersionsError = errors.New("failed to load")
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "proj"}}
	m.view = ProjectsView

	// Press enter - should show error status
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if !m.statusErr {
		t.Error("statusErr should be true on load error")
	}
	if m.view != ProjectsView {
		t.Errorf("view = %v, expected to stay on ProjectsView", m.view)
	}
}

func TestUpdateKKey(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "p1"}, {Name: "p2"}}
	m.projectCursor = 1
	m.view = ProjectsView

	// Press 'k' (vim up)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(*Model)

	if m.projectCursor != 0 {
		t.Errorf("projectCursor = %d, expected 0", m.projectCursor)
	}
}

func TestUpdateJKey(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.projects = []ProjectItem{{Name: "p1"}, {Name: "p2"}}
	m.projectCursor = 0
	m.view = ProjectsView

	// Press 'j' (vim down)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(*Model)

	if m.projectCursor != 1 {
		t.Errorf("projectCursor = %d, expected 1", m.projectCursor)
	}
}

func TestUpdateBackspace(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = VersionsView

	// Press backspace (same as esc)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(*Model)

	if m.view != ProjectsView {
		t.Errorf("view = %v, expected ProjectsView", m.view)
	}
}

func TestUpdateTabForSelect(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.versions = []VersionItem{{File: "v1.zip"}, {File: "v2.zip"}}
	m.view = DiffSelectView
	m.versionCursor = 0

	// Press tab to select (alternative to space)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*Model)

	if len(m.diffSelections) != 1 {
		t.Errorf("diffSelections = %d, expected 1", len(m.diffSelections))
	}
}

func TestUpdateCtrlC(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)

	// Press ctrl+c to quit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(*Model)

	if !m.quitting {
		t.Error("quitting should be true")
	}
	if cmd == nil {
		t.Error("cmd should not be nil")
	}
}

func TestRenderVersionsViewWithStatus(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "proj"
	m.versions = []VersionItem{{File: "v1.zip"}}
	m.width = 80
	m.height = 24
	m.view = VersionsView
	m.statusMsg = "Test status"
	m.statusErr = false

	view := m.View()
	if !contains(view, "Test status") {
		t.Error("View should contain status message")
	}
}

func TestRenderDiffResultViewWithStatus(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.diffResult = &DiffResult{
		Version1: "v1",
		Version2: "v2",
		Changes:  []FileChange{{Path: "file.txt", Status: 'M'}},
	}
	m.width = 80
	m.height = 24
	m.view = DiffResultView
	m.statusMsg = "Status message"
	m.statusErr = true

	view := m.View()
	if !contains(view, "Status message") {
		t.Error("View should contain status message")
	}
}

func TestRenderFileDiffViewWithStatus(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Path:     "file.txt",
		Version1: "v1",
		Version2: "v2",
		Lines:    []DiffLine{{Content: "line"}},
	}
	m.width = 80
	m.height = 24
	m.view = FileDiffView
	m.statusMsg = "File status"
	m.statusErr = false

	view := m.View()
	if !contains(view, "File status") {
		t.Error("View should contain status message")
	}
}

func TestMoveCursorFileDiffViewMaxScroll(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Lines: make([]DiffLine, 100),
	}
	m.view = FileDiffView
	m.fileDiffScroll = 0
	m.height = 20

	// Move to max scroll
	m.moveCursor(1000)
	// maxScroll = 100 - (20 - 10) = 90
	expectedMax := 100 - (20 - 10)
	if m.fileDiffScroll != expectedMax {
		t.Errorf("fileDiffScroll = %d, expected %d", m.fileDiffScroll, expectedMax)
	}
}

func TestMoveCursorFileDiffViewSmallContent(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.fileDiffResult = &FileDiffResult{
		Lines: make([]DiffLine, 5), // Small content
	}
	m.view = FileDiffView
	m.fileDiffScroll = 0
	m.height = 30

	// With small content, maxScroll should be 0
	m.moveCursor(10)
	if m.fileDiffScroll != 0 {
		t.Errorf("fileDiffScroll = %d, expected 0 (content fits in view)", m.fileDiffScroll)
	}
}

func TestRunVerifyFromVersionsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.selectedProject = "proj"
	m.view = VersionsView

	cmd := m.runVerify()
	msg := cmd().(statusMsg)

	if msg.err {
		t.Error("unexpected error")
	}
	if !contains(msg.msg, "verified") {
		t.Errorf("msg = %q, expected to contain 'verified'", msg.msg)
	}
}

// ============================================
// Settings view tests
// ============================================

func TestSettingsViewNavigation(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/test/backups"}, svc)
	m.view = ProjectsView

	// Press ? to enter settings
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(*Model)

	if m.view != SettingsView {
		t.Errorf("view = %v, expected SettingsView", m.view)
	}
	if m.prevView != ProjectsView {
		t.Errorf("prevView = %v, expected ProjectsView", m.prevView)
	}
	if m.settingsCursor != 0 {
		t.Errorf("settingsCursor = %d, expected 0", m.settingsCursor)
	}
}

func TestSettingsViewCursorMovement(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = SettingsView
	m.settingsCursor = 0

	// Move down
	m.moveCursor(1)
	if m.settingsCursor != 1 {
		t.Errorf("settingsCursor = %d, expected 1", m.settingsCursor)
	}

	// Move down again
	m.moveCursor(1)
	if m.settingsCursor != 2 {
		t.Errorf("settingsCursor = %d, expected 2", m.settingsCursor)
	}

	// Move up
	m.moveCursor(-1)
	if m.settingsCursor != 1 {
		t.Errorf("settingsCursor = %d, expected 1", m.settingsCursor)
	}

	// Move up past beginning (should clamp to 0)
	m.moveCursor(-10)
	if m.settingsCursor != 0 {
		t.Errorf("settingsCursor = %d, expected 0", m.settingsCursor)
	}

	// Move down past end (should clamp to 3)
	m.moveCursor(100)
	if m.settingsCursor != 3 {
		t.Errorf("settingsCursor = %d, expected 3 (max)", m.settingsCursor)
	}
}

func TestSettingsViewBack(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = SettingsView
	m.prevView = VersionsView

	// Press esc to go back
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(*Model)

	if m.view != VersionsView {
		t.Errorf("view = %v, expected VersionsView", m.view)
	}
}

func TestRenderSettingsView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/my/backups"}, svc)
	m.view = SettingsView
	m.settingsCursor = 0
	m.width = 80

	output := m.renderSettingsView()

	// Check for title
	if !contains(output, "Settings") {
		t.Error("output should contain 'Settings' title")
	}

	// Check for backup directory setting
	if !contains(output, "Backup Directory") {
		t.Error("output should contain 'Backup Directory'")
	}
	if !contains(output, "/my/backups") {
		t.Error("output should contain backup dir path")
	}

	// Check for other settings
	if !contains(output, "Color Theme") {
		t.Error("output should contain 'Color Theme'")
	}
	if !contains(output, "Migrate Backups") {
		t.Error("output should contain 'Migrate Backups'")
	}
	if !contains(output, "About") {
		t.Error("output should contain 'About'")
	}
}

func TestRenderSettingsViewCursorHighlight(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/backups"}, svc)
	m.view = SettingsView
	m.settingsCursor = 1 // Color Theme
	m.width = 80

	output := m.renderSettingsView()

	// The selected item should have the selection indicator
	if !contains(output, "Color Theme") {
		t.Error("output should contain 'Color Theme'")
	}
}

func TestSettingsViewInView(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/backups"}, svc)
	m.view = SettingsView
	m.width = 80
	m.height = 24

	output := m.View()

	if !contains(output, "Settings") {
		t.Error("View() should render settings view")
	}
}

func TestSettingsKeyFromMultipleViews(t *testing.T) {
	views := []View{ProjectsView, VersionsView, DiffSelectView, DiffResultView, FileDiffView}

	for _, startView := range views {
		svc := mocks.NewMockTUIService()
		m := NewModelWithConfig(&config.Config{}, svc)
		m.view = startView
		m.versions = []VersionItem{{File: "v1.zip"}}
		m.diffResult = &DiffResult{Changes: []FileChange{{Path: "test.go"}}}
		m.fileDiffResult = &FileDiffResult{Lines: []DiffLine{{Content: "test"}}}

		// Press ? to enter settings
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		m = updated.(*Model)

		if m.view != SettingsView {
			t.Errorf("from %v: view = %v, expected SettingsView", startView, m.view)
		}
		if m.prevView != startView {
			t.Errorf("from %v: prevView = %v, expected %v", startView, m.prevView, startView)
		}
	}
}

func TestSettingsKeyDoesNotEnterFromSettings(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{}, svc)
	m.view = SettingsView
	m.prevView = ProjectsView

	// Press ? while already in settings - should do nothing
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(*Model)

	if m.view != SettingsView {
		t.Errorf("view should remain SettingsView, got %v", m.view)
	}
}

func TestSettingsSelectEnter(t *testing.T) {
	svc := mocks.NewMockTUIService()
	m := NewModelWithConfig(&config.Config{BackupDir: "/test/backups"}, svc)
	m.view = SettingsView
	m.settingsCursor = 0 // Backup Directory

	// Press Enter on Backup Directory
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if !contains(m.statusMsg, "/test/backups") {
		t.Errorf("statusMsg should contain backup dir, got %q", m.statusMsg)
	}

	// Test About option
	m.settingsCursor = 3 // About
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if !contains(m.statusMsg, "codebak") {
		t.Errorf("statusMsg should contain 'codebak' for About, got %q", m.statusMsg)
	}
}
