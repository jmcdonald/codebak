// Package cli provides the command-line interface with injectable io.Writer for testing.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mcdonaldj/codebak/internal/backup"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/launchd"
	"github.com/mcdonaldj/codebak/internal/manifest"
	"github.com/mcdonaldj/codebak/internal/recovery"
)

// ConfigService provides configuration operations for the CLI.
type ConfigService interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
	ConfigPath() (string, error)
	DefaultConfig() (*config.Config, error)
}

// BackupService provides backup operations for the CLI.
type BackupService interface {
	BackupProject(cfg *config.Config, project string) backup.BackupResult
	RunBackup(cfg *config.Config) ([]backup.BackupResult, error)
}

// RecoveryService provides recovery operations for the CLI.
type RecoveryService interface {
	Verify(cfg *config.Config, project, version string) error
	Recover(cfg *config.Config, opts recovery.RecoverOptions) error
	ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error)
}

// LaunchdService provides launchd operations for the CLI.
type LaunchdService interface {
	IsInstalled() bool
	Install(hour, minute int) error
	Uninstall() error
	Status() (bool, error)
	PlistPath() string
	LogPath() string
}

// CLI represents the command-line interface with injectable dependencies.
type CLI struct {
	Out     io.Writer // Standard output
	Err     io.Writer // Standard error
	Version string    // Application version
	Args    []string  // Command arguments (like os.Args)

	// Exit function for testability (defaults to os.Exit)
	Exit func(code int)

	// Injectable dependencies (nil means use defaults)
	ConfigSvc   ConfigService
	BackupSvc   BackupService
	RecoverySvc RecoveryService
	LaunchdSvc  LaunchdService

	// Color functions (can be disabled for testing)
	green  func(a ...interface{}) string
	yellow func(a ...interface{}) string
	cyan   func(a ...interface{}) string
	gray   func(a ...interface{}) string
	red    func(a ...interface{}) string
}

// New creates a new CLI with default settings.
func New(version string) *CLI {
	return &CLI{
		Out:     os.Stdout,
		Err:     os.Stderr,
		Version: version,
		Args:    os.Args,
		Exit:    os.Exit,
		green:   color.New(color.FgGreen, color.Bold).SprintFunc(),
		yellow:  color.New(color.FgYellow).SprintFunc(),
		cyan:    color.New(color.FgCyan).SprintFunc(),
		gray:    color.New(color.FgHiBlack).SprintFunc(),
		red:     color.New(color.FgRed).SprintFunc(),
	}
}

// NewForTesting creates a CLI configured for testing (no colors, captured output).
func NewForTesting(out, errOut io.Writer, args []string) *CLI {
	noColor := func(a ...interface{}) string { return fmt.Sprint(a...) }
	exitCode := 0
	return &CLI{
		Out:     out,
		Err:     errOut,
		Version: "test",
		Args:    args,
		Exit:    func(code int) { exitCode = code; _ = exitCode },
		green:   noColor,
		yellow:  noColor,
		cyan:    noColor,
		gray:    noColor,
		red:     noColor,
	}
}

// defaultConfigService wraps the config package functions.
type defaultConfigService struct{}

func (d *defaultConfigService) Load() (*config.Config, error) { return config.Load() }
func (d *defaultConfigService) Save(cfg *config.Config) error { return cfg.Save() }
func (d *defaultConfigService) ConfigPath() (string, error)       { return config.ConfigPath() }
func (d *defaultConfigService) DefaultConfig() (*config.Config, error) { return config.DefaultConfig() }

// defaultBackupService wraps the backup package functions.
type defaultBackupService struct{}

func (d *defaultBackupService) BackupProject(cfg *config.Config, project string) backup.BackupResult {
	return backup.BackupProject(cfg, project)
}
func (d *defaultBackupService) RunBackup(cfg *config.Config) ([]backup.BackupResult, error) {
	return backup.RunBackup(cfg)
}

// defaultRecoveryService wraps the recovery package functions.
type defaultRecoveryService struct{}

func (d *defaultRecoveryService) Verify(cfg *config.Config, project, version string) error {
	return recovery.Verify(cfg, project, version)
}
func (d *defaultRecoveryService) Recover(cfg *config.Config, opts recovery.RecoverOptions) error {
	return recovery.Recover(cfg, opts)
}
func (d *defaultRecoveryService) ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error) {
	return recovery.ListVersions(cfg, project)
}

