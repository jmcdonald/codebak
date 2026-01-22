package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcdonald/codebak/internal/adapters/execgit"
	"github.com/jmcdonald/codebak/internal/adapters/osfs"
	"github.com/jmcdonald/codebak/internal/adapters/ziparchiver"
	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/manifest"
	"github.com/jmcdonald/codebak/internal/mocks"
)

func TestCreateZipRoundTrip(t *testing.T) {
	// Create temp directories
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source directory with test files
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"file1.txt":          "content 1",
		"subdir/file2.txt":   "content 2",
		"deep/nested/f3.txt": "content 3",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", path, err)
		}
	}

	// Create zip using archiver adapter
	archiver := ziparchiver.New()
	zipPath := filepath.Join(tempDir, "backup.zip")
	fileCount, err := archiver.Create(zipPath, sourceDir, nil)
	if err != nil {
		t.Fatalf("archiver.Create failed: %v", err)
	}

	if fileCount != len(testFiles) {
		t.Errorf("fileCount = %d, expected %d", fileCount, len(testFiles))
	}

	// Verify zip contents
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	foundFiles := make(map[string]bool)
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			foundFiles[f.Name] = true
		}
	}

	// Check all expected files are in the zip (with project prefix)
	baseName := filepath.Base(sourceDir)
	for path := range testFiles {
		expectedPath := filepath.Join(baseName, path)
		if !foundFiles[expectedPath] {
			t.Errorf("Expected file %s not found in zip", expectedPath)
		}
	}
}

func TestCreateZipExclusions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")

	// Create files including ones that should be excluded
	files := []string{
		"main.go",
		"node_modules/dep/index.js",
		".venv/lib/python.py",
		"build/output.js",
		".DS_Store",
	}

	for _, f := range files {
		fullPath := filepath.Join(sourceDir, f)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Create zip with exclusions using archiver adapter
	archiver := ziparchiver.New()
	zipPath := filepath.Join(tempDir, "backup.zip")
	exclude := []string{"node_modules", ".venv", "build", ".DS_Store"}
	fileCount, err := archiver.Create(zipPath, sourceDir, exclude)
	if err != nil {
		t.Fatalf("archiver.Create failed: %v", err)
	}

	// Only main.go should be included
	if fileCount != 1 {
		t.Errorf("fileCount = %d, expected 1 (only main.go)", fileCount)
	}

	// Verify excluded files are NOT in zip
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		for _, excluded := range exclude {
			if filepath.Base(f.Name) == excluded || filepath.Base(filepath.Dir(f.Name)) == excluded {
				t.Errorf("Excluded file/dir found in zip: %s", f.Name)
			}
		}
	}
}

