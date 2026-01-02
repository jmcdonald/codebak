package ports

import (
	"time"

	"github.com/jmcdonald/codebak/internal/config"
)

// TUIProjectInfo contains project metadata for display.
type TUIProjectInfo struct {
	Name       string
	Path       string
	Versions   int
	LastBackup time.Time
	TotalSize  int64
}

// TUIVersionInfo contains backup version metadata for display.
type TUIVersionInfo struct {
	File      string
	Size      int64
	FileCount int
	GitHead   string
	CreatedAt time.Time
}

// TUIBackupResult contains the result of a backup operation.
type TUIBackupResult struct {
	Size    int64
	Error   error
	Skipped bool
	Reason  string
}

// TUIService provides operations needed by the TUI.
// This abstraction allows the TUI to be tested without real filesystem/backup operations.
type TUIService interface {
	// LoadConfig loads the application configuration.
	LoadConfig() (*config.Config, error)

	// ListProjects returns all projects with their metadata.
	ListProjects(cfg *config.Config) ([]TUIProjectInfo, error)

	// ListVersions returns all backup versions for a project.
	ListVersions(cfg *config.Config, project string) ([]TUIVersionInfo, error)

	// RunBackup performs a backup of the specified project.
	RunBackup(cfg *config.Config, project string) TUIBackupResult

	// VerifyBackup verifies the latest backup of a project.
	// Returns nil if verified successfully, error otherwise.
	VerifyBackup(cfg *config.Config, project string) error
}