// defaultLaunchdService wraps the launchd package functions.
type defaultLaunchdService struct{}

func (d *defaultLaunchdService) IsInstalled() bool            { return launchd.IsInstalled() }
func (d *defaultLaunchdService) Install(hour, minute int) error { return launchd.Install(hour, minute) }
func (d *defaultLaunchdService) Uninstall() error             { return launchd.Uninstall() }
func (d *defaultLaunchdService) Status() (bool, error)        { return launchd.Status() }
func (d *defaultLaunchdService) PlistPath() string            { return launchd.PlistPath() }
func (d *defaultLaunchdService) LogPath() string              { return launchd.LogPath() }

// Helper methods to get the service or default
func (c *CLI) configSvc() ConfigService {
	if c.ConfigSvc != nil {
		return c.ConfigSvc
	}
	return &defaultConfigService{}
}

func (c *CLI) backupSvc() BackupService {
	if c.BackupSvc != nil {
		return c.BackupSvc
	}
	return &defaultBackupService{}
}

func (c *CLI) recoverySvc() RecoveryService {
	if c.RecoverySvc != nil {
		return c.RecoverySvc
	}
	return &defaultRecoveryService{}
}

func (c *CLI) launchdSvc() LaunchdService {
	if c.LaunchdSvc != nil {
		return c.LaunchdSvc
	}
	return &defaultLaunchdService{}
}

// Run executes the CLI with the configured arguments.
func (c *CLI) Run() {
	if len(c.Args) < 2 {
		// No command - would launch TUI, but we skip that for CLI testing
		fmt.Fprintln(c.Out, "No command specified. Use 'codebak help' for usage.")
		return
	}

	switch c.Args[1] {
	case "run":
		c.RunBackup()
	case "init":
		c.InitConfig()
	case "install":
		c.InstallLaunchd()
	case "uninstall":
		c.UninstallLaunchd()
	case "status":
		c.ShowStatus()
	case "verify":
		c.RunVerify()
	case "recover":
		c.RunRecover()
	case "list":
		c.ListBackups()
	case "version", "-v", "--version":
		fmt.Fprintf(c.Out, "codebak v%s\n", c.Version)
	case "help", "-h", "--help":
		c.PrintUsage()
	default:
		fmt.Fprintf(c.Err, "Unknown command: %s\n", c.Args[1])
		c.PrintUsage()
		c.Exit(1)
	}
}

// PrintUsage prints the help message.
func (c *CLI) PrintUsage() {
	fmt.Fprintln(c.Out, `codebak - Incremental Code Backup Tool

Usage:
  codebak                                  Launch interactive TUI
  codebak ui                               Launch interactive TUI
  codebak run [project]                    Backup all changed projects (or specific project)
  codebak list <project>                   List all backup versions for a project
  codebak verify <project> [version]       Verify backup integrity
  codebak recover <project> [--wipe|--archive] [--version=YYYYMMDD-HHMMSS]
                                           Recover project from backup
  codebak install                          Install daily launchd schedule (3am)
  codebak uninstall                        Remove launchd schedule
  codebak status                           Show launchd status
  codebak init                             Create default config file
  codebak version, -v                      Show version
  codebak help, -h                         Show this help

Config: ~/.codebak/config.yaml`)
}

// InitConfig creates the default config file.
func (c *CLI) InitConfig() {
	svc := c.configSvc()
	cfg, err := svc.DefaultConfig()
	if err != nil {
		fmt.Fprintf(c.Err, "Error: %v\n", err)
		c.Exit(1)
		return
	}
	if err := svc.Save(cfg); err != nil {
		fmt.Fprintf(c.Err, "Error saving config: %v\n", err)
		c.Exit(1)
		return
	}
	path, err := svc.ConfigPath()
	if err != nil {
		fmt.Fprintf(c.Err, "Error: %v\n", err)
		c.Exit(1)
		return
	}
	fmt.Fprintf(c.Out, "Created config at %s\n", path)
}