func TestShouldExclude(t *testing.T) {
	tests := []struct {
		path     string
		patterns []string
		expected bool
	}{
		{"node_modules", []string{"node_modules"}, true},
		{"src/node_modules", []string{"node_modules"}, true},
		{"file.pyc", []string{"*.pyc"}, true},
		{"dir/file.pyc", []string{"*.pyc"}, true},
		{"main.go", []string{"*.pyc", "node_modules"}, false},
		{".DS_Store", []string{".DS_Store"}, true},
		{"readme.md", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := shouldExclude(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("shouldExclude(%q, %v) = %v, expected %v", tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestHasChangesNoBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hasChanges, reason := HasChanges(tempDir, nil)
	if !hasChanges {
		t.Error("HasChanges should return true when no previous backup exists")
	}
	if reason != "no previous backup" {
		t.Errorf("reason = %q, expected %q", reason, "no previous backup")
	}
}

func TestHasChangesModifiedFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with current timestamp
	testFile := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a "previous backup" from yesterday
	lastBackup := &manifest.BackupEntry{
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}

	hasChanges, reason := HasChanges(tempDir, lastBackup)
	if !hasChanges {
		t.Error("HasChanges should return true when files are modified after last backup")
	}
	if reason != "files modified since last backup" {
		t.Errorf("reason = %q, expected %q", reason, "files modified since last backup")
	}
}

func TestHasChangesNoChanges(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a "previous backup" from the future (to ensure no files are newer)
	lastBackup := &manifest.BackupEntry{
		CreatedAt: time.Now().Add(24 * time.Hour),
	}

	hasChanges, _ := HasChanges(tempDir, lastBackup)
	if hasChanges {
		t.Error("HasChanges should return false when no files are modified after last backup")
	}
}

func TestShortHash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc123def456789", "abc123d"},
		{"short", "short"},
		{"exactly7", "exactly"},
		{"", ""},
		{"1234567890", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shortHash(tt.input)
			if result != tt.expected {
				t.Errorf("shortHash(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, expected %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestListProjects(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some project directories
	projects := []string{"project-a", "project-b", "project-c"}
	for _, p := range projects {
		if err := os.MkdirAll(filepath.Join(tempDir, p), 0755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}
	}

	// Create hidden directory (should be excluded)
	if err := os.MkdirAll(filepath.Join(tempDir, ".hidden"), 0755); err != nil {
		t.Fatalf("Failed to create hidden dir: %v", err)
	}

	// Create a file (should be excluded - only dirs)
	if err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	result, err := ListProjects(tempDir)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(result) != len(projects) {
		t.Errorf("ListProjects returned %d projects, expected %d", len(result), len(projects))
	}

	// Check all expected projects are present
	projectMap := make(map[string]bool)
	for _, p := range result {
		projectMap[p] = true
	}
	for _, p := range projects {
		if !projectMap[p] {
			t.Errorf("Expected project %q not found", p)
		}
	}

	// Hidden dir should not be included
	if projectMap[".hidden"] {
		t.Error("Hidden directory should not be included")
	}
}

func TestListProjectsEmptyDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	result, err := ListProjects(tempDir)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("ListProjects returned %d projects for empty dir, expected 0", len(result))
	}
}

func TestListProjectsNonExistent(t *testing.T) {
	_, err := ListProjects("/nonexistent/path")
	if err == nil {
		t.Error("ListProjects should fail for non-existent directory")
	}
}

func TestGetGitHeadNonGitRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	result := GetGitHead(tempDir)
	if result != "" {
		t.Errorf("GetGitHead for non-git repo should return empty string, got %q", result)
	}
}

func TestBackupProjectNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		SourceDir: tempDir,
		BackupDir: filepath.Join(tempDir, "backups"),
	}

	result := BackupProject(cfg, "nonexistent-project")
	if result.Error == nil {
		t.Error("BackupProject should fail for non-existent project")
	}
}

func TestBackupProjectSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(sourceDir, "test-project")

	// Create project with files
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
		Exclude:   []string{"node_modules"},
	}

	result := BackupProject(cfg, "test-project")
	if result.Error != nil {
		t.Fatalf("BackupProject failed: %v", result.Error)
	}

	if result.Skipped {
		t.Error("First backup should not be skipped")
	}

	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, expected 1", result.FileCount)
	}

	if result.Size == 0 {
		t.Error("Size should not be 0")
	}

	// Verify zip was created
	if _, err := os.Stat(result.ZipPath); os.IsNotExist(err) {
		t.Error("Zip file was not created")
	}
}

func TestBackupProjectSkipsUnchanged(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(sourceDir, "test-project")

	// Create project
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	// First backup
	result1 := BackupProject(cfg, "test-project")
	if result1.Error != nil {
		t.Fatalf("First backup failed: %v", result1.Error)
	}

	// Second backup (should be skipped since no changes)
	result2 := BackupProject(cfg, "test-project")
	if result2.Error != nil {
		t.Fatalf("Second backup failed: %v", result2.Error)
	}

	if !result2.Skipped {
		t.Error("Second backup should be skipped (no changes)")
	}
}

func TestRunBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backups")

	// Create two projects
	for _, name := range []string{"project-a", "project-b"} {
		projectDir := filepath.Join(sourceDir, name)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	results, err := RunBackup(cfg)
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("RunBackup returned %d results, expected 2", len(results))
	}

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("Backup of %s failed: %v", r.Project, r.Error)
		}
	}
}

// ============================================================================
// Additional tests for coverage improvement
// ============================================================================

func TestHasChangesGitHeadChanged(t *testing.T) {
	// Test git HEAD changed path using mocks
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Setup: Project is a git repo with different HEAD than last backup
	projectPath := "/test/project"
	mockGit.Repos[projectPath] = true
	mockGit.Heads[projectPath] = "newhead1234567890"

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	lastBackup := &manifest.BackupEntry{
		GitHead:   "oldhead0987654321",
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}

	hasChanges, reason := svc.HasChanges(projectPath, lastBackup)
	if !hasChanges {
		t.Error("HasChanges should return true when git HEAD changed")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestHasChangesGitHeadUnchanged(t *testing.T) {
	// Test git HEAD unchanged path
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	projectPath := "/test/project"
	headCommit := "samehead1234567890"
	mockGit.Repos[projectPath] = true
	mockGit.Heads[projectPath] = headCommit

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	lastBackup := &manifest.BackupEntry{
		GitHead:   headCommit, // Same as current
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}

	hasChanges, reason := svc.HasChanges(projectPath, lastBackup)
	if hasChanges {
		t.Error("HasChanges should return false when git HEAD unchanged")
	}
	if reason != "git HEAD unchanged" {
		t.Errorf("reason = %q, expected %q", reason, "git HEAD unchanged")
	}
}

func TestHasChangesNonGitRepoWithNewerFiles(t *testing.T) {
	// Test non-git repo with newer files (mtime fallback)
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	projectPath := "/test/project"
	// Not a git repo
	mockGit.Repos[projectPath] = false

	// Setup walk to return a newer file
	mockFS.WalkEntries = []mocks.WalkEntry{
		{
			Path: projectPath + "/file.txt",
			Info: &mockFileInfo{modTime: time.Now()}, // File modified now
		},
	}

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	lastBackup := &manifest.BackupEntry{
		CreatedAt: time.Now().Add(-24 * time.Hour), // Backup was yesterday
	}

	hasChanges, reason := svc.HasChanges(projectPath, lastBackup)
	if !hasChanges {
		t.Error("HasChanges should return true when files modified after last backup")
	}
	if reason != "files modified since last backup" {
		t.Errorf("reason = %q, expected %q", reason, "files modified since last backup")
	}
}

func TestHasChangesNonGitRepoNoChanges(t *testing.T) {
	// Test non-git repo with no newer files
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	projectPath := "/test/project"
	mockGit.Repos[projectPath] = false

	// Setup walk to return an older file
	mockFS.WalkEntries = []mocks.WalkEntry{
		{
			Path: projectPath + "/file.txt",
			Info: &mockFileInfo{modTime: time.Now().Add(-48 * time.Hour)}, // File 2 days old
		},
	}

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	lastBackup := &manifest.BackupEntry{
		CreatedAt: time.Now().Add(-24 * time.Hour), // Backup was yesterday
	}

	hasChanges, reason := svc.HasChanges(projectPath, lastBackup)
	if hasChanges {
		t.Error("HasChanges should return false when no files modified after last backup")
	}
	if reason != "no changes detected" {
		t.Errorf("reason = %q, expected %q", reason, "no changes detected")
	}
}

func TestBackupProjectMkdirAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	sourceDir := filepath.Join(tempDir, "source")
	projectDir := filepath.Join(sourceDir, "test-project")
	backupDir := filepath.Join(tempDir, "backups")
	projectBackupDir := filepath.Join(backupDir, "test-project")

	// Create actual project dir (so Stat succeeds via real FS)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Mock Stat to succeed, but MkdirAll to fail
	mockFS.Stats[projectDir] = &mockFileInfo{name: "test-project", isDir: true}
	mockFS.Errors[projectBackupDir] = os.ErrPermission

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	result := svc.BackupProject(cfg, "test-project")
	if result.Error == nil {
		t.Error("BackupProject should fail when MkdirAll fails")
	}
}

func TestBackupProjectArchiverCreateError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	sourceDir := filepath.Join(tempDir, "source")
	projectDir := filepath.Join(sourceDir, "test-project")
	backupDir := filepath.Join(tempDir, "backups")

	// Create actual project dir
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Mock Stat to succeed
	mockFS.Stats[projectDir] = &mockFileInfo{name: "test-project", isDir: true}

	// Mock archiver to fail
	mockArchiver.Errors["Create"] = os.ErrPermission

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	result := svc.BackupProject(cfg, "test-project")
	if result.Error == nil {
		t.Error("BackupProject should fail when archiver.Create fails")
	}
}

