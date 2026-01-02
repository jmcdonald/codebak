package tui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcdonald/codebak/internal/config"
)

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "plain text",
			content:  "Hello, world!\nThis is a test file.\n",
			expected: false,
		},
		{
			name:     "text with unicode",
			content:  "Hello 世界! Émojis: 🎉",
			expected: false,
		},
		{
			name:     "binary with null bytes",
			content:  "some\x00binary\x00content",
			expected: true,
		},
		{
			name:     "invalid UTF-8",
			content:  string([]byte{0xff, 0xfe, 0x00, 0x01}),
			expected: true,
		},
		{
			name:     "go source code",
			content:  "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBinaryContent(tt.content)
			if result != tt.expected {
				t.Errorf("IsBinaryContent() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestContainsLine(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		target   string
		expected bool
	}{
		{
			name:     "found at start",
			lines:    []string{"apple", "banana", "cherry"},
			target:   "apple",
			expected: true,
		},
		{
			name:     "found at end",
			lines:    []string{"apple", "banana", "cherry"},
			target:   "cherry",
			expected: true,
		},
		{
			name:     "not found",
			lines:    []string{"apple", "banana", "cherry"},
			target:   "grape",
			expected: false,
		},
		{
			name:     "empty slice",
			lines:    []string{},
			target:   "anything",
			expected: false,
		},
		{
			name:     "empty target",
			lines:    []string{"apple", "", "cherry"},
			target:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsLine(tt.lines, tt.target)
			if result != tt.expected {
				t.Errorf("containsLine() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConvertToLineDiff(t *testing.T) {
	tests := []struct {
		name         string
		content1     string
		content2     string
		expectedLen  int
		checkAdded   int
		checkDeleted int
	}{
		{
			name:         "identical content",
			content1:     "line1\nline2\nline3",
			content2:     "line1\nline2\nline3",
			expectedLen:  3,
			checkAdded:   0,
			checkDeleted: 0,
		},
		{
			name:         "added lines",
			content1:     "line1",
			content2:     "line1\nline2",
			expectedLen:  2,
			checkAdded:   1,
			checkDeleted: 0,
		},
		{
			name:         "deleted lines",
			content1:     "line1\nline2",
			content2:     "line1",
			expectedLen:  2,
			checkAdded:   0,
			checkDeleted: 1,
		},
		{
			name:         "empty to content",
			content1:     "",
			content2:     "new content",
			expectedLen:  2, // "" splits to [""], so we get a delete of "" and add of "new content"
			checkAdded:   1,
			checkDeleted: 1,
		},
		{
			name:         "content to empty",
			content1:     "old content",
			content2:     "",
			expectedLen:  2, // "" splits to [""], so we get a delete of "old content" and add of ""
			checkAdded:   1,
			checkDeleted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We pass nil for diffs since convertToLineDiff does its own line comparison
			result := convertToLineDiff(tt.content1, tt.content2, nil)

			if len(result) != tt.expectedLen {
				t.Errorf("len(result) = %d, expected %d", len(result), tt.expectedLen)
			}

			added := 0
			deleted := 0
			for _, line := range result {
				if line.Type == '+' {
					added++
				}
				if line.Type == '-' {
					deleted++
				}
			}

			if added != tt.checkAdded {
				t.Errorf("added lines = %d, expected %d", added, tt.checkAdded)
			}
			if deleted != tt.checkDeleted {
				t.Errorf("deleted lines = %d, expected %d", deleted, tt.checkDeleted)
			}
		})
	}
}

func TestListZipFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test zip
	zipPath := filepath.Join(tempDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"project/file1.txt":        "content 1",
		"project/subdir/file2.txt": "content 2",
	})

	files, err := listZipFiles(zipPath)
	if err != nil {
		t.Fatalf("listZipFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("len(files) = %d, expected 2", len(files))
	}

	if _, ok := files["file1.txt"]; !ok {
		t.Error("file1.txt not found")
	}
	if _, ok := files["subdir/file2.txt"]; !ok {
		t.Error("subdir/file2.txt not found")
	}
}

func TestReadZipFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test zip
	zipPath := filepath.Join(tempDir, "test.zip")
	expectedContent := "hello world"
	createTestZip(t, zipPath, map[string]string{
		"myproject/readme.txt": expectedContent,
	})

	content, err := ReadZipFile(zipPath, "readme.txt", "myproject")
	if err != nil {
		t.Fatalf("ReadZipFile failed: %v", err)
	}

	if content != expectedContent {
		t.Errorf("content = %q, expected %q", content, expectedContent)
	}
}

func TestReadZipFileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"project/exists.txt": "content",
	})

	_, err = ReadZipFile(zipPath, "notexists.txt", "project")
	if err == nil {
		t.Error("ReadZipFile should fail for non-existent file")
	}
}

