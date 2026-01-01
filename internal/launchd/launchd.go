package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
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

type PlistConfig struct {
	BinaryPath string
	Hour       int
	Minute     int
	LogPath    string
}

func PlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // Fallback - will fail gracefully when accessing Library/LaunchAgents
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com.user.codebak.plist")
}

func LogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // Fallback to current directory
	}
	return filepath.Join(home, ".codebak", "codebak.log")
}

func Install(hour, minute int) error {
	// Find codebak binary
	binaryPath, err := exec.LookPath("codebak")
	if err != nil {
		return fmt.Errorf("codebak not found in PATH: %w", err)
	}

	// Ensure log directory exists
	logPath := LogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	// Generate plist
	config := PlistConfig{
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
	plistPath := PlistPath()
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	// Write plist file
	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("creating plist: %w", err)
	}

	if err := tmpl.Execute(f, config); err != nil {
		f.Close()
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

func Uninstall() error {
	plistPath := PlistPath()

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

func IsInstalled() bool {
	_, err := os.Stat(PlistPath())
	return err == nil
}

func Status() (bool, error) {
	if !IsInstalled() {
		return false, nil
	}

	cmd := exec.Command("launchctl", "list", "com.user.codebak")
	err := cmd.Run()
	return err == nil, nil
}
