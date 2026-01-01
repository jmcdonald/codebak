package mocks

import (
	"github.com/mcdonaldj/codebak/internal/ports"
)

// MockLaunchdService implements ports.LaunchdService for testing.
type MockLaunchdService struct {
	// Installed tracks whether the service is "installed"
	Installed bool
	// StatusResult is the status to return
	StatusResult string
	// PlistPathResult is the plist path to return
	PlistPathResult string
	// LogPathResult is the log path to return
	LogPathResult string
	// InstallCalls records calls to Install
	InstallCalls []InstallCall
	// Errors maps method names to errors
	Errors map[string]error
}

// InstallCall records parameters of an Install call.
type InstallCall struct {
	ExecPath   string
	ConfigPath string
	Hour       int
	Minute     int
}

// NewMockLaunchdService creates a new mock launchd service.
func NewMockLaunchdService() *MockLaunchdService {
	return &MockLaunchdService{
		StatusResult:    "not installed",
		PlistPathResult: "/tmp/mock.plist",
		LogPathResult:   "/tmp/mock.log",
		Errors:          make(map[string]error),
	}
}

// PlistPath returns the path where the plist file should be stored.
func (m *MockLaunchdService) PlistPath() string {
	return m.PlistPathResult
}

// LogPath returns the path where logs should be written.
func (m *MockLaunchdService) LogPath() string {
	return m.LogPathResult
}

// Install creates the plist file and loads the service.
func (m *MockLaunchdService) Install(execPath, configPath string, hour, minute int) error {
	m.InstallCalls = append(m.InstallCalls, InstallCall{
		ExecPath:   execPath,
		ConfigPath: configPath,
		Hour:       hour,
		Minute:     minute,
	})
	if err, ok := m.Errors["Install"]; ok {
		return err
	}
	m.Installed = true
	m.StatusResult = "loaded"
	return nil
}

// Uninstall unloads the service and removes the plist file.
func (m *MockLaunchdService) Uninstall() error {
	if err, ok := m.Errors["Uninstall"]; ok {
		return err
	}
	m.Installed = false
	m.StatusResult = "not installed"
	return nil
}

// IsInstalled checks if the service is currently installed.
func (m *MockLaunchdService) IsInstalled() bool {
	return m.Installed
}

// Status returns the current status of the service.
func (m *MockLaunchdService) Status() string {
	return m.StatusResult
}

// Compile-time check that MockLaunchdService implements ports.LaunchdService.
var _ ports.LaunchdService = (*MockLaunchdService)(nil)
