package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SourceDir string   `yaml:"source_dir"`
	BackupDir string   `yaml:"backup_dir"`
	Schedule  string   `yaml:"schedule"`
	Time      string   `yaml:"time"`
	Exclude   []string `yaml:"exclude"`
	Retention struct {
		KeepLast int `yaml:"keep_last"`
	} `yaml:"retention"`
}

func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "." // Fallback to current directory
	}
	return &Config{
		SourceDir: filepath.Join(home, "code"),
		BackupDir: filepath.Join(home, "backups"),
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
	}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".codebak", "config.yaml")
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	path := ConfigPath()
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
	path := ConfigPath()

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

// ExpandPath expands ~ to home directory
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path // Return unexpanded if home unavailable
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
