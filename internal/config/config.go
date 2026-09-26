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

	// ExcludedProjects lists project names (basename of the repo directory)
	// to skip entirely. Unlike Exclude (a glob applied to paths inside a
	// project while zipping), this stops the project from being backed up
	// at all.
	ExcludedProjects []string `yaml:"excluded_projects,omitempty"`

	// MinIntervalHours is the minimum number of hours that must pass since a
	// project's last backup before it will be backed up again. Zero or
	// negative disables the check.
	MinIntervalHours float64 `yaml:"min_interval_hours,omitempty"`

	// RetentionOverrides maps a project name to a per-project keep_last
	// value, overriding Retention.KeepLast for that project only.
	RetentionOverrides map[string]int `yaml:"retention_overrides,omitempty"`

	// MaxTotalGBPerProject caps the total on-disk size (in GB) of a single
	// project's backup zips. After a backup, the oldest zips are pruned
	// until the project is at or under this cap, in addition to (whichever
	// is stricter than) Retention.KeepLast. Zero or negative disables the
	// check.
	MaxTotalGBPerProject float64 `yaml:"max_total_gb_per_project,omitempty"`

	// DryRun is a runtime-only flag, set from the CLI (e.g. `codebak run
	// --dry-run`). It is never persisted to config.yaml. When true, no zip
	// is written, no manifest is saved, and no backup file is deleted;
	// BackupResult reports what would have happened instead.
	DryRun bool `yaml:"-"`
}

// Retention and safety-limit defaults.
const (
	// LegacyDefaultKeepLast is the keep_last value codebak used before the
	// per-project size cap and safety limits existed. Preserved so a config
	// file written before those limits existed, and that does not
	// explicitly set retention.keep_last, keeps its historical behavior
	// instead of silently adopting the new, much lower default. See
	// Load() for how this is applied.
	LegacyDefaultKeepLast = 30

	// DefaultKeepLast is the safer default used for brand-new configs
	// (fresh `codebak init`, or Load() when no config file exists yet).
	DefaultKeepLast = 5

	// DefaultMinIntervalHours is the default debounce window between
	// backups of the same project.
	DefaultMinIntervalHours = 12

	// DefaultMaxTotalGBPerProject is the default per-project on-disk size
	// cap, in GB.
	DefaultMaxTotalGBPerProject = 50
)

// MaxTotalBytesPerProject returns MaxTotalGBPerProject converted to bytes.
// Returns 0 if the cap is disabled (MaxTotalGBPerProject <= 0).
func (c *Config) MaxTotalBytesPerProject() int64 {
	if c.MaxTotalGBPerProject <= 0 {
		return 0
	}
	return int64(c.MaxTotalGBPerProject * 1024 * 1024 * 1024)
}

// EffectiveKeepLast returns the keep_last value that applies to project,
// honoring RetentionOverrides when present.
func (c *Config) EffectiveKeepLast(project string) int {
	if c.RetentionOverrides != nil {
		if override, ok := c.RetentionOverrides[project]; ok {
			return override
		}
	}
	return c.Retention.KeepLast
}

// IsExcludedProject reports whether project is listed in ExcludedProjects.
// Matching is an exact match on the project (directory) name.
func (c *Config) IsExcludedProject(project string) bool {
	for _, excluded := range c.ExcludedProjects {
		if excluded == project {
			return true
		}
	}
	return false
}

// GetSources returns all sources, migrating from SourceDir if needed
func (c *Config) GetSources() []Source {
	// If new Sources format is used, return it with defaults applied
	if len(c.Sources) > 0 {
		return applySourceDefaults(c.Sources)
	}
	// Migrate from old SourceDir format
	if c.SourceDir != "" {
		return []Source{{Path: c.SourceDir, Label: "Code", Icon: "●", Type: SourceTypeGit}}
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
				result[i].Icon = "◆"
			} else {
				result[i].Icon = "●"
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
// #nosec G101 -- This is an env var name, not a credential
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
			{Path: codeDir, Label: "Code", Icon: "●"},
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
		}{KeepLast: DefaultKeepLast},
		MinIntervalHours:     DefaultMinIntervalHours,
		MaxTotalGBPerProject: DefaultMaxTotalGBPerProject,
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
			return cfg, nil // No config file yet: use the new, safer defaults.
		}
		return nil, err
	}

	// An existing config file was found. If it does not explicitly set
	// retention.keep_last, preserve the historical default (30) rather than
	// silently dropping to the new lower default (5): the lower default
	// applies to new configs, but must not change behavior for a config
	// the user already had in place. The user opts into the new
	// default by leaving keep_last unset in a freshly generated config, or
	// opts into any value at all by setting retention.keep_last explicitly
	// (which always wins, at any value, old or new).
	if !hasExplicitKeepLast(data) {
		cfg.Retention.KeepLast = LegacyDefaultKeepLast
	}

	// Same rule for the two newer safety limits added alongside the lower
	// keep_last default: min_interval_hours and max_total_gb_per_project.
	// Both ship with non-zero defaults for a brand-new config, but an
	// existing config that has never heard of these keys must keep
	// behaving exactly as it does today, not silently start debouncing
	// runs or pruning by size.
	// Absent means off (0); present at any value, including 0, always wins.
	if !hasExplicitMinIntervalHours(data) {
		cfg.MinIntervalHours = 0
	}
	if !hasExplicitMaxTotalGBPerProject(data) {
		cfg.MaxTotalGBPerProject = 0
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// hasExplicitKeepLast reports whether the raw YAML config source explicitly
// sets retention.keep_last, as opposed to that field being absent and
// therefore defaulted.
func hasExplicitKeepLast(data []byte) bool {
	var raw struct {
		Retention struct {
			KeepLast *int `yaml:"keep_last"`
		} `yaml:"retention"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	return raw.Retention.KeepLast != nil
}

// hasExplicitMinIntervalHours reports whether the raw YAML config source
// explicitly sets min_interval_hours, as opposed to that field being absent
// and therefore defaulted.
func hasExplicitMinIntervalHours(data []byte) bool {
	var raw struct {
		MinIntervalHours *float64 `yaml:"min_interval_hours"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	return raw.MinIntervalHours != nil
}

// hasExplicitMaxTotalGBPerProject reports whether the raw YAML config
// source explicitly sets max_total_gb_per_project, as opposed to that field
// being absent and therefore defaulted.
func hasExplicitMaxTotalGBPerProject(data []byte) bool {
	var raw struct {
		MaxTotalGBPerProject *float64 `yaml:"max_total_gb_per_project"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	return raw.MaxTotalGBPerProject != nil
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
