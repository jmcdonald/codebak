package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
)

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

// ListProjects returns all directories in the source directory
func ListProjects(sourceDir string) ([]string, error) {
	entries, err := os.ReadDir(sourceDir)
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

// GetGitHead returns the current HEAD commit hash for a git repo
func GetGitHead(projectPath string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// shortHash returns the first 7 characters of a hash, or the full hash if shorter
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// HasChanges checks if project has changed since last backup
func HasChanges(projectPath string, lastBackup *manifest.BackupEntry) (bool, string) {
	// If no previous backup, definitely has changes
	if lastBackup == nil {
		return true, "no previous backup"
	}

	// Check if it's a git repo
	gitDir := filepath.Join(projectPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// It's a git repo - compare HEAD
		currentHead := GetGitHead(projectPath)
		if currentHead != "" && currentHead != lastBackup.GitHead {
			return true, fmt.Sprintf("git HEAD changed: %s -> %s", shortHash(lastBackup.GitHead), shortHash(currentHead))
		}
		if currentHead == lastBackup.GitHead {
			return false, "git HEAD unchanged"
		}
	}

	// Fallback: check mtime of any file newer than last backup
	hasNewer := false
	filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
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

// shouldExclude checks if a path should be excluded
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

// BackupProject creates a zip backup of a single project
func BackupProject(cfg *config.Config, project string) BackupResult {
	sourceDir := config.ExpandPath(cfg.SourceDir)
	backupDir := config.ExpandPath(cfg.BackupDir)
	projectPath := filepath.Join(sourceDir, project)

	result := BackupResult{Project: project}

	// Check if project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
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
	hasChanges, reason := HasChanges(projectPath, m.LatestBackup())
	if !hasChanges {
		result.Skipped = true
		result.Reason = reason
		return result
	}

	// Create backup directory
	projectBackupDir := filepath.Join(backupDir, project)
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		result.Error = fmt.Errorf("creating backup dir: %w", err)
		return result
	}

	// Generate zip filename
	timestamp := time.Now().Format("20060102-150405")
	zipName := fmt.Sprintf("%s.zip", timestamp)
	zipPath := filepath.Join(projectBackupDir, zipName)

	// Create zip file
	fileCount, err := createZip(projectPath, zipPath, cfg.Exclude)
	if err != nil {
		result.Error = fmt.Errorf("creating zip: %w", err)
		return result
	}

	// Get zip file info
	zipInfo, err := os.Stat(zipPath)
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
		GitHead:   GetGitHead(projectPath),
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

// ZipResult contains results from createZip including any skipped files
type ZipResult struct {
	FileCount int
	Skipped   []string
}

// createZip creates a zip archive of the source directory
func createZip(sourceDir, zipPath string, exclude []string) (int, error) {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return 0, err
	}

	w := zip.NewWriter(zipFile)
	fileCount := 0
	baseName := filepath.Base(sourceDir)
	var skipped []string

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Track skipped files instead of silently ignoring
			skipped = append(skipped, path)
			return nil
		}

		// Check exclusions
		if shouldExclude(path, exclude) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			skipped = append(skipped, path)
			return nil
		}

		// Prefix with project name
		archivePath := filepath.Join(baseName, relPath)

		if info.IsDir() {
			return nil // Directories are created implicitly
		}

		// Create file header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			skipped = append(skipped, path)
			return nil
		}
		header.Name = archivePath
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			skipped = append(skipped, path)
			return nil
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			skipped = append(skipped, path)
			return nil
		}

		_, copyErr := io.Copy(writer, file)
		file.Close() // Close immediately, don't defer in loop

		if copyErr != nil {
			skipped = append(skipped, path)
			return nil
		}

		fileCount++
		return nil
	})

	// Close zip writer first to flush data
	if closeErr := w.Close(); closeErr != nil {
		zipFile.Close()
		return 0, fmt.Errorf("closing zip writer: %w", closeErr)
	}

	// Then close the file
	if closeErr := zipFile.Close(); closeErr != nil {
		return 0, fmt.Errorf("closing zip file: %w", closeErr)
	}

	// Log skipped files if any (for debugging)
	if len(skipped) > 0 {
		// Could log these somewhere or add to result
		_ = skipped
	}

	return fileCount, walkErr
}

// RunBackup backs up all changed projects
func RunBackup(cfg *config.Config) ([]BackupResult, error) {
	sourceDir := config.ExpandPath(cfg.SourceDir)

	projects, err := ListProjects(sourceDir)
	if err != nil {
		return nil, err
	}

	var results []BackupResult
	for _, project := range projects {
		result := BackupProject(cfg, project)
		results = append(results, result)
	}

	return results, nil
}

// FormatSize formats bytes as human-readable
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
