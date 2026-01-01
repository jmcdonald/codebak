package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/mocks"
	"github.com/mcdonaldj/codebak/internal/ports"
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
