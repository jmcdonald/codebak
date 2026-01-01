package recovery

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
)

type RecoverOptions struct {
	Project string
	Version string // YYYYMMDD-HHMMSS format, empty for latest
	Wipe    bool   // Delete current before restore
	Archive bool   // Archive current before restore
}

// Verify checks the integrity of a backup by comparing checksums
func Verify(cfg *config.Config, project, version string) error {
	backupDir := config.ExpandPath(cfg.BackupDir)

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

// Recover restores a project from backup
func Recover(cfg *config.Config, opts RecoverOptions) error {
	sourceDir := config.ExpandPath(cfg.SourceDir)
	backupDir := config.ExpandPath(cfg.BackupDir)
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
	if err := Verify(cfg, opts.Project, opts.Version); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Handle existing project directory
	if _, err := os.Stat(projectPath); err == nil {
		if opts.Wipe {
			// Delete current
			if err := os.RemoveAll(projectPath); err != nil {
				return fmt.Errorf("removing current project: %w", err)
			}
		} else if opts.Archive {
			// Archive current first
			archiveName := fmt.Sprintf("%s-archived-%s", opts.Project, time.Now().Format("20060102-150405"))
			archivePath := filepath.Join(sourceDir, archiveName)
			if err := os.Rename(projectPath, archivePath); err != nil {
				return fmt.Errorf("archiving current project: %w", err)
			}
		} else {
			return fmt.Errorf("project already exists: %s (use --wipe or --archive)", projectPath)
		}
	}

	// Extract zip
	if err := extractZip(zipPath, sourceDir); err != nil {
		return fmt.Errorf("extracting backup: %w", err)
	}

	return nil
}

// extractZip extracts a zip file to the destination directory
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Get cleaned absolute path for destination
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolving destination path: %w", err)
	}
	absDestDir = filepath.Clean(absDestDir)

	for _, f := range r.File {
		// SECURITY: Block symlinks to prevent symlink attacks
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not supported in backups: %s", f.Name)
		}

		fpath := filepath.Join(destDir, f.Name)

		// SECURITY: Check for ZipSlip vulnerability using cleaned absolute paths
		if !isWithinDir(absDestDir, fpath) {
			return fmt.Errorf("invalid file path (path traversal detected): %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return fmt.Errorf("creating directory %s: %w", fpath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", fpath, err)
		}

		// Create file
		if err := extractFile(f, fpath); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}

	return nil
}

// extractFile extracts a single file from the zip
func extractFile(f *zip.File, destPath string) error {
	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// isWithinDir checks if the target path is within the base directory
// SECURITY: Uses cleaned absolute paths to prevent path traversal attacks
func isWithinDir(absBaseDir, targetPath string) bool {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	absTarget = filepath.Clean(absTarget)

	// Ensure target is within base directory
	// Add separator to prevent matching partial directory names
	// e.g., /home/user vs /home/username
	return strings.HasPrefix(absTarget, absBaseDir+string(filepath.Separator)) ||
		absTarget == absBaseDir
}

// ListVersions returns all backup versions for a project
func ListVersions(cfg *config.Config, project string) ([]manifest.BackupEntry, error) {
	backupDir := config.ExpandPath(cfg.BackupDir)

	m, err := manifest.Load(backupDir, project)
	if err != nil {
		return nil, err
	}

	return m.Backups, nil
}