func TestComputeDiff(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create two versions with differences
	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/file1.txt": "original content",
		"testproj/file2.txt": "will be deleted",
	})

	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/file1.txt": "modified content",
		"testproj/file3.txt": "new file",
	})

	cfg := &config.Config{
		BackupDir: backupDir,
	}

	result, err := ComputeDiff(cfg, "testproj", "v1.zip", "v2.zip")
	if err != nil {
		t.Fatalf("ComputeDiff failed: %v", err)
	}

	if result.Added != 1 {
		t.Errorf("Added = %d, expected 1", result.Added)
	}
	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, expected 1", result.Deleted)
	}
	if result.Modified != 1 {
		t.Errorf("Modified = %d, expected 1", result.Modified)
	}
}

// Helper to create test zips
func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
		fw.Write([]byte(content))
	}
	w.Close()
}

// ============================================
// ComputeFileDiff tests
// ============================================

func TestComputeFileDiffModified(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create two versions with modified file
	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/main.go": "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/main.go": "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n",
	})

	cfg := &config.Config{BackupDir: backupDir}

	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "main.go", 'M')
	if err != nil {
		t.Fatalf("ComputeFileDiff failed: %v", err)
	}

	if result.Path != "main.go" {
		t.Errorf("Path = %q, expected 'main.go'", result.Path)
	}
	if result.IsBinary {
		t.Error("IsBinary should be false")
	}
	if result.Error != "" {
		t.Errorf("Error = %q, expected empty", result.Error)
	}
	if len(result.Lines) == 0 {
		t.Error("Lines should not be empty")
	}
}

func TestComputeFileDiffAdded(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/existing.txt": "existing content",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/existing.txt": "existing content",
		"testproj/newfile.txt":  "new file content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "newfile.txt", 'A')
	if err != nil {
		t.Fatalf("ComputeFileDiff failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Error = %q, expected empty", result.Error)
	}
	// Added files should show all lines as additions
	hasAdditions := false
	for _, line := range result.Lines {
		if line.Type == '+' {
			hasAdditions = true
			break
		}
	}
	if !hasAdditions {
		t.Error("Added file should have addition lines")
	}
}

func TestComputeFileDiffDeleted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/deleted.txt": "deleted content",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/other.txt": "other content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "deleted.txt", 'D')
	if err != nil {
		t.Fatalf("ComputeFileDiff failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Error = %q, expected empty", result.Error)
	}
	// Deleted files should show all lines as deletions
	hasDeletions := false
	for _, line := range result.Lines {
		if line.Type == '-' {
			hasDeletions = true
			break
		}
	}
	if !hasDeletions {
		t.Error("Deleted file should have deletion lines")
	}
}

func TestComputeFileDiffBinaryFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create files with binary content (null bytes)
	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/binary.bin": "binary\x00content\x00here",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/binary.bin": "modified\x00binary\x00content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "binary.bin", 'M')
	if err != nil {
		t.Fatalf("ComputeFileDiff failed: %v", err)
	}

	if !result.IsBinary {
		t.Error("IsBinary should be true for binary files")
	}
}

func TestComputeFileDiffReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/file.txt": "content",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/file.txt": "content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	// Try to read a file that doesn't exist in the zip
	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "nonexistent.txt", 'M')
	if err != nil {
		t.Fatalf("ComputeFileDiff should not return error, it sets result.Error: %v", err)
	}

	if result.Error == "" {
		t.Error("Error should be set for missing file")
	}
}

func TestComputeFileDiffReadErrorV2(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/file.txt": "content v1",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/other.txt": "other content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	// Try to read a modified file where v2 doesn't have it
	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "file.txt", 'M')
	if err != nil {
		t.Fatalf("ComputeFileDiff should not return error: %v", err)
	}

	if result.Error == "" {
		t.Error("Error should be set when v2 is missing the file")
	}
}

func TestComputeFileDiffAddedReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/other.txt": "content",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/other.txt": "content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	// Try to read an added file that doesn't exist
	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "missing.txt", 'A')
	if err != nil {
		t.Fatalf("ComputeFileDiff should not return error: %v", err)
	}

	if result.Error == "" {
		t.Error("Error should be set for missing added file")
	}
}

func TestComputeFileDiffDeletedReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-filediff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backupDir := filepath.Join(tempDir, "backups")
	projectDir := filepath.Join(backupDir, "testproj")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	createTestZip(t, filepath.Join(projectDir, "v1.zip"), map[string]string{
		"testproj/other.txt": "content",
	})
	createTestZip(t, filepath.Join(projectDir, "v2.zip"), map[string]string{
		"testproj/other.txt": "content",
	})

	cfg := &config.Config{BackupDir: backupDir}

	// Try to read a deleted file that doesn't exist in v1
	result, err := ComputeFileDiff(cfg, "testproj", "v1", "v2", "missing.txt", 'D')
	if err != nil {
		t.Fatalf("ComputeFileDiff should not return error: %v", err)
	}

	if result.Error == "" {
		t.Error("Error should be set for missing deleted file")
	}
}

func TestListZipFilesError(t *testing.T) {
	_, err := listZipFiles("/nonexistent/path/to.zip")
	if err == nil {
		t.Error("listZipFiles should return error for missing file")
	}
}

func TestComputeDiffZipError(t *testing.T) {
	cfg := &config.Config{BackupDir: "/nonexistent"}

	_, err := ComputeDiff(cfg, "project", "v1.zip", "v2.zip")
	if err == nil {
		t.Error("ComputeDiff should return error for missing zip files")
	}
}

func TestListZipFilesWithDirectories(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-zipdir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}

	w := zip.NewWriter(f)
	// Create a directory entry (ends with /)
	_, err = w.Create("project/subdir/")
	if err != nil {
		t.Fatalf("Failed to create dir entry: %v", err)
	}
	// Create a file
	fw, err := w.Create("project/file.txt")
	if err != nil {
		t.Fatalf("Failed to create file entry: %v", err)
	}
	fw.Write([]byte("content"))
	w.Close()
	f.Close()

	files, err := listZipFiles(zipPath)
	if err != nil {
		t.Fatalf("listZipFiles failed: %v", err)
	}

	// Should only contain files, not directories
	if len(files) != 1 {
		t.Errorf("len(files) = %d, expected 1", len(files))
	}
}

func TestListZipFilesShortPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-zipshort-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip: %v", err)
	}

	w := zip.NewWriter(f)
	// Create a file with no directory prefix
	fw, err := w.Create("toplevel.txt")
	if err != nil {
		t.Fatalf("Failed to create file entry: %v", err)
	}
	fw.Write([]byte("content"))
	w.Close()
	f.Close()

	files, err := listZipFiles(zipPath)
	if err != nil {
		t.Fatalf("listZipFiles failed: %v", err)
	}

	// Should not include files without project prefix
	if len(files) != 0 {
		t.Errorf("len(files) = %d, expected 0 (files without project prefix should be skipped)", len(files))
	}
}

func TestIsBinaryContentLongContent(t *testing.T) {
	// Test with content longer than 8000 bytes
	longText := make([]byte, 10000)
	for i := range longText {
		longText[i] = 'a'
	}

	if IsBinaryContent(string(longText)) {
		t.Error("Long text content should not be detected as binary")
	}

	// Add null byte after 8000 chars - should not be detected
	longTextWithNull := make([]byte, 10000)
	copy(longTextWithNull, longText)
	longTextWithNull[9000] = 0

	if IsBinaryContent(string(longTextWithNull)) {
		t.Error("Null byte after 8000 chars should not be detected as binary")
	}
}

func TestConvertToLineDiffComplexCases(t *testing.T) {
	tests := []struct {
		name     string
		content1 string
		content2 string
	}{
		{
			name:     "lines reordered",
			content1: "line1\nline2\nline3",
			content2: "line3\nline2\nline1",
		},
		{
			name:     "lines duplicated",
			content1: "line1\nline2",
			content2: "line1\nline1\nline2",
		},
		{
			name:     "multiple additions and deletions",
			content1: "a\nb\nc\nd\ne",
			content2: "a\nx\nc\ny\ne",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			result := convertToLineDiff(tt.content1, tt.content2, nil)
			if result == nil {
				t.Error("result should not be nil")
			}
		})
	}
}
