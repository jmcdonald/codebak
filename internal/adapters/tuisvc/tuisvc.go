// Package tuisvc provides the real implementation of ports.TUIService.
package tuisvc

import (
	"fmt"
	"path/filepath"

	"github.com/jmcdonald/codebak/internal/adapters/execrestic"
	"github.com/jmcdonald/codebak/internal/backup"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/manifest"
	"github.com/jmcdonald/codebak/internal/ports"
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

// ListProjects returns all projects with their metadata from all configured sources.
func (s *Service) ListProjects(cfg *config.Config) ([]ports.TUIProjectInfo, error) {
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return nil, err
	}

	var result []ports.TUIProjectInfo
	seen := make(map[string]bool) // Track project names to avoid duplicates

	// Get snapshot count for sensitive sources (cached for all sensitive items)
	sensitiveSnapshotCount := 0
	if len(cfg.GetSourcesByType(config.SourceTypeSensitive)) > 0 {
		snapshots, err := s.ListSnapshots(cfg, "codebak-sensitive")
		if err == nil {
			sensitiveSnapshotCount = len(snapshots)
		}
	}

	// Iterate over all sources
	for _, source := range cfg.GetSources() {
		sourceDir, err := config.ExpandPath(source.Path)
		if err != nil {
			continue // Skip sources that can't be expanded
		}

		// Sensitive sources are shown as a single item, not as individual projects
		if source.Type == config.SourceTypeSensitive {
			label := source.Label
			if label == "" {
				label = filepath.Base(sourceDir)
			}
			item := ports.TUIProjectInfo{
				Name:        label,
				Path:        sourceDir,
				SourceLabel: "Sensitive",
				SourceIcon:  source.Icon,
				SourceType:  string(source.Type),
				Versions:    sensitiveSnapshotCount,
			}
			result = append(result, item)
			continue
		}

		projects, err := backup.ListProjects(sourceDir)
		if err != nil {
			continue // Skip sources that can't be read
		}

		for _, name := range projects {
			// Skip if we've already seen this project name
			if seen[name] {
				continue
			}
			seen[name] = true

			item := ports.TUIProjectInfo{
				Name:        name,
				Path:        filepath.Join(sourceDir, name),
				SourceLabel: source.Label,
				SourceIcon:  source.Icon,
				SourceType:  string(source.Type),
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
	}

	// Also include individual projects from cfg.Projects
	for _, projectPath := range cfg.Projects {
		expandedPath, err := config.ExpandPath(projectPath)
		if err != nil {
			continue
		}
		name := filepath.Base(expandedPath)
		if seen[name] {
			continue
		}
		seen[name] = true

		item := ports.TUIProjectInfo{
			Name:        name,
			Path:        expandedPath,
			SourceLabel: "Project",
			SourceIcon:  "■",
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
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return nil, err
	}

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
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		return err
	}
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

// ListSnapshots returns all restic snapshots for sensitive sources.
func (s *Service) ListSnapshots(cfg *config.Config, tag string) ([]ports.TUISnapshotInfo, error) {
	repoPath, err := cfg.GetResticRepoPath()
	if err != nil {
		return nil, err
	}

	password, err := cfg.GetResticPassword()
	if err != nil {
		// No password configured - return empty list rather than error
		return []ports.TUISnapshotInfo{}, nil
	}

	restic := execrestic.New()
	if !restic.IsInitialized(repoPath) {
		// No repo yet - return empty list
		return []ports.TUISnapshotInfo{}, nil
	}

	var tags []string
	if tag != "" {
		tags = []string{tag}
	}

	snapshots, err := restic.Snapshots(repoPath, password, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	result := make([]ports.TUISnapshotInfo, len(snapshots))
	for i, snap := range snapshots {
		result[i] = ports.TUISnapshotInfo{
			ID:    snap.ID,
			Time:  snap.Time,
			Paths: snap.Paths,
			Tags:  snap.Tags,
		}
	}

	// Reverse so newest is first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// Compile-time check that Service implements ports.TUIService.
var _ ports.TUIService = (*Service)(nil)
