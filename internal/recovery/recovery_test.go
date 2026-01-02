package recovery

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcdonaldj/codebak/internal/adapters/ziparchiver"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/ports"
)

// isWithinDir checks if the target path is within the base directory.
// This is a test helper that mirrors the security check in ziparchiver.
func isWithinDir(absBaseDir, targetPath string) bool {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	absTarget = filepath.Clean(absTarget)

	return strings.HasPrefix(absTarget, absBaseDir+string(filepath.Separator)) ||
		absTarget == absBaseDir
}

// extractFile extracts a single file from the zip.
// This is a test helper that mirrors the extraction logic.
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

func TestIsWithinDir(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		target   string
		expected bool
	}{
		{
			name:     "valid path within directory",
			baseDir:  "/home/user/dest",
			target:   "/home/user/dest/subdir/file.txt",
			expected: true,
		},
		{
			name:     "exact match",
			baseDir:  "/home/user/dest",
			target:   "/home/user/dest",
			expected: true,
		},
		{
			name:     "parent directory traversal blocked",
			baseDir:  "/home/user/dest",
			target:   "/home/user/dest/../../../etc/passwd",
			expected: false,
		},
		{
			name:     "sibling directory blocked",
			baseDir:  "/home/user/dest",
			target:   "/home/user/other/file.txt",
			expected: false,
		},
		{
			name:     "prefix match but different directory (ZipSlip variant)",
			baseDir:  "/home/user",
			target:   "/home/username/evil.txt",
			expected: false,
		},
		{
			name:     "double dot in filename allowed",
			baseDir:  "/home/user/dest",
			target:   "/home/user/dest/file..txt",
			expected: true,
		},
		{
			name:     "absolute path outside base",
			baseDir:  "/home/user/dest",
			target:   "/tmp/evil.txt",
			expected: false,
		},
		{
			name:     "hidden directory within base",
			baseDir:  "/home/user/dest",
			target:   "/home/user/dest/.hidden/file.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use absolute paths for consistent testing
			absBase, _ := filepath.Abs(tt.baseDir)
			absBase = filepath.Clean(absBase)

			result := isWithinDir(absBase, tt.target)
			if result != tt.expected {
				t.Errorf("isWithinDir(%q, %q) = %v, expected %v", absBase, tt.target, result, tt.expected)
			}
		})
	}
}

func TestExtractZipZipSlipPrevention(t *testing.T) {
	// Create a temp directory for the test
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a malicious zip file with path traversal
	maliciousZipPath := filepath.Join(tempDir, "malicious.zip")
	createMaliciousZip(t, maliciousZipPath)

	// Try to extract - should fail with path traversal error
	destDir := filepath.Join(tempDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	archiver := ziparchiver.New()
	err = archiver.Extract(maliciousZipPath, destDir)
	if err == nil {
		t.Error("Extract should have rejected malicious zip with path traversal")
	}

	// Verify the malicious file was NOT created
	evilPath := filepath.Join(tempDir, "evil.txt")
	if _, err := os.Stat(evilPath); err == nil {
		t.Error("Malicious file was created outside destination - ZipSlip vulnerability!")
	}
}

func TestExtractZipValidArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid zip file
	zipPath := filepath.Join(tempDir, "valid.zip")
	createValidZip(t, zipPath)

	// Extract it
	destDir := filepath.Join(tempDir, "dest")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	archiver := ziparchiver.New()
	err = archiver.Extract(zipPath, destDir)
	if err != nil {
		t.Fatalf("Extract failed for valid archive: %v", err)
	}

	// Verify files were extracted
	extractedFile := filepath.Join(destDir, "test-project", "file.txt")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Extracted content = %q, expected %q", string(content), "test content")
	}
}

// createMaliciousZip creates a zip with a path traversal attack
func createMaliciousZip(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	// Add a file with path traversal
	fw, err := w.Create("../evil.txt")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	fw.Write([]byte("malicious content"))

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}
}

// createValidZip creates a normal zip archive
func createValidZip(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)

	// Add a valid file (using test-project as folder name to match manifest)
	fw, err := w.Create("test-project/file.txt")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	fw.Write([]byte("test content"))

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}
}

func TestVerifySuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	err = Verify(cfg, "test-project", "")
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}

func TestVerifyNoBackups(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		BackupDir: tempDir,
	}

	err = Verify(cfg, "nonexistent", "")
	if err == nil {
		t.Error("Verify should fail for non-existent project")
	}
}

