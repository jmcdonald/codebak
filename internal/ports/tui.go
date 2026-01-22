package ports

import (
	"time"

	"github.com/jmcdonald/codebak/internal/config"
)

// TUIProjectInfo contains project metadata for display.
type TUIProjectInfo struct {
	Name        string
	Path        string
	SourceLabel string // Label of the source directory (e.g., "Code", "Work")
	SourceIcon  string // Icon for display (e.g., "📁", "💼", "🔒")
	SourceType  string // Source type: "git" or "sensitive"
	Versions    int
	LastBackup  time.Time
	TotalSize   int64
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

// TUISnapshotInfo contains restic snapshot metadata for display.
type TUISnapshotInfo struct {
	ID        string    // Short snapshot ID
	Time      time.Time // Snapshot creation time
	Paths     []string  // Backed up paths
	Tags      []string  // Snapshot tags
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

	// ListSnapshots returns all restic snapshots for sensitive sources.
	// Returns snapshots filtered by the given tag (typically "codebak-sensitive").
	ListSnapshots(cfg *config.Config, tag string) ([]TUISnapshotInfo, error)

	// RunBackup performs a backup of the specified project.
	RunBackup(cfg *config.Config, project string) TUIBackupResult

	// VerifyBackup verifies the latest backup of a project.
	// Returns nil if verified successfully, error otherwise.
	VerifyBackup(cfg *config.Config, project string) error
}