func TestRunBackupListProjectsError(t *testing.T) {
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Mock ReadDir to fail
	sourceDir := "/test/source"
	mockFS.Errors[sourceDir] = os.ErrPermission

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: "/test/backups",
	}

	// With multi-source support, failed sources are skipped gracefully
	// (returns empty results instead of error)
	results, err := svc.RunBackup(cfg)
	if err != nil {
		t.Errorf("RunBackup should not fail with multi-source; got error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results when source fails, got %d", len(results))
	}
}

func TestNewServiceAndNewDefaultService(t *testing.T) {
	// Test NewDefaultService
	svc := NewDefaultService()
	if svc == nil {
		t.Fatal("NewDefaultService returned nil")
	}
	if svc.fs == nil {
		t.Error("NewDefaultService should set filesystem")
	}
	if svc.git == nil {
		t.Error("NewDefaultService should set git client")
	}
	if svc.archiver == nil {
		t.Error("NewDefaultService should set archiver")
	}
	if svc.restic == nil {
		t.Error("NewDefaultService should set restic client")
	}

	// Test NewService with mocks
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()
	svc2 := NewService(mockFS, mockGit, mockArchiver, mockRestic)
	if svc2 == nil {
		t.Fatal("NewService returned nil")
	}
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *mockFileInfo) Name() string       { return fi.name }
func (fi *mockFileInfo) Size() int64        { return fi.size }
func (fi *mockFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *mockFileInfo) IsDir() bool        { return fi.isDir }
func (fi *mockFileInfo) Sys() interface{}   { return nil }

func TestBackupProjectManifestLoadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(sourceDir, "test-project")
	projectBackupDir := filepath.Join(backupDir, "test-project")

	// Create project with files
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create malformed manifest
	if err := os.MkdirAll(projectBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}
	manifestPath := filepath.Join(projectBackupDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("invalid json {{{"), 0644); err != nil {
		t.Fatalf("Failed to create manifest: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
	}

	result := BackupProject(cfg, "test-project")
	if result.Error == nil {
		t.Error("BackupProject should fail when manifest Load fails")
	}
}

func TestHasChangesWalkError(t *testing.T) {
	// Test walk error handling in HasChanges
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	projectPath := "/test/project"
	// Not a git repo so it falls back to mtime check
	mockGit.Repos[projectPath] = false

	// Setup walk with an error
	mockFS.WalkEntries = []mocks.WalkEntry{
		{
			Path: projectPath + "/file.txt",
			Info: nil,
			Err:  os.ErrPermission, // Walk encounters an error
		},
	}

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	lastBackup := &manifest.BackupEntry{
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}

	// Should handle error gracefully and continue (return false/no changes)
	hasChanges, _ := svc.HasChanges(projectPath, lastBackup)
	// Error handling in walk continues, so it should return no changes
	if hasChanges {
		t.Error("HasChanges should return false when walk encounters error and finds no newer files")
	}
}

