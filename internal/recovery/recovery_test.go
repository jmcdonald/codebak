package recovery

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

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

	err = extractZip(maliciousZipPath, destDir)
	if err == nil {
		t.Error("extractZip should have rejected malicious zip with path traversal")
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

	err = extractZip(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractZip failed for valid archive: %v", err)
	}

	// Verify files were extracted
	extractedFile := filepath.Join(destDir, "project", "file.txt")
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

	// Add a valid file
	fw, err := w.Create("project/file.txt")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	fw.Write([]byte("test content"))

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}
}
