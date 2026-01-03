package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNoHomeDir is returned when the home directory cannot be determined
var ErrNoHomeDir = fmt.Errorf("cannot determine home directory: HOME environment variable not set")

// Source represents a directory to scan for projects
type Source struct {
	Path  string `yaml:"path"`
	Label string `yaml:"label,omitempty"` // Display label (defaults to path basename)
	Icon  string `yaml:"icon,omitempty"`  // Emoji icon for TUI display
}

type Config struct {
	// Deprecated: Use Sources instead. Kept for backwards compatibility.
	SourceDir string   `yaml:"source_dir,omitempty"`
	Sources   []Source `yaml:"sources,omitempty"`
	// Individual projects outside source dirs (a la carte)
	Projects  []string `yaml:"projects,omitempty"`
	BackupDir string   `yaml:"backup_dir"`
	Schedule  string   `yaml:"schedule"`
	Time      string   `yaml:"time"`
	Exclude   []string `yaml:"exclude"`
	Retention struct {
		KeepLast int `yaml:"keep_last"`
	} `yaml:"retention"`
}

// GetSources returns all sources, migrating from SourceDir if needed
func (c *Config) GetSources() []Source {
	// If new Sources format is used, return it
	if len(c.Sources) > 0 {
		return c.Sources
	}
	// Migrate from old SourceDir format
	if c.SourceDir != "" {
		return []Source{{Path: c.SourceDir, Label: "Code", Icon: "📁"}}
	}
	return nil
}

func DefaultConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoHomeDir, err)
	}
	codeDir := filepath.Join(home, "code")
	return &Config{
		SourceDir: codeDir, // Deprecated but kept for backward compatibility
		Sources: []Source{
			{Path: codeDir, Label: "Code", Icon: "📁"},
		},
		BackupDir: filepath.Join(home, ".backups"),
		Schedule:  "daily",
		Time:      "03:00",
		Exclude: []string{
			"node_modules",
			".venv",
			"__pycache__",
			".git",
			"*.pyc",
			".DS_Store",
			".idea",
			".vscode",
			"target",
			"dist",
			"build",
		},
		Retention: struct {
			KeepLast int `yaml:"keep_last"`
		}{KeepLast: 30},
	}, nil
}

func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoHomeDir, err)
	}
	return filepath.Join(home, ".codebak", "config.yaml"), nil
}

func Load() (*Config, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Use defaults
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ExpandPath expands ~ to home directory. Returns error if path starts with ~
// but home directory cannot be determined.
func ExpandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%w: cannot expand %q", ErrNoHomeDir, path)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}
