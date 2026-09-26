package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmcdonald/codebak/internal/adapters/execgit"
	"github.com/jmcdonald/codebak/internal/adapters/execrestic"
	"github.com/jmcdonald/codebak/internal/adapters/osfs"
	"github.com/jmcdonald/codebak/internal/adapters/ziparchiver"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/manifest"
	"github.com/jmcdonald/codebak/internal/ports"
)

// BackupResult contains the result of a backup operation.
type BackupResult struct {
	Project    string
	ZipPath    string
	SnapshotID string // For restic backups
	Size       int64
	FileCount  int
	GitHead    string
	Skipped    bool
	Reason     string
	Error      error
	SourceType config.SourceType // git or sensitive
	// Pruned lists backup files deleted by retention (count-based or
	// total-size-based). When DryRun is true, these were NOT deleted; they
	// are what a real run would delete.
	Pruned []string
	// DryRun mirrors config.Config.DryRun for this result: true means no
	// files were written or deleted, everything above is a preview.
	DryRun bool
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
	restic   ports.ResticClient
}

// NewService creates a new backup service with the given dependencies.
func NewService(fs ports.FileSystem, git ports.GitClient, archiver ports.Archiver, restic ports.ResticClient) *Service {
	return &Service{
		fs:       fs,
		git:      git,
		archiver: archiver,
		restic:   restic,
	}
}

// NewDefaultService creates a backup service with real production dependencies.
func NewDefaultService() *Service {
	return NewService(
		osfs.New(),
		execgit.New(),
		ziparchiver.New(),
		execrestic.New(),
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

// isExcludedProject reports whether project is in the excluded_projects list.
func isExcludedProject(project string, excludedProjects []string) bool {
	for _, excluded := range excludedProjects {
		if excluded == project {
			return true
		}
	}
	return false
}

// minIntervalReason returns a non-empty skip reason if last is recent enough
// that minIntervalHours has not yet elapsed. Returns "" if the
// project should proceed.
func minIntervalReason(minIntervalHours float64, last *manifest.BackupEntry) string {
	if minIntervalHours <= 0 || last == nil {
		return ""
	}
	minInterval := time.Duration(minIntervalHours * float64(time.Hour))
	elapsed := time.Since(last.CreatedAt)
	if elapsed < minInterval {
		return fmt.Sprintf("min-interval (%gh, last run %s ago)", minIntervalHours, elapsed.Round(time.Second))
	}
	return ""
}

// dirSize sums the size in bytes of every regular file under root.
func dirSize(fsys ports.FileSystem, root string) int64 {
	var total int64
	_ = fsys.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// diskSpaceReason returns a non-empty skip reason if the backup destination
// does not have at least 2x sourceSize bytes free. Returns "" if
// there is enough room, or if free space could not be determined (fails
// open: a precheck we can't evaluate should not block a backup).
func diskSpaceReason(fsys ports.FileSystem, backupDir string, sourceSize int64) string {
	free, err := fsys.FreeSpace(backupDir)
	if err != nil {
		return ""
	}
	needed := uint64(2 * sourceSize) // #nosec G115 -- sourceSize is a non-negative file-size sum
	if free < needed {
		return fmt.Sprintf("disk-space (%s free, need %s)", FormatSize(int64(free)), FormatSize(int64(needed)))
	}
	return ""
}

// BackupProject creates a zip backup of a single project.
func (s *Service) BackupProject(cfg *config.Config, project string) BackupResult {
	result := BackupResult{Project: project, DryRun: cfg.DryRun}

	// excluded_projects skips the project entirely, before anything else
	// is touched.
	if isExcludedProject(project, cfg.ExcludedProjects) {
		result.Skipped = true
		result.Reason = "excluded"
		return result
	}

	backupDir, err := config.ExpandPath(cfg.BackupDir)
	if err != nil {
		result.Error = err
		return result
	}

	// Search for project in all sources
	var projectPath string
	for _, source := range cfg.GetSources() {
		sourceDir, err := config.ExpandPath(source.Path)
		if err != nil {
			continue
		}
		candidatePath := filepath.Join(sourceDir, project)
		if _, err := s.fs.Stat(candidatePath); err == nil {
			projectPath = candidatePath
			break
		}
	}

	// Check if project was found
	if projectPath == "" {
		result.Error = fmt.Errorf("project not found: %s", project)
		return result
	}

	// Load manifest
	m, err := manifest.Load(backupDir, project)
	if err != nil {
		result.Error = fmt.Errorf("loading manifest: %w", err)
		return result
	}
	m.Source = projectPath

	// min_interval_hours debounces repeated runs.
	if reason := minIntervalReason(cfg.MinIntervalHours, m.LatestBackup()); reason != "" {
		result.Skipped = true
		result.Reason = reason
		return result
	}

	// Check for changes
	hasChanges, reason := s.HasChanges(projectPath, m.LatestBackup())
	if !hasChanges {
		result.Skipped = true
		result.Reason = reason
		return result
	}

	// Disk-space precheck. Evaluated before opening any output file so a
	// near-full disk never gets a partial zip.
	sourceSize := dirSize(s.fs, projectPath)
	if reason := diskSpaceReason(s.fs, backupDir, sourceSize); reason != "" {
		result.Skipped = true
		result.Reason = reason
		return result
	}

	// Generate zip filename
	timestamp := time.Now().Format("20060102-150405")
	zipName := fmt.Sprintf("%s.zip", timestamp)
	projectBackupDir := filepath.Join(backupDir, project)
	zipPath := filepath.Join(projectBackupDir, zipName)

	var entry manifest.BackupEntry

	if cfg.DryRun {
		// Dry run: no zip is written, no manifest is saved. Report what
		// would happen using the on-disk source size as an estimate.
		result.ZipPath = zipPath
		result.Size = sourceSize
		result.GitHead = s.git.GetHead(projectPath)
		result.Reason = "(dry-run) would back up: " + reason
		entry = manifest.BackupEntry{
			File:      zipName,
			SizeBytes: sourceSize,
			CreatedAt: time.Now(),
			GitHead:   result.GitHead,
		}
	} else {
		// Create backup directory
		if err := s.fs.MkdirAll(projectBackupDir, 0755); err != nil {
			result.Error = fmt.Errorf("creating backup dir: %w", err)
			return result
		}

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

		entry = manifest.BackupEntry{
			File:      zipName,
			SHA256:    checksum,
			SizeBytes: zipInfo.Size(),
			CreatedAt: time.Now(),
			GitHead:   s.git.GetHead(projectPath),
			FileCount: fileCount,
			Excluded:  cfg.Exclude,
		}

		result.ZipPath = zipPath
		result.Size = zipInfo.Size()
		result.FileCount = fileCount
		result.GitHead = entry.GitHead
		result.Reason = reason
	}

	m.AddBackup(entry)

	// Count-based and total-size-based retention, whichever is stricter
	// wins. Both log exactly what they delete (or would delete, in
	// dry-run) via the returned file lists on BackupResult.
	keepLast := cfg.EffectiveKeepLast(project)
	if keepLast > 0 {
		prunedByCount, _ := m.PruneWithOptions(backupDir, keepLast, cfg.DryRun)
		result.Pruned = append(result.Pruned, prunedByCount...)
	}
	if maxBytes := cfg.MaxTotalBytesPerProject(); maxBytes > 0 {
		prunedBySize, _ := m.PruneByTotalSize(backupDir, maxBytes, cfg.DryRun)
		result.Pruned = append(result.Pruned, prunedBySize...)
	}

	if cfg.DryRun {
		// Nothing was written; do not persist the hypothetical manifest.
		return result
	}

	// Save manifest
	if err := m.Save(backupDir); err != nil {
		result.Error = fmt.Errorf("saving manifest: %w", err)
		return result
	}

	return result
}

// BackupSensitiveSource backs up a sensitive source using restic.
func (s *Service) BackupSensitiveSource(cfg *config.Config, source config.Source) BackupResult {
	result := BackupResult{
		Project:    source.Label,
		SourceType: config.SourceTypeSensitive,
	}

	// Use label or path basename as name
	if result.Project == "" {
		result.Project = filepath.Base(source.Path)
	}

	// Get restic repo path
	repoPath, err := cfg.GetResticRepoPath()
	if err != nil {
		result.Error = fmt.Errorf("getting restic repo path: %w", err)
		return result
	}

	// Get restic password
	password, err := cfg.GetResticPassword()
	if err != nil {
		result.Error = err
		return result
	}

	// Expand source path
	sourcePath, err := config.ExpandPath(source.Path)
	if err != nil {
		result.Error = fmt.Errorf("expanding source path: %w", err)
		return result
	}

	// Check if source exists
	if _, err := s.fs.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			result.Skipped = true
			result.Reason = "source path does not exist"
			return result
		}
		result.Error = fmt.Errorf("checking source path: %w", err)
		return result
	}

	// Initialize repo if needed
	if !s.restic.IsInitialized(repoPath) {
		// Create repo directory
		if err := s.fs.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
			result.Error = fmt.Errorf("creating restic repo directory: %w", err)
			return result
		}
		if err := s.restic.Init(repoPath, password); err != nil {
			result.Error = fmt.Errorf("initializing restic repo: %w", err)
			return result
		}
	}

	// Create backup with source path as tag for identification
	tag := filepath.Base(sourcePath)
	snapshotID, err := s.restic.Backup(repoPath, password, []string{sourcePath}, []string{tag})
	if err != nil {
		result.Error = fmt.Errorf("restic backup failed: %w", err)
		return result
	}

	result.SnapshotID = snapshotID
	result.Reason = "restic backup created"

	// Apply retention policy
	if cfg.Retention.KeepLast > 0 {
		_ = s.restic.Forget(repoPath, password, cfg.Retention.KeepLast, false)
	}

	return result
}

