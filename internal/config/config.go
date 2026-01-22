package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNoHomeDir is returned when the home directory cannot be determined
var ErrNoHomeDir = fmt.Errorf("cannot determine home directory: HOME environment variable not set")

// SourceType defines how a source should be backed up
type SourceType string

const (
	// SourceTypeGit backs up using git bundles (default for code directories)
	SourceTypeGit SourceType = "git"
	// SourceTypeSensitive backs up using restic for encrypted incremental backups
	SourceTypeSensitive SourceType = "sensitive"
)

// DefaultSensitivePaths returns the default paths to back up with restic encryption.
// These are common dotfiles and config directories containing sensitive data.
func DefaultSensitivePaths() []string {
	return []string{
		"~/.ssh",
		"~/.aws",
		"~/.azure",
		"~/.gitconfig",
		"~/.zshrc",
		"~/.zprofile",
		"~/.bashrc",
		"~/.bash_profile",
		"~/.tmux.conf",
		"~/.config",
		"~/.haute",
		"~/.claude",
		"~/.beads",
		"~/.jervais",
	}
}

// Source represents a directory to scan for projects
type Source struct {
	Path  string     `yaml:"path"`
	Label string     `yaml:"label,omitempty"` // Display label (defaults to path basename)
	Icon  string     `yaml:"icon,omitempty"`  // Emoji icon for TUI display
	Type  SourceType `yaml:"type,omitempty"`  // Backup type: git (default) or sensitive
}

// ResticConfig holds configuration for restic encrypted backups.
type ResticConfig struct {
	// RepoPath is the path to the restic repository for sensitive backups.
	// Defaults to ~/.codebak/restic-repo
	RepoPath string `yaml:"repo_path,omitempty"`
	// PasswordEnvVar is the environment variable name containing the repository password.
	// Defaults to CODEBAK_RESTIC_PASSWORD
	PasswordEnvVar string `yaml:"password_env_var,omitempty"`
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
	// Restic configuration for sensitive path backups
	Restic ResticConfig `yaml:"restic,omitempty"`
}

// GetSources returns all sources, migrating from SourceDir if needed
func (c *Config) GetSources() []Source {
	// If new Sources format is used, return it with defaults applied
	if len(c.Sources) > 0 {
		return applySourceDefaults(c.Sources)
	}
	// Migrate from old SourceDir format
	if c.SourceDir != "" {
		return []Source{{Path: c.SourceDir, Label: "Code", Icon: "📁", Type: SourceTypeGit}}
	}
	return nil
}

// applySourceDefaults ensures all sources have their default values set
func applySourceDefaults(sources []Source) []Source {
	result := make([]Source, len(sources))
	for i, s := range sources {
		result[i] = s
		// Default type is git
		if result[i].Type == "" {
			result[i].Type = SourceTypeGit
		}
		// Default icon based on type
		if result[i].Icon == "" {
			if result[i].Type == SourceTypeSensitive {
				result[i].Icon = "🔒"
			} else {
				result[i].Icon = "📁"
			}
		}
	}
	return result
}

// GetSourcesByType returns sources filtered by type
func (c *Config) GetSourcesByType(t SourceType) []Source {
	var result []Source
	for _, s := range c.GetSources() {
		if s.Type == t {
			result = append(result, s)
		}
	}
	return result
}

// DefaultResticRepoPath returns the default restic repository path.
func DefaultResticRepoPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoHomeDir, err)
	}
	return filepath.Join(home, ".codebak", "restic-repo"), nil
}

// DefaultResticPasswordEnvVar is the default environment variable for the restic password.
const DefaultResticPasswordEnvVar = "CODEBAK_RESTIC_PASSWORD"

// GetResticRepoPath returns the restic repository path with defaults applied.
func (c *Config) GetResticRepoPath() (string, error) {
	if c.Restic.RepoPath != "" {
		return ExpandPath(c.Restic.RepoPath)
	}
	return DefaultResticRepoPath()
}

// GetResticPasswordEnvVar returns the environment variable name for the restic password.
func (c *Config) GetResticPasswordEnvVar() string {
	if c.Restic.PasswordEnvVar != "" {
		return c.Restic.PasswordEnvVar
	}
	return DefaultResticPasswordEnvVar
}

// GetResticPassword retrieves the restic password from the configured environment variable.
// Returns an error if the environment variable is not set.
func (c *Config) GetResticPassword() (string, error) {
	envVar := c.GetResticPasswordEnvVar()
	password := os.Getenv(envVar)
	if password == "" {
		return "", fmt.Errorf("restic password not set: environment variable %s is empty", envVar)
	}
	return password, nil
}

// IsValidSourceType checks if a source type is valid
func IsValidSourceType(t SourceType) bool {
	return t == SourceTypeGit || t == SourceTypeSensitive
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
		BackupDir: filepath.Join(home, ".codebak", "backups"),
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