// RunBackup runs the backup command.
func (c *CLI) RunBackup() {
	cfgSvc := c.configSvc()
	backupSvc := c.backupSvc()

	cfg, err := cfgSvc.Load()
	if err != nil {
		fmt.Fprintf(c.Err, "Error loading config: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintf(c.Out, "%s Scanning %s...\n", c.cyan("=>"), cfg.SourceDir)

	var results []backup.BackupResult
	if len(c.Args) > 2 {
		project := c.Args[2]
		result := backupSvc.BackupProject(cfg, project)
		results = []backup.BackupResult{result}
	} else {
		results, err = backupSvc.RunBackup(cfg)
		if err != nil {
			fmt.Fprintf(c.Err, "Error: %v\n", err)
			c.Exit(1)
			return
		}
	}

	backedUp := 0
	skipped := 0
	errors := 0

	fmt.Fprintln(c.Out)
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(c.Out, "  %s %s: %v\n", c.red("x"), r.Project, r.Error)
			errors++
		} else if r.Skipped {
			fmt.Fprintf(c.Out, "  %s %s %s\n", c.gray("-"), c.gray(r.Project), c.gray("("+r.Reason+")"))
			skipped++
		} else {
			sizeStr := backup.FormatSize(r.Size)
			fmt.Fprintf(c.Out, "  %s %s %s %s %d files\n",
				c.green("*"),
				r.Project,
				c.yellow(sizeStr),
				c.gray(r.Reason),
				r.FileCount)
			backedUp++
		}
	}

	fmt.Fprintln(c.Out)
	fmt.Fprintf(c.Out, "Done: %s backed up, %s skipped",
		c.green(fmt.Sprintf("%d", backedUp)),
		c.gray(fmt.Sprintf("%d", skipped)))
	if errors > 0 {
		fmt.Fprintf(c.Out, ", %s errors", c.red(fmt.Sprintf("%d", errors)))
	}
	fmt.Fprintln(c.Out)
}

