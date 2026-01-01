package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

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
			result := ExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPathEmptyString(t *testing.T) {
	result := ExpandPath("")
	if result != "" {
		t.Errorf("ExpandPath(\"\") = %q, expected empty string", result)
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}

	// Should end with expected path
	if !filepath.IsAbs(path) {
		// In fallback mode (home unavailable), will be relative
		if filepath.Base(filepath.Dir(path)) != ".codebak" {
			t.Errorf("ConfigPath should be in .codebak directory, got %s", path)
		}
	}
}
