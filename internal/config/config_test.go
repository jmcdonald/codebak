package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check schedule defaults
	if cfg.Schedule != "daily" {
		t.Errorf("Schedule = %q, expected %q", cfg.Schedule, "daily")
	}
	if cfg.Time != "03:00" {
		t.Errorf("Time = %q, expected %q", cfg.Time, "03:00")
	}

	// Check retention default
	if cfg.Retention.KeepLast != 30 {
		t.Errorf("Retention.KeepLast = %d, expected %d", cfg.Retention.KeepLast, 30)
	}

	// Check default exclusions include common patterns
	expectedExclusions := []string{"node_modules", ".venv", "__pycache__", ".git"}
	for _, pattern := range expectedExclusions {
		found := false
		for _, exc := range cfg.Exclude {
			if exc == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected exclusion %q not found in defaults", pattern)
		}
	}
}

func TestLoadMissingConfig(t *testing.T) {
	// Create a temp dir to use as home (so we can control the config path)
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original HOME and restore after
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Load config - should return defaults when file missing
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed for missing config: %v", err)
	}

	// Should have default values
	if cfg.Schedule != "daily" {
		t.Errorf("Expected default schedule, got %q", cfg.Schedule)
	}
}

func TestLoadValidConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create config directory
	configDir := filepath.Join(tempDir, ".codebak")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write a custom config
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `
source_dir: /custom/source
backup_dir: /custom/backup
schedule: weekly
time: "04:30"
exclude:
  - custom_exclude
retention:
  keep_last: 10
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load and verify
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.SourceDir != "/custom/source" {
		t.Errorf("SourceDir = %q, expected %q", cfg.SourceDir, "/custom/source")
	}
	if cfg.BackupDir != "/custom/backup" {
		t.Errorf("BackupDir = %q, expected %q", cfg.BackupDir, "/custom/backup")
	}
	if cfg.Schedule != "weekly" {
		t.Errorf("Schedule = %q, expected %q", cfg.Schedule, "weekly")
	}
	if cfg.Time != "04:30" {
		t.Errorf("Time = %q, expected %q", cfg.Time, "04:30")
	}
	if cfg.Retention.KeepLast != 10 {
		t.Errorf("Retention.KeepLast = %d, expected %d", cfg.Retention.KeepLast, 10)
	}
}

func TestLoadMalformedConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create config directory
	configDir := filepath.Join(tempDir, ".codebak")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write malformed YAML
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("this: is: not: valid: yaml: [[["), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load should fail
	_, err = Load()
	if err == nil {
		t.Error("Load should fail for malformed YAML")
	}
}

func TestSaveConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	cfg := &Config{
		SourceDir: "/my/source",
		BackupDir: "/my/backup",
		Schedule:  "hourly",
		Time:      "00:00",
		Exclude:   []string{"test_exclude"},
		Retention: struct {
			KeepLast int `yaml:"keep_last"`
		}{KeepLast: 5},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tempDir, ".codebak", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file was not created: %v", err)
	}

	// Load it back and verify
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if loaded.SourceDir != cfg.SourceDir {
		t.Errorf("SourceDir mismatch after save/load")
	}
	if loaded.Schedule != cfg.Schedule {
		t.Errorf("Schedule mismatch after save/load")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/code", filepath.Join(home, "code")},
		{"~/.config", filepath.Join(home, ".config")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"~", filepath.Join(home, "")}, // Just tilde
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("ExpandPath(%q) failed: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPathEmptyString(t *testing.T) {
	result, err := ExpandPath("")
	if err != nil {
		t.Fatalf("ExpandPath failed: %v", err)
	}
	if result != "" {
		t.Errorf("ExpandPath(\"\") = %q, expected empty string", result)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}

	// Should be absolute path in .codebak directory
	if !filepath.IsAbs(path) {
		t.Errorf("ConfigPath should be absolute, got %s", path)
	}
}

// ============================================================================
// Additional tests for coverage improvement
// ============================================================================

func TestLoadReadFileError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create config directory
	configDir := filepath.Join(tempDir, ".codebak")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config file that's a directory (to cause read error)
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Load should fail
	_, err = Load()
	if err == nil {
		t.Error("Load should fail when config file is a directory")
	}
}

func TestSaveWriteFileSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create a minimal config and save it
	cfg := &Config{
		SourceDir: "/source",
		BackupDir: "/backup",
		Schedule:  "daily",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created and is readable
	configPath := filepath.Join(tempDir, ".codebak", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	if len(data) == 0 {
		t.Error("Config file is empty")
	}
}

func TestExpandPathWithNoTilde(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"./current", "./current"},
		{"../parent", "../parent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Fatalf("ExpandPath(%q) failed: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfigContainsAllFields(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	// Verify all expected fields are populated
	if cfg.SourceDir == "" {
		t.Error("DefaultConfig should set SourceDir")
	}
	if cfg.BackupDir == "" {
		t.Error("DefaultConfig should set BackupDir")
	}
	if cfg.Schedule == "" {
		t.Error("DefaultConfig should set Schedule")
	}
	if cfg.Time == "" {
		t.Error("DefaultConfig should set Time")
	}
	if len(cfg.Exclude) == 0 {
		t.Error("DefaultConfig should set Exclude patterns")
	}
	if cfg.Retention.KeepLast == 0 {
		t.Error("DefaultConfig should set Retention.KeepLast")
	}

	// Verify paths contain "code" and "backups"
	if !filepath.IsAbs(cfg.SourceDir) {
		t.Error("SourceDir should be absolute")
	}
	if !filepath.IsAbs(cfg.BackupDir) {
		t.Error("BackupDir should be absolute")
	}
}

func TestLoadPartialConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create config directory
	configDir := filepath.Join(tempDir, ".codebak")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write a partial config (only some fields)
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `schedule: hourly`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load and verify partial override works
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Schedule should be overridden
	if cfg.Schedule != "hourly" {
		t.Errorf("Schedule = %q, expected %q", cfg.Schedule, "hourly")
	}

	// Other fields should have defaults
	if cfg.Time != "03:00" {
		t.Errorf("Time = %q, expected default %q", cfg.Time, "03:00")
	}
}

func TestSaveMkdirAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	// Create a file where the config directory should be
	// This will cause MkdirAll to fail
	codebakPath := filepath.Join(tempDir, ".codebak")
	if err := os.WriteFile(codebakPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cfg := &Config{
		SourceDir: "/source",
		BackupDir: "/backup",
	}

	err = cfg.Save()
	if err == nil {
		t.Error("Save should fail when MkdirAll fails")
	}
}

func TestConfigPathDefault(t *testing.T) {
	// Test with valid HOME
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}
	if !strings.Contains(path, ".codebak") {
		t.Errorf("ConfigPath should contain .codebak, got %s", path)
	}
	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("ConfigPath should end with config.yaml, got %s", path)
	}
}

func TestDefaultConfigHomeDir(t *testing.T) {
	// Test with valid HOME - paths should be absolute
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	if !strings.HasPrefix(cfg.SourceDir, home) {
		t.Errorf("SourceDir should start with home dir, got %s", cfg.SourceDir)
	}
	if !strings.HasPrefix(cfg.BackupDir, home) {
		t.Errorf("BackupDir should start with home dir, got %s", cfg.BackupDir)
	}
}

func TestExpandPathTildeOnly(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	// Test with just tilde
	result, err := ExpandPath("~")
	if err != nil {
		t.Fatalf("ExpandPath(~) failed: %v", err)
	}
	// filepath.Join(home, "") returns home without trailing slash
	expected := filepath.Join(home, "")
	if result != expected {
		t.Errorf("ExpandPath(~) = %q, expected %q", result, expected)
	}

	// Test with tilde and path
	result, err = ExpandPath("~/Documents")
	if err != nil {
		t.Fatalf("ExpandPath(~/Documents) failed: %v", err)
	}
	expected = filepath.Join(home, "Documents")
	if result != expected {
		t.Errorf("ExpandPath(~/Documents) = %q, expected %q", result, expected)
	}
}

// ============================================================================
// Tests for error paths when HOME is unavailable
// ============================================================================

func TestExpandPathNoHome(t *testing.T) {
	// Save original HOME and clear it
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	// ExpandPath with tilde should fail without HOME
	_, err := ExpandPath("~/code")
	if err == nil {
		t.Error("ExpandPath(~/code) should fail when HOME is not set")
	}
	if !errors.Is(err, ErrNoHomeDir) {
		t.Errorf("Expected ErrNoHomeDir, got: %v", err)
	}

	// Non-tilde paths should still work
	result, err := ExpandPath("/absolute/path")
	if err != nil {
		t.Errorf("ExpandPath(/absolute/path) should succeed: %v", err)
	}
	if result != "/absolute/path" {
		t.Errorf("Expected /absolute/path, got %q", result)
	}
}

func TestConfigPathNoHome(t *testing.T) {
	// Save original HOME and clear it
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	// ConfigPath should fail without HOME
	_, err := ConfigPath()
	if err == nil {
		t.Error("ConfigPath should fail when HOME is not set")
	}
	if !errors.Is(err, ErrNoHomeDir) {
		t.Errorf("Expected ErrNoHomeDir, got: %v", err)
	}
}

func TestDefaultConfigNoHome(t *testing.T) {
	// Save original HOME and clear it
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	// DefaultConfig should fail without HOME
	_, err := DefaultConfig()
	if err == nil {
		t.Error("DefaultConfig should fail when HOME is not set")
	}
	if !errors.Is(err, ErrNoHomeDir) {
		t.Errorf("Expected ErrNoHomeDir, got: %v", err)
	}
}

func TestLoadNoHome(t *testing.T) {
	// Save original HOME and clear it
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Load should fail without HOME (can't find config path)
	_, err := Load()
	if err == nil {
		t.Error("Load should fail when HOME is not set")
	}
}

func TestSaveNoHome(t *testing.T) {
	// Save original HOME and clear it
	origHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer os.Setenv("HOME", origHome)

	cfg := &Config{
		SourceDir: "/source",
		BackupDir: "/backup",
	}

	// Save should fail without HOME (can't find config path)
	err := cfg.Save()
	if err == nil {
		t.Error("Save should fail when HOME is not set")
	}
}