// InstallLaunchd installs the launchd schedule.
func (c *CLI) InstallLaunchd() {
	svc := c.launchdSvc()

	if svc.IsInstalled() {
		fmt.Fprintln(c.Out, "launchd already installed. Uninstall first to reinstall.")
		c.Exit(1)
		return
	}

	if err := svc.Install(3, 0); err != nil {
		fmt.Fprintf(c.Err, "Error installing launchd: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintf(c.Out, "%s Installed launchd schedule (daily at 3:00 AM)\n", c.green("*"))
	fmt.Fprintf(c.Out, "  Plist: %s\n", svc.PlistPath())
	fmt.Fprintf(c.Out, "  Log:   %s\n", svc.LogPath())
}

// UninstallLaunchd removes the launchd schedule.
func (c *CLI) UninstallLaunchd() {
	svc := c.launchdSvc()

	if !svc.IsInstalled() {
		fmt.Fprintln(c.Out, "launchd not installed.")
		c.Exit(1)
		return
	}

	if err := svc.Uninstall(); err != nil {
		fmt.Fprintf(c.Err, "Error uninstalling launchd: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintf(c.Out, "%s Uninstalled launchd schedule\n", c.yellow("-"))
}

// ShowStatus shows the current status.
func (c *CLI) ShowStatus() {
	cfgSvc := c.configSvc()
	launchdSvc := c.launchdSvc()

	cfg, err := cfgSvc.Load()
	if err != nil {
		fmt.Fprintf(c.Err, "Error loading config: %v\n", err)
		c.Exit(1)
		return
	}

	configPath, err := cfgSvc.ConfigPath()
	if err != nil {
		fmt.Fprintf(c.Err, "Error: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintln(c.Out, "codebak status:")
	fmt.Fprintf(c.Out, "  Source:  %s\n", cfg.SourceDir)
	fmt.Fprintf(c.Out, "  Backup:  %s\n", cfg.BackupDir)
	fmt.Fprintf(c.Out, "  Config:  %s\n", configPath)

	if launchdSvc.IsInstalled() {
		loaded, _ := launchdSvc.Status()
		if loaded {
			fmt.Fprintf(c.Out, "  launchd: %s\n", c.green("installed & loaded"))
		} else {
			fmt.Fprintf(c.Out, "  launchd: %s\n", c.gray("installed (not loaded)"))
		}
	} else {
		fmt.Fprintf(c.Out, "  launchd: %s\n", c.gray("not installed"))
	}
}

// RunVerify verifies a backup.
func (c *CLI) RunVerify() {
	if len(c.Args) < 3 {
		fmt.Fprintln(c.Out, "Usage: codebak verify <project> [version]")
		c.Exit(1)
		return
	}

	cfgSvc := c.configSvc()
	recoverySvc := c.recoverySvc()

	cfg, err := cfgSvc.Load()
	if err != nil {
		fmt.Fprintf(c.Err, "Error loading config: %v\n", err)
		c.Exit(1)
		return
	}

	project := c.Args[2]
	version := ""
	if len(c.Args) > 3 {
		version = c.Args[3]
	}

	if err := recoverySvc.Verify(cfg, project, version); err != nil {
		fmt.Fprintf(c.Err, "Verification failed: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintf(c.Out, "%s Checksum verified for %s\n", c.green("*"), project)
}

// RunRecover recovers a project from backup.
func (c *CLI) RunRecover() {
	if len(c.Args) < 3 {
		fmt.Fprintln(c.Out, "Usage: codebak recover <project> [--wipe|--archive] [--version=YYYYMMDD-HHMMSS]")
		c.Exit(1)
		return
	}

	cfgSvc := c.configSvc()
	recoverySvc := c.recoverySvc()

	cfg, err := cfgSvc.Load()
	if err != nil {
		fmt.Fprintf(c.Err, "Error loading config: %v\n", err)
		c.Exit(1)
		return
	}

	opts := recovery.RecoverOptions{
		Project: c.Args[2],
	}

	// Parse flags
	for _, arg := range c.Args[3:] {
		switch {
		case arg == "--wipe":
			opts.Wipe = true
		case arg == "--archive":
			opts.Archive = true
		case strings.HasPrefix(arg, "--version="):
			opts.Version = strings.TrimPrefix(arg, "--version=")
		}
	}

	if opts.Wipe && opts.Archive {
		fmt.Fprintln(c.Out, "Cannot use both --wipe and --archive")
		c.Exit(1)
		return
	}

	if opts.Wipe {
		fmt.Fprintf(c.Out, "%s Recovering %s (wiping current)...\n", c.yellow("!"), opts.Project)
	} else if opts.Archive {
		fmt.Fprintf(c.Out, "%s Recovering %s (archiving current)...\n", c.yellow("!"), opts.Project)
	} else {
		fmt.Fprintf(c.Out, "Recovering %s...\n", opts.Project)
	}

	if err := recoverySvc.Recover(cfg, opts); err != nil {
		fmt.Fprintf(c.Err, "Recovery failed: %v\n", err)
		c.Exit(1)
		return
	}

	fmt.Fprintf(c.Out, "%s Successfully recovered %s\n", c.green("*"), opts.Project)
}

// ListBackups lists all backups for a project.
func (c *CLI) ListBackups() {
	if len(c.Args) < 3 {
		fmt.Fprintln(c.Out, "Usage: codebak list <project>")
		c.Exit(1)
		return
	}

	cfgSvc := c.configSvc()
	recoverySvc := c.recoverySvc()

	cfg, err := cfgSvc.Load()
	if err != nil {
		fmt.Fprintf(c.Err, "Error loading config: %v\n", err)
		c.Exit(1)
		return
	}

	project := c.Args[2]

	backups, err := recoverySvc.ListVersions(cfg, project)
	if err != nil {
		fmt.Fprintf(c.Err, "Error: %v\n", err)
		c.Exit(1)
		return
	}

	if len(backups) == 0 {
		fmt.Fprintf(c.Out, "No backups found for %s\n", project)
		return
	}

	fmt.Fprintf(c.Out, "Backups for %s:\n\n", c.cyan(project))
	fmt.Fprintf(c.Out, "  %-20s %10s %8s %s\n", "VERSION", "SIZE", "FILES", "GIT HEAD")
	fmt.Fprintf(c.Out, "  %-20s %10s %8s %s\n", "-------", "----", "-----", "--------")

	for _, b := range backups {
		gitHead := b.GitHead
		if len(gitHead) > 7 {
			gitHead = gitHead[:7]
		}
		if gitHead == "" {
			gitHead = c.gray("-")
		}
		fmt.Fprintf(c.Out, "  %-20s %10s %8d %s\n",
			strings.TrimSuffix(b.File, ".zip"),
			backup.FormatSize(b.SizeBytes),
			b.FileCount,
			gitHead)
	}
}