func TestBackupProjectWithRetention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(sourceDir, "test-project")

	// Create project with files
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		SourceDir: sourceDir,
		BackupDir: backupDir,
		Exclude:   []string{"node_modules"},
		Retention: struct {
			KeepLast int `yaml:"keep_last"`
		}{KeepLast: 5}, // Enable retention
	}

	result := BackupProject(cfg, "test-project")
	if result.Error != nil {
		t.Fatalf("BackupProject failed: %v", result.Error)
	}

	if result.Skipped {
		t.Error("First backup should not be skipped")
	}
}

// ============================================================================
// Tests for sensitive source backup and mixed configs
// ============================================================================

func TestBackupSensitiveSourceSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Create sensitive source directory
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0755); err != nil {
		t.Fatalf("Failed to create ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("private key"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	repoPath := filepath.Join(tempDir, "restic-repo")
	backupDir := filepath.Join(tempDir, "backups")

	// Setup mock to track stat calls
	mockFS.Stats[sshDir] = &mockFileInfo{name: ".ssh", isDir: true}

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	// Set password env var
	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	source := config.Source{
		Path:  sshDir,
		Label: "SSH Keys",
		Type:  config.SourceTypeSensitive,
	}

	cfg := &config.Config{
		BackupDir: backupDir,
		Restic: config.ResticConfig{
			RepoPath: repoPath,
		},
	}

	result := svc.BackupSensitiveSource(cfg, source)
	if result.Error != nil {
		t.Fatalf("BackupSensitiveSource failed: %v", result.Error)
	}

	if result.Project != "SSH Keys" {
		t.Errorf("result.Project = %q, expected %q", result.Project, "SSH Keys")
	}

	if result.SnapshotID == "" {
		t.Error("SnapshotID should not be empty")
	}

	if result.SourceType != config.SourceTypeSensitive {
		t.Errorf("result.SourceType = %q, expected %q", result.SourceType, config.SourceTypeSensitive)
	}

	// Verify restic was initialized
	if !mockRestic.InitializedRepos[repoPath] {
		t.Error("Restic repo should be initialized")
	}
}

func TestBackupSensitiveSourceNonExistentPath(t *testing.T) {
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Mock Stat to return not found
	mockFS.Errors["/nonexistent/.ssh"] = os.ErrNotExist

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	// Set password env var
	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	source := config.Source{
		Path: "/nonexistent/.ssh",
		Type: config.SourceTypeSensitive,
	}

	cfg := &config.Config{
		BackupDir: "/tmp/backups",
		Restic: config.ResticConfig{
			RepoPath: "/tmp/restic-repo",
		},
	}

	result := svc.BackupSensitiveSource(cfg, source)
	if !result.Skipped {
		t.Error("Should be skipped when source path doesn't exist")
	}
	if result.Reason != "source path does not exist" {
		t.Errorf("reason = %q, expected %q", result.Reason, "source path does not exist")
	}
}

func TestBackupSensitiveSourceNoPassword(t *testing.T) {
	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Make sure password is not set
	os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	source := config.Source{
		Path: "/test/.ssh",
		Type: config.SourceTypeSensitive,
	}

	cfg := &config.Config{
		BackupDir: "/tmp/backups",
		Restic: config.ResticConfig{
			RepoPath: "/tmp/restic-repo",
		},
	}

	result := svc.BackupSensitiveSource(cfg, source)
	if result.Error == nil {
		t.Error("Should fail when password env var is not set")
	}
}

func TestBackupSensitiveSourceResticBackupError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockFS := mocks.NewMockFileSystem()
	mockGit := mocks.NewMockGitClient()
	mockArchiver := mocks.NewMockArchiver()
	mockRestic := mocks.NewMockResticClient()

	// Create directory
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0755); err != nil {
		t.Fatalf("Failed to create ssh dir: %v", err)
	}

	repoPath := filepath.Join(tempDir, "restic-repo")

	mockFS.Stats[sshDir] = &mockFileInfo{name: ".ssh", isDir: true}
	mockRestic.Errors.Backup = os.ErrPermission

	svc := NewService(mockFS, mockGit, mockArchiver, mockRestic)

	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	source := config.Source{
		Path: sshDir,
		Type: config.SourceTypeSensitive,
	}

	cfg := &config.Config{
		BackupDir: filepath.Join(tempDir, "backups"),
		Restic: config.ResticConfig{
			RepoPath: repoPath,
		},
	}

	result := svc.BackupSensitiveSource(cfg, source)
	if result.Error == nil {
		t.Error("Should fail when restic backup fails")
	}
}