func TestListVersions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	cfg := &config.Config{
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	versions, err := ListVersions(cfg, "test-project")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("ListVersions returned %d versions, expected 1", len(versions))
	}
}

func TestListVersionsNoBackups(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		BackupDir: tempDir,
	}

	versions, err := ListVersions(cfg, "nonexistent")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("ListVersions should return empty for non-existent project, got %d", len(versions))
	}
}

func TestRecoverWithWipe(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	projectPath := filepath.Join(sourceDir, "test-project")

	// Create existing project
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "existing.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
		Wipe:    true,
	}

	err = Recover(cfg, opts)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Check that restored file exists
	restoredFile := filepath.Join(projectPath, "file.txt")
	if _, err := os.Stat(restoredFile); os.IsNotExist(err) {
		t.Error("Restored file not found")
	}

	// Check that old file is gone
	if _, err := os.Stat(filepath.Join(projectPath, "existing.txt")); err == nil {
		t.Error("Existing file should have been wiped")
	}
}

func TestRecoverWithArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	projectPath := filepath.Join(sourceDir, "test-project")

	// Create existing project
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "existing.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
		Archive: true,
	}

	err = Recover(cfg, opts)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Check that archived project exists
	entries, _ := os.ReadDir(sourceDir)
	hasArchive := false
	for _, e := range entries {
		if len(e.Name()) > len("test-project-archived-") && e.Name()[:len("test-project-archived-")] == "test-project-archived-" {
			hasArchive = true
			break
		}
	}
	if !hasArchive {
		t.Error("Archived project not found")
	}
}

func TestRecoverExistingNoOption(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	projectPath := filepath.Join(sourceDir, "test-project")

	// Create existing project
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
		// No Wipe or Archive
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail when project exists without --wipe or --archive")
	}
}

func TestRecoverToNewLocation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	projectPath := filepath.Join(sourceDir, "test-project")

	// Ensure project doesn't exist
	os.RemoveAll(projectPath)

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
	}

	err = Recover(cfg, opts)
	if err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	// Check that restored file exists
	restoredFile := filepath.Join(projectPath, "file.txt")
	content, err := os.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Restored content = %q, expected %q", string(content), "test content")
	}
}

// setupTestBackup creates a test backup with manifest
func setupTestBackup(t *testing.T, tempDir string) {
	t.Helper()

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")

	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create a valid zip
	zipPath := filepath.Join(projectBackupDir, "20260101-120000.zip")
	createValidZip(t, zipPath)

	// Create manifest with correct checksum
	checksum := computeTestChecksum(t, zipPath)
	manifestContent := fmt.Sprintf(`{
		"project": "test-project",
		"source": "%s",
		"backups": [{
			"file": "20260101-120000.zip",
			"sha256": "%s",
			"size_bytes": 100,
			"created_at": "2026-01-01T12:00:00Z",
			"git_head": "abc123",
			"file_count": 1,
			"excluded": []
		}]
	}`, filepath.Join(tempDir, "source", "test-project"), checksum)

	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}
}

func computeTestChecksum(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file for checksum: %v", err)
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// ============================================================================
// Additional tests for coverage improvement
// ============================================================================

func TestVerifySpecificVersion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	// Test with specific version (without .zip extension)
	err = Verify(cfg, "test-project", "20260101-120000")
	if err != nil {
		t.Errorf("Verify with specific version failed: %v", err)
	}
}

func TestVerifyVersionNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	// Test with non-existent version
	err = Verify(cfg, "test-project", "19990101-000000")
	if err == nil {
		t.Error("Verify should fail for non-existent version")
	}
	if !strings.Contains(err.Error(), "backup not found") {
		t.Errorf("Expected 'backup not found' error, got: %v", err)
	}
}

func TestVerifyChecksumMismatch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create a zip
	zipPath := filepath.Join(projectBackupDir, "20260101-120000.zip")
	createValidZip(t, zipPath)

	// Create manifest with WRONG checksum
	manifestContent := `{
		"project": "test-project",
		"source": "/some/path",
		"backups": [{
			"file": "20260101-120000.zip",
			"sha256": "wrong_checksum_that_will_never_match",
			"size_bytes": 100,
			"created_at": "2026-01-01T12:00:00Z",
			"git_head": "abc123",
			"file_count": 1,
			"excluded": []
		}]
	}`

	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	cfg := &config.Config{
		BackupDir: backupDir,
	}

	err = Verify(cfg, "test-project", "")
	if err == nil {
		t.Error("Verify should fail for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Expected 'checksum mismatch' error, got: %v", err)
	}
}

