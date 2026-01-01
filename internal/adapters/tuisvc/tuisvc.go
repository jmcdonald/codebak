// Package tuisvc provides the real implementation of ports.TUIService.
package tuisvc

import (
	"fmt"
	"path/filepath"

	"github.com/mcdonaldj/codebak/internal/backup"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
	"github.com/mcdonaldj/codebak/internal/ports"
)

// Service implements ports.TUIService using real filesystem operations.
type Service struct{}

// New creates a new TUI service.
func New() *Service {
	return &Service{}
}

// LoadConfig loads the application configuration.
func (s *Service) LoadConfig() (*config.Config, error) {
	return config.Load()
}

// ListProjects returns all projects with their metadata.
func (s *Service) ListProjects(cfg *config.Config) ([]ports.TUIProjectInfo, error) {
	sourceDir := config.ExpandPath(cfg.SourceDir)
	backupDir := config.ExpandPath(cfg.BackupDir)

	projects, err := backup.ListProjects(sourceDir)
	if err != nil {
		return nil, err
	}

	var result []ports.TUIProjectInfo
	for _, name := range projects {
		item := ports.TUIProjectInfo{
			Name: name,
			Path: filepath.Join(sourceDir, name),
		}

		// Load manifest if exists
		mf, err := manifest.Load(backupDir, name)
		if err == nil && len(mf.Backups) > 0 {
			item.Versions = len(mf.Backups)
			latest := mf.LatestBackup()
			if latest != nil {
				item.LastBackup = latest.CreatedAt
			}
			for _, b := range mf.Backups {
				item.TotalSize += b.SizeBytes
			}
		}

		result = append(result, item)
	}

	return result, nil
}

// ListVersions returns all backup versions for a project.
func (s *Service) ListVersions(cfg *config.Config, project string) ([]ports.TUIVersionInfo, error) {
	backupDir := config.ExpandPath(cfg.BackupDir)

	mf, err := manifest.Load(backupDir, project)
	if err != nil {
		return nil, err
	}

	var result []ports.TUIVersionInfo
	for _, b := range mf.Backups {
		result = append(result, ports.TUIVersionInfo{
			File:      b.File,
			Size:      b.SizeBytes,
			FileCount: b.FileCount,
			GitHead:   b.GitHead,
			CreatedAt: b.CreatedAt,
		})
	}

	// Reverse so newest is first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// RunBackup performs a backup of the specified project.
func (s *Service) RunBackup(cfg *config.Config, project string) ports.TUIBackupResult {
	result := backup.BackupProject(cfg, project)
	return ports.TUIBackupResult{
		Size:    result.Size,
		Error:   result.Error,
		Skipped: result.Skipped,
		Reason:  result.Reason,
	}
}

// VerifyBackup verifies the latest backup of a project.
func (s *Service) VerifyBackup(cfg *config.Config, project string) error {
	backupDir := config.ExpandPath(cfg.BackupDir)
	mf, err := manifest.Load(backupDir, project)
	if err != nil || len(mf.Backups) == 0 {
		return fmt.Errorf("no backups to verify")
	}

	latest := mf.LatestBackup()
	zipPath := filepath.Join(backupDir, project, latest.File)
	actualChecksum, err := manifest.ComputeSHA256(zipPath)
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}

	if actualChecksum != latest.SHA256 {
		return fmt.Errorf("checksum mismatch")
	}

	return nil
}

// Compile-time check that Service implements ports.TUIService.
var _ ports.TUIService = (*Service)(nil)
