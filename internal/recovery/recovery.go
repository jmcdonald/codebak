package recovery

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mcdonaldj/codebak/internal/adapters/osfs"
	"github.com/mcdonaldj/codebak/internal/adapters/ziparchiver"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
	"github.com/mcdonaldj/codebak/internal/ports"
)

// RecoverOptions configures a recovery operation.
type RecoverOptions struct {
	Project string
	Version string // YYYYMMDD-HHMMSS format, empty for latest
	Wipe    bool   // Delete current before restore
	Archive bool   // Archive current before restore
}

// Service provides recovery operations with injected dependencies.
type Service struct {
	fs       ports.FileSystem
	archiver ports.Archiver
}

// NewService creates a new recovery service with the given dependencies.
func NewService(fs ports.FileSystem, archiver ports.Archiver) *Service {
	return &Service{
		fs:       fs,
		archiver: archiver,
	}
}

// NewDefaultService creates a recovery service with real production dependencies.
func NewDefaultService() *Service {
	return NewService(
		osfs.New(),
		ziparchiver.New(),
	)
}

// Verify checks the integrity of a backup by comparing checksums.
func (s *Service) Verify(cfg *config.Config, project, version string) error {
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return err
	}

	m, err := manifest.Load(backupDir, project)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	// Find the backup entry
	var entry *manifest.BackupEntry
	if version == "" {
		entry = m.LatestBackup()
	} else {
		for i := range m.Backups {
			if m.Backups[i].File == version+".zip" {
				entry = &m.Backups[i]
				break
			}
		}
	}

	if entry == nil {
		if version == "" {
			return fmt.Errorf("no backups found for project: %s", project)
		}
		return fmt.Errorf("backup not found: %s", version)
	}

	// Verify checksum
	zipPath := filepath.Join(backupDir, project, entry.File)
	actualChecksum, err := manifest.ComputeSHA256(zipPath)
	if err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	if actualChecksum != entry.SHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", entry.SHA256, actualChecksum)
	}

	return nil
}

// Recover restores a project from backup.
func (s *Service) Recover(cfg *config.Config, opts RecoverOptions) error {
	sourceDir, err := config.ExpandPath(cfg.SourceDir)
	if err != nil {
		return err
	}
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return err
	}
	projectPath := filepath.Join(sourceDir, opts.Project)

	// Load manifest
	m, err := manifest.Load(backupDir, opts.Project)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	// Find the backup entry
	var entry *manifest.BackupEntry
	if opts.Version == "" {
		entry = m.LatestBackup()
	} else {
		for i := range m.Backups {
			if m.Backups[i].File == opts.Version+".zip" {
				entry = &m.Backups[i]
				break
			}
		}
	}

	if entry == nil {
		if opts.Version == "" {
			return fmt.Errorf("no backups found for project: %s", opts.Project)
		}
		return fmt.Errorf("backup version not found: %s", opts.Version)
	}

	// Verify checksum before recovery
	zipPath := filepath.Join(backupDir, opts.Project, entry.File)
	if err := s.Verify(cfg, opts.Project, opts.Version); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Handle existing project directory
	if _, err := s.fs.Stat(projectPath); err == nil {
		if opts.Wipe {
			// Delete current
			if err := s.fs.RemoveAll(projectPath); err != nil {
				return fmt.Errorf("removing current project: %w", err)
			}
		} else if opts.Archive {
			// Archive current first
			archiveName := fmt.Sprintf("%s-archived-%s", opts.Project, time.Now().Format("20060102-150405"))
			archivePath := filepath.Join(sourceDir, archiveName)
			if err := s.fs.Rename(projectPath, archivePath); err != nil {
				return fmt.Errorf("archiving current project: %w", err)
			}
		} else {
			return fmt.Errorf("project already exists: %s (use --wipe or --archive)", projectPath)
		}
	}

	// Extract zip using archiver
	if err := s.archiver.Extract(zipPath, sourceDir); err != nil {
		return fmt.Errorf("extracting backup: %w", err)
	}

	return nil
}

// ListVersions returns all backup versions for a project.
func (s *Service) ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error) {
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return nil, err
	}

	m, err := manifest.Load(backupDir, project)
	if err != nil {
		return nil, err
	}

	return m.Backups, nil
}

// ============================================================================
// Backward-compatible package-level functions using default service
// ============================================================================

var defaultService = NewDefaultService()

// Verify checks the integrity of a backup by comparing checksums.
// Uses the default production dependencies.
func Verify(cfg *config.Config, project, version string) error {
	return defaultService.Verify(cfg, project, version)
}

// Recover restores a project from backup.
// Uses the default production dependencies.
func Recover(cfg *config.Config, opts RecoverOptions) error {
	return defaultService.Recover(cfg, opts)
}

// ListVersions returns all backup versions for a project.
// Uses the default production dependencies.
func ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error) {
	return defaultService.ListVersions(cfg, project)
}