func TestRecoverSpecificVersion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	projectPath := filepath.Join(sourceDir, "test-project")

	// Ensure project doesn't exist
	os.RemoveAll(projectPath)

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
		Version: "20260101-120000", // Specific version
	}

	err = Recover(cfg, opts)
	if err != nil {
		t.Fatalf("Recover with specific version failed: %v", err)
	}

	// Check that restored file exists
	restoredFile := filepath.Join(projectPath, "file.txt")
	if _, err := os.Stat(restoredFile); os.IsNotExist(err) {
		t.Error("Restored file not found")
	}
}

func TestRecoverVersionNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup test backup
	setupTestBackup(t, tempDir)

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	opts := RecoverOptions{
		Project: "test-project",
		Version: "19990101-000000", // Non-existent version
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail for non-existent version")
	}
	if !strings.Contains(err.Error(), "backup version not found") {
		t.Errorf("Expected 'backup version not found' error, got: %v", err)
	}
}

func TestRecoverNoBackupsForProject(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: tempDir,
	}

	opts := RecoverOptions{
		Project: "nonexistent-project",
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail for project with no backups")
	}
	if !strings.Contains(err.Error(), "no backups found") {
		t.Errorf("Expected 'no backups found' error, got: %v", err)
	}
}

func TestServiceNewServiceAndNewDefaultService(t *testing.T) {
	// Test NewDefaultService
	svc := NewDefaultService()
	if svc == nil {
		t.Fatal("NewDefaultService returned nil")
	}
	if svc.fs == nil {
		t.Error("NewDefaultService should set filesystem")
	}
	if svc.archiver == nil {
		t.Error("NewDefaultService should set archiver")
	}

	// Test NewService with mocks
	mockFS := &mockTestFS{}
	mockArch := &mockTestArchiver{}
	svc2 := NewService(mockFS, mockArch)
	if svc2 == nil {
		t.Fatal("NewService returned nil")
	}
	// Verify it was created (cannot compare interface to concrete type directly)
	if svc2.fs == nil {
		t.Error("NewService should use provided filesystem")
	}
	if svc2.archiver == nil {
		t.Error("NewService should use provided archiver")
	}
}

// mockTestFS is a minimal mock for testing
type mockTestFS struct{}

func (m *mockTestFS) ReadDir(name string) ([]os.DirEntry, error)                 { return nil, nil }
func (m *mockTestFS) Stat(name string) (os.FileInfo, error)                      { return nil, nil }
func (m *mockTestFS) MkdirAll(path string, perm os.FileMode) error               { return nil }
func (m *mockTestFS) WriteFile(name string, data []byte, perm os.FileMode) error { return nil }
func (m *mockTestFS) ReadFile(name string) ([]byte, error)                       { return nil, nil }
func (m *mockTestFS) Remove(name string) error                                   { return nil }
func (m *mockTestFS) RemoveAll(path string) error                                { return nil }
func (m *mockTestFS) Rename(oldpath, newpath string) error                       { return nil }
func (m *mockTestFS) Open(name string) (fs.File, error)                          { return nil, nil }
func (m *mockTestFS) Create(name string) (*os.File, error)                       { return nil, nil }
func (m *mockTestFS) Walk(root string, fn ports.WalkFunc) error                  { return nil }

// mockTestArchiver is a minimal mock for testing
type mockTestArchiver struct{}

func (m *mockTestArchiver) Create(destPath, sourceDir string, exclude []string) (int, error) {
	return 0, nil
}
func (m *mockTestArchiver) Extract(zipPath, destDir string) error { return nil }
func (m *mockTestArchiver) List(zipPath string) (map[string]ports.FileInfo, error) {
	return nil, nil
}
func (m *mockTestArchiver) ReadFile(zipPath, filePath, projectName string) (string, error) {
	return "", nil
}

func TestListVersionsWithLoadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a project dir with a malformed manifest
	projectBackupDir := filepath.Join(tempDir, "bad-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Write malformed manifest
	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	cfg := &config.Config{
		BackupDir: tempDir,
	}

	_, err = ListVersions(cfg, "bad-project")
	if err == nil {
		t.Error("ListVersions should fail for malformed manifest")
	}
}

