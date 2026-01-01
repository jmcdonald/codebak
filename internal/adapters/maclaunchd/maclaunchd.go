// Package maclaunchd provides a launchd service adapter for macOS.
package maclaunchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/mcdonaldj/codebak/internal/ports"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.codebak</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>run</string>
    </array>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>{{.Hour}}</integer>
        <key>Minute</key>
        <integer>{{.Minute}}</integer>
    </dict>
    <key>RunAtLoad</key>
    <false/>
    <key>StandardOutPath</key>
    <string>{{.LogPath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogPath}}</string>
</dict>
</plist>
`

const serviceLabel = "com.user.codebak"

type plistConfig struct {
	BinaryPath string
	Hour       int
	Minute     int
	LogPath    string
}

// MacLaunchdService implements ports.LaunchdService for macOS.
type MacLaunchdService struct {
	homeDir string
}

// New creates a new MacLaunchdService adapter.
func New() *MacLaunchdService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &MacLaunchdService{homeDir: home}
}

// PlistPath returns the path where the plist file should be stored.
func (s *MacLaunchdService) PlistPath() string {
	return filepath.Join(s.homeDir, "Library", "LaunchAgents", "com.user.codebak.plist")
}

// LogPath returns the path where logs should be written.
func (s *MacLaunchdService) LogPath() string {
	return filepath.Join(s.homeDir, ".codebak", "codebak.log")
}

// Install creates the plist file and loads the service.
func (s *MacLaunchdService) Install(execPath, configPath string, hour, minute int) error {
	// Find codebak binary if not provided
	binaryPath := execPath
	if binaryPath == "" {
		var err error
		binaryPath, err = exec.LookPath("codebak")
		if err != nil {
			return fmt.Errorf("codebak not found in PATH: %w", err)
		}
	}

	// Ensure log directory exists
	logPath := s.LogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Generate plist
	config := plistConfig{
		BinaryPath: binaryPath,
		Hour:       hour,
		Minute:     minute,
		LogPath:    logPath,
	}

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	// Ensure LaunchAgents directory exists
	plistPath := s.PlistPath()
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	// Write plist file
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("creating plist: %w", err)
	}

	if err := tmpl.Execute(f, config); err != nil {
		_ = f.Close() // Best effort cleanup on error
		return fmt.Errorf("writing plist: %w", err)
	}

	// Close file BEFORE loading to ensure data is flushed
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing plist file: %w", err)
	}

	// Load the plist
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("loading plist: %w", err)
	}

	return nil
}

// Uninstall unloads the service and removes the plist file.
func (s *MacLaunchdService) Uninstall() error {
	plistPath := s.PlistPath()

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("plist not found: %s", plistPath)
	}

	// Unload the plist
	cmd := exec.Command("launchctl", "unload", plistPath)
	_ = cmd.Run() // Ignore error if not loaded

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

// IsInstalled checks if the service is currently installed.
func (s *MacLaunchdService) IsInstalled() bool {
	_, err := os.Stat(s.PlistPath())
	return err == nil
}

// Status returns the current status of the service.
// Returns "running", "loaded", "not loaded", or error message.
func (s *MacLaunchdService) Status() string {
	if !s.IsInstalled() {
		return "not installed"
	}

	cmd := exec.Command("launchctl", "list", serviceLabel)
	err := cmd.Run()
	if err == nil {
		return "loaded"
	}
	return "not loaded"
}

// Compile-time check that MacLaunchdService implements ports.LaunchdService.
var _ ports.LaunchdService = (*MacLaunchdService)(nil)