func TestRunBackupMixedSources(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create git source directory with a project
	codeDir := filepath.Join(tempDir, "code")
	projectDir := filepath.Join(codeDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create sensitive source directory
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0755); err != nil {
		t.Fatalf("Failed to create ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("key"), 0600); err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}

	backupDir := filepath.Join(tempDir, "backups")
	repoPath := filepath.Join(tempDir, "restic-repo")

	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	// Use real filesystem and git, but mock restic
	mockRestic := mocks.NewMockResticClient()
	svc := NewService(
		&osfs.OSFileSystem{},
		&execgit.ExecGitClient{},
		ziparchiver.New(),
		mockRestic,
	)

	cfg := &config.Config{
		Sources: []config.Source{
			{Path: codeDir, Type: config.SourceTypeGit, Label: "Code"},
			{Path: sshDir, Type: config.SourceTypeSensitive, Label: "SSH Keys"},
		},
		BackupDir: backupDir,
		Restic: config.ResticConfig{
			RepoPath: repoPath,
		},
	}

	results, err := svc.RunBackup(cfg)
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Check we have one of each type
	var hasGit, hasSensitive bool
	for _, r := range results {
		if r.SourceType == config.SourceTypeGit {
			hasGit = true
			if r.Error != nil {
				t.Errorf("Git backup failed: %v", r.Error)
			}
		}
		if r.SourceType == config.SourceTypeSensitive {
			hasSensitive = true
			if r.Error != nil {
				t.Errorf("Sensitive backup failed: %v", r.Error)
			}
			if r.SnapshotID == "" {
				t.Error("Sensitive backup should have SnapshotID")
			}
		}
	}

	if !hasGit {
		t.Error("Missing git backup result")
	}
	if !hasSensitive {
		t.Error("Missing sensitive backup result")
	}
}

func TestRunBackupSensitiveSourcesOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create two sensitive source directories
	sshDir := filepath.Join(tempDir, ".ssh")
	awsDir := filepath.Join(tempDir, ".aws")
	for _, dir := range []string{sshDir, awsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config"), []byte("config"), 0600); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	backupDir := filepath.Join(tempDir, "backups")
	repoPath := filepath.Join(tempDir, "restic-repo")

	os.Setenv("CODEBAK_RESTIC_PASSWORD", "test-password")
	defer os.Unsetenv("CODEBAK_RESTIC_PASSWORD")

	// Use real filesystem but mock restic
	mockRestic := mocks.NewMockResticClient()
	svc := NewService(
		&osfs.OSFileSystem{},
		&execgit.ExecGitClient{},
		ziparchiver.New(),
		mockRestic,
	)

	cfg := &config.Config{
		Sources: []config.Source{
			{Path: sshDir, Type: config.SourceTypeSensitive, Label: "SSH"},
			{Path: awsDir, Type: config.SourceTypeSensitive, Label: "AWS"},
		},
		BackupDir: backupDir,
		Restic: config.ResticConfig{
			RepoPath: repoPath,
		},
	}

	results, err := svc.RunBackup(cfg)
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("Backup of %s failed: %v", r.Project, r.Error)
		}
		if r.SourceType != config.SourceTypeSensitive {
			t.Errorf("Expected SourceType sensitive, got %q", r.SourceType)
		}
	}
}
