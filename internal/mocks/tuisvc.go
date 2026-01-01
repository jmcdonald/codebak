package mocks

import (
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/ports"
)

// MockTUIService implements ports.TUIService for testing.
type MockTUIService struct {
	// ConfigResult is the config to return from LoadConfig
	ConfigResult *config.Config
	// ConfigError is the error to return from LoadConfig
	ConfigError error

	// Projects is the list of projects to return
	Projects []ports.TUIProjectInfo
	// ProjectsError is the error to return from ListProjects
	ProjectsError error

	// Versions maps project names to their versions
	Versions map[string][]ports.TUIVersionInfo
	// VersionsError is the error to return from ListVersions
	VersionsError error

	// BackupResults maps project names to backup results
	BackupResults map[string]ports.TUIBackupResult

	// VerifyErrors maps project names to verify errors
	VerifyErrors map[string]error

	// Call tracking
	LoadConfigCalls     int
	ListProjectsCalls   int
	ListVersionsCalls   []string
	RunBackupCalls      []string
	VerifyBackupCalls   []string
}

// NewMockTUIService creates a new mock TUI service.
func NewMockTUIService() *MockTUIService {
	return &MockTUIService{
		ConfigResult:  &config.Config{},
		Versions:      make(map[string][]ports.TUIVersionInfo),
		BackupResults: make(map[string]ports.TUIBackupResult),
		VerifyErrors:  make(map[string]error),
	}
}

// LoadConfig loads the application configuration.
func (m *MockTUIService) LoadConfig() (*config.Config, error) {
	m.LoadConfigCalls++
	if m.ConfigError != nil {
		return nil, m.ConfigError
	}
	return m.ConfigResult, nil
}

// ListProjects returns all projects with their metadata.
func (m *MockTUIService) ListProjects(cfg *config.Config) ([]ports.TUIProjectInfo, error) {
	m.ListProjectsCalls++
	if m.ProjectsError != nil {
		return nil, m.ProjectsError
	}
	return m.Projects, nil
}

// ListVersions returns all backup versions for a project.
func (m *MockTUIService) ListVersions(cfg *config.Config, project string) ([]ports.TUIVersionInfo, error) {
	m.ListVersionsCalls = append(m.ListVersionsCalls, project)
	if m.VersionsError != nil {
		return nil, m.VersionsError
	}
	if versions, ok := m.Versions[project]; ok {
		return versions, nil
	}
	return nil, nil
}

// RunBackup performs a backup of the specified project.
func (m *MockTUIService) RunBackup(cfg *config.Config, project string) ports.TUIBackupResult {
	m.RunBackupCalls = append(m.RunBackupCalls, project)
	if result, ok := m.BackupResults[project]; ok {
		return result
	}
	return ports.TUIBackupResult{Size: 1024}
}

// VerifyBackup verifies the latest backup of a project.
func (m *MockTUIService) VerifyBackup(cfg *config.Config, project string) error {
	m.VerifyBackupCalls = append(m.VerifyBackupCalls, project)
	if err, ok := m.VerifyErrors[project]; ok {
		return err
	}
	return nil
}

// Compile-time check that MockTUIService implements ports.TUIService.
var _ ports.TUIService = (*MockTUIService)(nil)
