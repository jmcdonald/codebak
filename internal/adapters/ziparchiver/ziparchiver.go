// Package ziparchiver provides an archiver adapter using the archive/zip package.
package ziparchiver

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/mcdonaldj/codebak/internal/ports"
)

// ZipArchiver implements ports.Archiver using archive/zip.
type ZipArchiver struct{}

// New creates a new ZipArchiver adapter.
func New() *ZipArchiver {
	return &ZipArchiver{}
}

// shouldExclude checks if a path should be excluded based on patterns.
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

// Create creates a zip archive of sourceDir at destPath.
// Returns the number of files archived.
// exclude is a list of patterns to skip (e.g., "node_modules", "*.pyc").
func (a *ZipArchiver) Create(destPath, sourceDir string, exclude []string) (int, error) {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return 0, err
	}

	w := zip.NewWriter(zipFile)
	fileCount := 0
	baseName := filepath.Base(sourceDir)

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
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
			return nil
		}
		header.Name = archivePath
		header.Method = zip.Deflate

		writer, err := w.CreateHeader(header)
		if err != nil {
			return nil
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return nil
		}

		_, copyErr := io.Copy(writer, file)
		_ = file.Close() // Explicitly ignore close error - data already copied

		if copyErr != nil {
			return nil
		}

		fileCount++
		return nil
	})

	// Close zip writer first to flush data
	if closeErr := w.Close(); closeErr != nil {
		_ = zipFile.Close() // Best effort cleanup on error path
		return 0, fmt.Errorf("closing zip writer: %w", closeErr)
	}

	// Then close the file
	if closeErr := zipFile.Close(); closeErr != nil {
		return 0, fmt.Errorf("closing zip file: %w", closeErr)
	}

	return fileCount, walkErr
}

// Extract extracts a zip archive to destDir.
func (a *ZipArchiver) Extract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

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

		// SECURITY: Check for ZipSlip vulnerability
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

		// Extract file
		if err := extractFile(f, fpath); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}

	return nil
}

// MaxDecompressSize is the maximum allowed uncompressed file size (10GB).
// This prevents decompression bomb attacks (G110).
const MaxDecompressSize = 10 * 1024 * 1024 * 1024 // 10GB

// extractFile extracts a single file from the zip.
func extractFile(f *zip.File, destPath string) error {
	// SECURITY: Limit decompression size to prevent zip bombs (G110)
	declaredSize := f.UncompressedSize64
	if declaredSize > MaxDecompressSize {
		return fmt.Errorf("file too large: %d bytes exceeds limit of %d bytes", declaredSize, MaxDecompressSize)
	}

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	// Use LimitReader to enforce size limit during decompression
	// Add 1 byte to detect if actual size exceeds declared size
	limitedReader := io.LimitReader(rc, int64(declaredSize)+1)
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return err
	}

	// Check if more data was available than declared (corrupted/malicious zip)
	if written > int64(declaredSize) {
		return fmt.Errorf("decompressed size exceeds declared size")
	}

	return nil
}

// isWithinDir checks if the target path is within the base directory.
func isWithinDir(absBaseDir, targetPath string) bool {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	absTarget = filepath.Clean(absTarget)

	return strings.HasPrefix(absTarget, absBaseDir+string(filepath.Separator)) ||
		absTarget == absBaseDir
}

// List returns a map of file paths to their info from the archive.
// The path key has the project prefix stripped.
func (a *ZipArchiver) List(zipPath string) (map[string]ports.FileInfo, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	files := make(map[string]ports.FileInfo)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Strip project prefix (first path component)
		name := f.Name
		if idx := strings.Index(name, "/"); idx != -1 {
			name = name[idx+1:]
		}

		// Safe conversion: check for overflow before uint64 -> int64
		size := int64(0)
		if f.UncompressedSize64 <= math.MaxInt64 {
			size = int64(f.UncompressedSize64)
		}
		files[name] = ports.FileInfo{
			Size:  size,
			CRC32: f.CRC32,
		}
	}

	return files, nil
}

// ReadFile reads the contents of a file from inside a zip archive.
func (a *ZipArchiver) ReadFile(zipPath, filePath, projectName string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	// Look for the file with project prefix
	targetPath := filepath.Join(projectName, filePath)

	for _, f := range r.File {
		if f.Name == targetPath {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer func() { _ = rc.Close() }()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("file not found in archive: %s", filePath)
}

// Compile-time check that ZipArchiver implements ports.Archiver.
var _ ports.Archiver = (*ZipArchiver)(nil)
