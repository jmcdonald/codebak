package tui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcdonaldj/codebak/internal/config"
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
