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

func TestDefaultBackupDirIsCodebak(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	expectedBackupDir := filepath.Join(home, ".codebak", "backups")
	if cfg.BackupDir != expectedBackupDir {
		t.Errorf("BackupDir = %q, expected %q", cfg.BackupDir, expectedBackupDir)
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

// ============================================================================
// Tests for SourceType and sensitive paths
// ============================================================================

func TestSourceTypeConstants(t *testing.T) {
	// Verify type constants exist and have expected values
	if SourceTypeGit != "git" {
		t.Errorf("SourceTypeGit = %q, expected %q", SourceTypeGit, "git")
	}
	if SourceTypeSensitive != "sensitive" {
		t.Errorf("SourceTypeSensitive = %q, expected %q", SourceTypeSensitive, "sensitive")
	}
}

func TestIsValidSourceType(t *testing.T) {
	tests := []struct {
		input    SourceType
		expected bool
	}{
		{SourceTypeGit, true},
		{SourceTypeSensitive, true},
		{"invalid", false},
		{"", false},
		{"GIT", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := IsValidSourceType(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidSourceType(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultSensitivePaths(t *testing.T) {
	paths := DefaultSensitivePaths()

	// Should have at least some paths
	if len(paths) == 0 {
		t.Error("DefaultSensitivePaths returned empty slice")
	}

	// Check for key expected paths
	expectedPaths := []string{"~/.ssh", "~/.aws", "~/.config"}
	for _, expected := range expectedPaths {
		found := false
		for _, p := range paths {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected path %q not found in DefaultSensitivePaths", expected)
		}
	}

	// All paths should start with ~/
	for _, p := range paths {
		if !strings.HasPrefix(p, "~/") {
			t.Errorf("Path %q does not start with ~/", p)
		}
	}
}

func TestGetSourcesAppliesDefaults(t *testing.T) {
	cfg := &Config{
		Sources: []Source{
			{Path: "/code/project1"},                          // No type, no icon
			{Path: "/code/project2", Type: SourceTypeGit},     // Has type, no icon
			{Path: "~/.ssh", Type: SourceTypeSensitive},       // Sensitive, no icon
			{Path: "~/.aws", Type: SourceTypeSensitive, Icon: "🔑"}, // Sensitive with custom icon
		},
	}

	sources := cfg.GetSources()

	// First source should default to git with bullet icon
	if sources[0].Type != SourceTypeGit {
		t.Errorf("sources[0].Type = %q, expected %q", sources[0].Type, SourceTypeGit)
	}
	if sources[0].Icon != "●" {
		t.Errorf("sources[0].Icon = %q, expected %q", sources[0].Icon, "●")
	}

	// Second source should keep git with bullet icon
	if sources[1].Type != SourceTypeGit {
		t.Errorf("sources[1].Type = %q, expected %q", sources[1].Type, SourceTypeGit)
	}
	if sources[1].Icon != "●" {
		t.Errorf("sources[1].Icon = %q, expected %q", sources[1].Icon, "●")
	}

	// Third source should have diamond icon for sensitive
	if sources[2].Type != SourceTypeSensitive {
		t.Errorf("sources[2].Type = %q, expected %q", sources[2].Type, SourceTypeSensitive)
	}
	if sources[2].Icon != "◆" {
		t.Errorf("sources[2].Icon = %q, expected %q", sources[2].Icon, "◆")
	}

	// Fourth source should keep custom icon
	if sources[3].Icon != "🔑" {
		t.Errorf("sources[3].Icon = %q, expected %q", sources[3].Icon, "🔑")
	}
}

func TestGetSourcesByType(t *testing.T) {
	cfg := &Config{
		Sources: []Source{
			{Path: "/code/project1", Type: SourceTypeGit},
			{Path: "/code/project2", Type: SourceTypeGit},
			{Path: "~/.ssh", Type: SourceTypeSensitive},
			{Path: "~/.aws", Type: SourceTypeSensitive},
			{Path: "~/.config", Type: SourceTypeSensitive},
		},
	}

	gitSources := cfg.GetSourcesByType(SourceTypeGit)
	if len(gitSources) != 2 {
		t.Errorf("Expected 2 git sources, got %d", len(gitSources))
	}

	sensitiveSources := cfg.GetSourcesByType(SourceTypeSensitive)
	if len(sensitiveSources) != 3 {
		t.Errorf("Expected 3 sensitive sources, got %d", len(sensitiveSources))
	}

	// Empty type should return none
	noSources := cfg.GetSourcesByType("invalid")
	if len(noSources) != 0 {
		t.Errorf("Expected 0 invalid sources, got %d", len(noSources))
	}
}

func TestGetSourcesBackwardsCompatibility(t *testing.T) {
	// Old-style config with just SourceDir
	cfg := &Config{
		SourceDir: "/old/source/dir",
	}

	sources := cfg.GetSources()

	if len(sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(sources))
	}

	if sources[0].Path != "/old/source/dir" {
		t.Errorf("sources[0].Path = %q, expected %q", sources[0].Path, "/old/source/dir")
	}

	// Migrated source should be git type
	if sources[0].Type != SourceTypeGit {
		t.Errorf("sources[0].Type = %q, expected %q (migration default)", sources[0].Type, SourceTypeGit)
	}
}

func TestLoadConfigWithMixedTypes(t *testing.T) {
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

	// Write config with mixed source types
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `
sources:
  - path: ~/code
    type: git
    label: Code
  - path: ~/.ssh
    type: sensitive
    label: SSH Keys
  - path: ~/.aws
    type: sensitive
    label: AWS Config
backup_dir: /backup
schedule: daily
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load and verify
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Sources) != 3 {
		t.Fatalf("Expected 3 sources, got %d", len(cfg.Sources))
	}

	// Check types were parsed correctly
	if cfg.Sources[0].Type != SourceTypeGit {
		t.Errorf("Sources[0].Type = %q, expected %q", cfg.Sources[0].Type, SourceTypeGit)
	}
	if cfg.Sources[1].Type != SourceTypeSensitive {
		t.Errorf("Sources[1].Type = %q, expected %q", cfg.Sources[1].Type, SourceTypeSensitive)
	}
	if cfg.Sources[2].Type != SourceTypeSensitive {
		t.Errorf("Sources[2].Type = %q, expected %q", cfg.Sources[2].Type, SourceTypeSensitive)
	}

	// Check labels
	if cfg.Sources[1].Label != "SSH Keys" {
		t.Errorf("Sources[1].Label = %q, expected %q", cfg.Sources[1].Label, "SSH Keys")
	}
}

func TestApplySourceDefaultsDoesNotModifyOriginal(t *testing.T) {
	cfg := &Config{
		Sources: []Source{
			{Path: "/code/project1"}, // No type
		},
	}

	// Get sources (which applies defaults)
	sources := cfg.GetSources()

	// Original should be unchanged
	if cfg.Sources[0].Type != "" {
		t.Error("Original source type was modified")
	}

	// Returned source should have defaults
	if sources[0].Type != SourceTypeGit {
		t.Errorf("Returned source should have default type")
	}
}

// ============================================================================
// Tests for restic configuration
// ============================================================================

func TestDefaultResticRepoPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	path, err := DefaultResticRepoPath()
	if err != nil {
		t.Fatalf("DefaultResticRepoPath failed: %v", err)
	}

	expected := filepath.Join(home, ".codebak", "restic-repo")
	if path != expected {
		t.Errorf("DefaultResticRepoPath = %q, expected %q", path, expected)
	}
}

func TestGetResticRepoPathDefault(t *testing.T) {
	cfg := &Config{}

	path, err := cfg.GetResticRepoPath()
	if err != nil {
		t.Fatalf("GetResticRepoPath failed: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	expected := filepath.Join(home, ".codebak", "restic-repo")
	if path != expected {
		t.Errorf("GetResticRepoPath = %q, expected %q", path, expected)
	}
}

func TestGetResticRepoPathCustom(t *testing.T) {
	cfg := &Config{
		Restic: ResticConfig{
			RepoPath: "/custom/restic-repo",
		},
	}

	path, err := cfg.GetResticRepoPath()
	if err != nil {
		t.Fatalf("GetResticRepoPath failed: %v", err)
	}

	if path != "/custom/restic-repo" {
		t.Errorf("GetResticRepoPath = %q, expected %q", path, "/custom/restic-repo")
	}
}

func TestGetResticRepoPathExpands(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home dir, skipping test")
	}

	cfg := &Config{
		Restic: ResticConfig{
			RepoPath: "~/my-restic-repo",
		},
	}

	path, err := cfg.GetResticRepoPath()
	if err != nil {
		t.Fatalf("GetResticRepoPath failed: %v", err)
	}

	expected := filepath.Join(home, "my-restic-repo")
	if path != expected {
		t.Errorf("GetResticRepoPath = %q, expected %q", path, expected)
	}
}

func TestGetResticPasswordEnvVarDefault(t *testing.T) {
	cfg := &Config{}

	envVar := cfg.GetResticPasswordEnvVar()
	if envVar != DefaultResticPasswordEnvVar {
		t.Errorf("GetResticPasswordEnvVar = %q, expected %q", envVar, DefaultResticPasswordEnvVar)
	}
}

func TestGetResticPasswordEnvVarCustom(t *testing.T) {
	cfg := &Config{
		Restic: ResticConfig{
			PasswordEnvVar: "MY_CUSTOM_PASSWORD",
		},
	}

	envVar := cfg.GetResticPasswordEnvVar()
	if envVar != "MY_CUSTOM_PASSWORD" {
		t.Errorf("GetResticPasswordEnvVar = %q, expected %q", envVar, "MY_CUSTOM_PASSWORD")
	}
}

func TestGetResticPassword(t *testing.T) {
	// Set password
	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password-123")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	cfg := &Config{}

	password, err := cfg.GetResticPassword()
	if err != nil {
		t.Fatalf("GetResticPassword failed: %v", err)
	}

	if password != "test-password-123" {
		t.Errorf("GetResticPassword = %q, expected %q", password, "test-password-123")
	}
}

func TestGetResticPasswordNotSet(t *testing.T) {
	// Ensure password is not set
	os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	cfg := &Config{}

	_, err := cfg.GetResticPassword()
	if err == nil {
		t.Error("GetResticPassword should fail when env var is not set")
	}
}

func TestGetResticPasswordCustomEnvVar(t *testing.T) {
	// Set custom env var
	os.Setenv("MY_RESTIC_PW", "custom-password")
	defer os.Unsetenv("MY_RESTIC_PW")

	cfg := &Config{
		Restic: ResticConfig{
			PasswordEnvVar: "MY_RESTIC_PW",
		},
	}

	password, err := cfg.GetResticPassword()
	if err != nil {
		t.Fatalf("GetResticPassword failed: %v", err)
	}

	if password != "custom-password" {
		t.Errorf("GetResticPassword = %q, expected %q", password, "custom-password")
	}
}

func TestResticConfigYAMLRoundtrip(t *testing.T) {
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

	// Write config with restic settings
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `
backup_dir: /backup
restic:
  repo_path: /my/restic/repo
  password_env_var: CUSTOM_PW_VAR
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load and verify
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Restic.RepoPath != "/my/restic/repo" {
		t.Errorf("Restic.RepoPath = %q, expected %q", cfg.Restic.RepoPath, "/my/restic/repo")
	}

	if cfg.Restic.PasswordEnvVar != "CUSTOM_PW_VAR" {
		t.Errorf("Restic.PasswordEnvVar = %q, expected %q", cfg.Restic.PasswordEnvVar, "CUSTOM_PW_VAR")
	}
}
