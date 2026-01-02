package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mcdonaldj/codebak/internal/adapters/execgit"
	"github.com/mcdonaldj/codebak/internal/adapters/osfs"
	"github.com/mcdonaldj/codebak/internal/adapters/ziparchiver"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
	"github.com/mcdonaldj/codebak/internal/ports"
)

// BackupResult contains the result of a backup operation.
type BackupResult struct {
	Project   string
	ZipPath   string
	Size      int64
	FileCount int
	GitHead   string
	Skipped   bool
	Reason    string
	Error     error
}

// ZipResult contains results from createZip including any skipped files.
type ZipResult struct {
	FileCount int
	Skipped   []string
}

// Service provides backup operations with injected dependencies.
type Service struct {
	fs       ports.FileSystem
	git      ports.GitClient
	archiver ports.Archiver
}

// NewService creates a new backup service with the given dependencies.
func NewService(fs ports.FileSystem, git ports.GitClient, archiver ports.Archiver) *Service {
	return &Service{
		fs:       fs,
		git:      git,
		archiver: archiver,
	}
}

// NewDefaultService creates a backup service with real production dependencies.
func NewDefaultService() *Service {
	return NewService(
		osfs.New(),
		execgit.New(),
		ziparchiver.New(),
	)
}

// ListProjects returns all directories in the source directory.
func (s *Service) ListProjects(sourceDir string) ([]string, error) {
	entries, err := s.fs.ReadDir(sourceDir)
	if err != nil {
		return nil, err
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			projects = append(projects, entry.Name())
		}
	}
	return projects, nil
}

// GetGitHead returns the current HEAD commit hash for a git repo.
func (s *Service) GetGitHead(projectPath string) string {
	return s.git.GetHead(projectPath)
}

// shortHash returns the first 7 characters of a hash, or the full hash if shorter.
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// HasChanges checks if project has changed since last backup.
func (s *Service) HasChanges(projectPath string, lastBackup *manifest.BackupEntry) (bool, string) {
	// If no previous backup, definitely has changes
	if lastBackup == nil {
		return true, "no previous backup"
	}

	// Check if it's a git repo
	if s.git.IsRepo(projectPath) {
		// It's a git repo - compare HEAD
		currentHead := s.git.GetHead(projectPath)
		if currentHead != "" && currentHead != lastBackup.GitHead {
			return true, fmt.Sprintf("git HEAD changed: %s -> %s", shortHash(lastBackup.GitHead), shortHash(currentHead))
		}
		if currentHead == lastBackup.GitHead {
			return false, "git HEAD unchanged"
		}
	}

	// Fallback: check mtime of any file newer than last backup
	hasNewer := false
	_ = s.fs.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.ModTime().After(lastBackup.CreatedAt) {
			hasNewer = true
			return filepath.SkipAll
		}
		return nil
	})

	if hasNewer {
		return true, "files modified since last backup"
	}
	return false, "no changes detected"
}