// RunBackup backs up all changed projects from all configured sources.
func (s *Service) RunBackup(cfg *config.Config) ([]BackupResult, error) {
	var results []BackupResult
	seen := make(map[string]bool) // Track project names to avoid duplicates

	// Iterate over all sources
	for _, source := range cfg.GetSources() {
		// Branch on source type
		if source.Type == config.SourceTypeSensitive {
			// Use source path as the unique identifier for sensitive sources
			sourcePath, err := config.ExpandPath(source.Path)
			if err != nil {
				continue
			}
			if seen[sourcePath] {
				continue
			}
			seen[sourcePath] = true
			result := s.BackupSensitiveSource(cfg, source)
			results = append(results, result)
			continue
		}

		// Git source: existing behavior
		sourceDir, err := config.ExpandPath(source.Path)
		if err != nil {
			continue // Skip sources that can't be expanded
		}

		projects, err := s.ListProjects(sourceDir)
		if err != nil {
			continue // Skip sources that can't be read
		}

		for _, project := range projects {
			if seen[project] {
				continue
			}
			seen[project] = true
			result := s.BackupProject(cfg, project)
			result.SourceType = config.SourceTypeGit
			results = append(results, result)
		}
	}

	// Also backup individual projects from cfg.Projects
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
		result := s.BackupProject(cfg, name)
		result.SourceType = config.SourceTypeGit
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