func TestVerifyComputeChecksumError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create manifest pointing to a non-existent zip file
	manifestContent := `{
		"project": "test-project",
		"source": "/some/path",
		"backups": [{
			"file": "missing.zip",
			"sha256": "abc123",
			"size_bytes": 100,
			"created_at": "2026-01-01T12:00:00Z",
			"git_head": "abc123",
			"file_count": 1,
			"excluded": []
		}]
	}`

	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	cfg := &config.Config{
		BackupDir: backupDir,
	}

	err = Verify(cfg, "test-project", "")
	if err == nil {
		t.Error("Verify should fail when zip file doesn't exist")
	}
	if !strings.Contains(err.Error(), "computing checksum") {
		t.Errorf("Expected 'computing checksum' error, got: %v", err)
	}
}

func TestVerifyManifestLoadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "bad-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Write malformed manifest
	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	cfg := &config.Config{
		BackupDir: backupDir,
	}

	err = Verify(cfg, "bad-project", "")
	if err == nil {
		t.Error("Verify should fail for malformed manifest")
	}
	if !strings.Contains(err.Error(), "loading manifest") {
		t.Errorf("Expected 'loading manifest' error, got: %v", err)
	}
}

func TestRecoverManifestLoadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "bad-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Write malformed manifest
	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	cfg := &config.Config{
		SourceDir: filepath.Join(tempDir, "source"),
		BackupDir: backupDir,
	}

	opts := RecoverOptions{
		Project: "bad-project",
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail for malformed manifest")
	}
	if !strings.Contains(err.Error(), "loading manifest") {
		t.Errorf("Expected 'loading manifest' error, got: %v", err)
	}
}

func TestRecoverVerificationFails(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create a valid zip
	zipPath := filepath.Join(projectBackupDir, "20260101-120000.zip")
	createValidZip(t, zipPath)

	// Create manifest with WRONG checksum so verification fails
	manifestContent := `{
		"project": "test-project",
		"source": "/some/path",
		"backups": [{
			"file": "20260101-120000.zip",
			"sha256": "wrong_checksum",
			"size_bytes": 100,
			"created_at": "2026-01-01T12:00:00Z",
			"git_head": "abc123",
			"file_count": 1,
			"excluded": []
		}]
	}`

	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	sourceDir := filepath.Join(tempDir, "source")

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	opts := RecoverOptions{
		Project: "test-project",
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail when verification fails")
	}
	if !strings.Contains(err.Error(), "verification failed") {
		t.Errorf("Expected 'verification failed' error, got: %v", err)
	}
}

func TestRecoverExtractError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create a corrupt/empty "zip" file
	zipPath := filepath.Join(projectBackupDir, "20260101-120000.zip")
	if err := os.WriteFile(zipPath, []byte("not a valid zip"), 0644); err != nil {
		t.Fatalf("Failed to create corrupt zip: %v", err)
	}

	// Compute checksum of the corrupt file (so verification passes)
	checksum := computeTestChecksum(t, zipPath)

	// Create manifest
	manifestContent := fmt.Sprintf(`{
		"project": "test-project",
		"source": "/some/path",
		"backups": [{
			"file": "20260101-120000.zip",
			"sha256": "%s",
			"size_bytes": 100,
			"created_at": "2026-01-01T12:00:00Z",
			"git_head": "abc123",
			"file_count": 1,
			"excluded": []
		}]
	}`, checksum)

	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	sourceDir := filepath.Join(tempDir, "source")

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	opts := RecoverOptions{
		Project: "test-project",
	}

	err = Recover(cfg, opts)
	if err == nil {
		t.Error("Recover should fail when extracting corrupt zip")
	}
	if !strings.Contains(err.Error(), "extracting backup") {
		t.Errorf("Expected 'extracting backup' error, got: %v", err)
	}
}

func TestExtractFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a zip file
	zipPath := filepath.Join(tempDir, "test.zip")
	createValidZip(t, zipPath)

	// Open and extract
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	if len(r.File) == 0 {
		t.Fatal("Zip has no files")
	}

	destPath := filepath.Join(tempDir, "extracted.txt")
	err = extractFile(r.File[0], destPath)
	if err != nil {
		t.Fatalf("extractFile failed: %v", err)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Content = %q, expected %q", string(content), "test content")
	}
}