// shouldExclude checks if a path should be excluded.
func shouldExclude(path string, excludePatterns []string) bool {
	base := filepath.Base(path)
	for _, pattern := range excludePatterns {
		// Check exact match
		if base == pattern {
			return true
		}
		// Check glob pattern
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// BackupProject creates a zip backup of a single project.
func (s *Service) BackupProject(cfg *config.Config, project string) BackupResult {
	result := BackupResult{Project: project}

	sourceDir, err := config.ExpandPath(cfg.SourceDir)
	if err != nil {
		result.Error = err
		return result
	}
	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		result.Error = err
		return result
	}
	projectPath := filepath.Join(sourceDir, project)

	// Check if project exists
	if _, err := s.fs.Stat(projectPath); err != nil {
		result.Error = fmt.Errorf("project not found: %s", projectPath)
		return result
	}

	// Load manifest
	m, err := manifest.Load(backupDir, project)
	if err != nil {
		result.Error = fmt.Errorf("loading manifest: %w", err)
		return result
	}
	m.Source = projectPath

	// Check for changes
	hasChanges, reason := s.HasChanges(projectPath, m.LatestBackup())
	if !hasChanges {
		result.Skipped = true
		result.Reason = reason
		return result
	}

	// Create backup directory
	projectBackupDir := filepath.Join(backupDir, project)
	if err := s.fs.MkdirAll(projectBackupDir, 0755); err != nil {
		result.Error = fmt.Errorf("creating backup dir: %w", err)
		return result
	}

	// Generate zip filename
	timestamp := time.Now().Format("20060102-150405")
	zipName := fmt.Sprintf("%s.zip", timestamp)
	zipPath := filepath.Join(projectBackupDir, zipName)

	// Create zip file using archiver
	fileCount, err := s.archiver.Create(zipPath, projectPath, cfg.Exclude)
	if err != nil {
		result.Error = fmt.Errorf("creating zip: %w", err)
		return result
	}

	// Get zip file info
	zipInfo, err := s.fs.Stat(zipPath)
	if err != nil {
		result.Error = fmt.Errorf("stat zip: %w", err)
		return result
	}

	// Compute checksum
	checksum, err := manifest.ComputeSHA256(zipPath)
	if err != nil {
		result.Error = fmt.Errorf("computing checksum: %w", err)
		return result
	}

	// Create manifest entry
	entry := manifest.BackupEntry{
		File:      zipName,
		SHA256:    checksum,
		SizeBytes: zipInfo.Size(),
		CreatedAt: time.Now(),
		GitHead:   s.git.GetHead(projectPath),
		FileCount: fileCount,
		Excluded:  cfg.Exclude,
	}

	m.AddBackup(entry)

	// Prune old backups if retention is configured
	if cfg.Retention.KeepLast > 0 {
		_, _ = m.Prune(backupDir, cfg.Retention.KeepLast)
	}

	// Save manifest
	if err := m.Save(backupDir); err != nil {
		result.Error = fmt.Errorf("saving manifest: %w", err)
		return result
	}

	result.ZipPath = zipPath
	result.Size = zipInfo.Size()
	result.FileCount = fileCount
	result.GitHead = entry.GitHead
	result.Reason = reason

	return result
}

// RunBackup backs up all changed projects.
func (s *Service) RunBackup(cfg *config.Config) ([]BackupResult, error) {
	sourceDir, err := config.ExpandPath(cfg.SourceDir)
	if err != nil {
		return nil, err
	}

	projects, err := s.ListProjects(sourceDir)
	if err != nil {
		return nil, err
	}

	var results []BackupResult
	for _, project := range projects {
		result := s.BackupProject(cfg, project)
		results = append(results, result)
	}

	return results, nil
}

// FormatSize formats bytes as human-readable.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ============================================================================
// Backward-compatible package-level functions using default service
// ============================================================================

var defaultService = NewDefaultService()

// ListProjects returns all directories in the source directory.
// Uses the default production dependencies.
func ListProjects(sourceDir string) ([]string, error) {
	return defaultService.ListProjects(sourceDir)
}

// GetGitHead returns the current HEAD commit hash for a git repo.
// Uses the default production dependencies.
func GetGitHead(projectPath string) string {
	return defaultService.GetGitHead(projectPath)
}

// HasChanges checks if project has changed since last backup.
// Uses the default production dependencies.
func HasChanges(projectPath string, lastBackup *manifest.BackupEntry) (bool, string) {
	return defaultService.HasChanges(projectPath, lastBackup)
}

// BackupProject creates a zip backup of a single project.
// Uses the default production dependencies.
func BackupProject(cfg *config.Config, project string) BackupResult {
	return defaultService.BackupProject(cfg, project)
}

// RunBackup backs up all changed projects.
// Uses the default production dependencies.
func RunBackup(cfg *config.Config) ([]BackupResult, error) {
	return defaultService.RunBackup(cfg)
}
