package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mcdonaldj/codebak/internal/adapters/ziparchiver"
	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/mcdonaldj/codebak/internal/manifest"
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
