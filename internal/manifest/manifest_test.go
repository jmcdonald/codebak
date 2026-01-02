package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManifestSerializationRoundTrip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a manifest with various data
	original := &Manifest{
		Project: "test-project",
		Source:  "/home/user/code/test-project",
		Backups: []BackupEntry{
			{
				File:      "20241215-100000.zip",
				SHA256:    "abc123def456789",
				SizeBytes: 1024000,
				CreatedAt: time.Date(2024, 12, 15, 10, 0, 0, 0, time.UTC),
				GitHead:   "deadbeef12345",
				FileCount: 150,
				Excluded:  []string{"node_modules", ".venv"},
			},
			{
				File:      "20241216-100000.zip",
				SHA256:    "xyz789abc123",
				SizeBytes: 1024500,
				CreatedAt: time.Date(2024, 12, 16, 10, 0, 0, 0, time.UTC),
				GitHead:   "cafebabe67890",
				FileCount: 155,
				Excluded:  []string{"node_modules", ".venv"},
			},
		},
	}

	// Save the manifest
	if err := original.Save(tempDir); err != nil {
		t.Fatalf("Failed to save manifest: %v", err)
	}

	// Load it back
	loaded, err := Load(tempDir, "test-project")
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	// Verify fields
	if loaded.Project != original.Project {
		t.Errorf("Project = %q, expected %q", loaded.Project, original.Project)
	}
	if loaded.Source != original.Source {
		t.Errorf("Source = %q, expected %q", loaded.Source, original.Source)
	}
	if len(loaded.Backups) != len(original.Backups) {
		t.Fatalf("Backups count = %d, expected %d", len(loaded.Backups), len(original.Backups))
	}

	// Verify backup entries
	for i, backup := range loaded.Backups {
		orig := original.Backups[i]
		if backup.File != orig.File {
			t.Errorf("Backup[%d].File = %q, expected %q", i, backup.File, orig.File)
		}
		if backup.SHA256 != orig.SHA256 {
			t.Errorf("Backup[%d].SHA256 = %q, expected %q", i, backup.SHA256, orig.SHA256)
		}
		if backup.SizeBytes != orig.SizeBytes {
			t.Errorf("Backup[%d].SizeBytes = %d, expected %d", i, backup.SizeBytes, orig.SizeBytes)
		}
		if backup.GitHead != orig.GitHead {
			t.Errorf("Backup[%d].GitHead = %q, expected %q", i, backup.GitHead, orig.GitHead)
		}
	}
}

func TestLoadMissingManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Load a manifest that doesn't exist
	m, err := Load(tempDir, "nonexistent-project")
	if err != nil {
		t.Fatalf("Load should not error for missing manifest: %v", err)
	}

	// Should return empty manifest
	if m.Project != "nonexistent-project" {
		t.Errorf("Project = %q, expected %q", m.Project, "nonexistent-project")
	}
	if len(m.Backups) != 0 {
		t.Errorf("Backups should be empty, got %d entries", len(m.Backups))
	}
}

func TestLoadMalformedManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create project directory and malformed manifest
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	manifestPath := filepath.Join(projectDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("this is not valid json {{{"), 0644); err != nil {
		t.Fatalf("Failed to write malformed manifest: %v", err)
	}

	// Load should fail
	_, err = Load(tempDir, "test-project")
	if err == nil {
		t.Error("Load should fail for malformed JSON")
	}
}

func TestLatestBackup(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{
			{File: "20241215-100000.zip"},
			{File: "20241216-100000.zip"},
			{File: "20241217-100000.zip"},
		},
	}

	latest := m.LatestBackup()
	if latest == nil {
		t.Fatal("LatestBackup returned nil")
	}
	if latest.File != "20241217-100000.zip" {
		t.Errorf("LatestBackup.File = %q, expected %q", latest.File, "20241217-100000.zip")
	}
}

func TestLatestBackupEmpty(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{},
	}

	latest := m.LatestBackup()
	if latest != nil {
		t.Error("LatestBackup should return nil for empty manifest")
	}
}

func TestAddBackup(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{},
	}

	entry := BackupEntry{
		File:      "20241215-100000.zip",
		SHA256:    "abc123",
		SizeBytes: 1024,
	}

	m.AddBackup(entry)

	if len(m.Backups) != 1 {
		t.Fatalf("Backups count = %d, expected 1", len(m.Backups))
	}
	if m.Backups[0].File != entry.File {
		t.Errorf("Added backup File = %q, expected %q", m.Backups[0].File, entry.File)
	}
}

func TestPrune(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create project backup dir
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create dummy backup files
	files := []string{
		"20241213-100000.zip",
		"20241214-100000.zip",
		"20241215-100000.zip",
		"20241216-100000.zip",
		"20241217-100000.zip",
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(projectDir, f), []byte("dummy"), 0644); err != nil {
			t.Fatalf("Failed to create backup file: %v", err)
		}
	}

	// Create manifest with all backups
	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{
			{File: "20241213-100000.zip"},
			{File: "20241214-100000.zip"},
			{File: "20241215-100000.zip"},
			{File: "20241216-100000.zip"},
			{File: "20241217-100000.zip"},
		},
	}

	// Prune to keep only 3
	deleted, err := m.Prune(tempDir, 3)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Should have deleted 2 oldest
	if len(deleted) != 2 {
		t.Errorf("Deleted count = %d, expected 2", len(deleted))
	}

	// Manifest should have 3 backups
	if len(m.Backups) != 3 {
		t.Errorf("Remaining backups = %d, expected 3", len(m.Backups))
	}

	// Check oldest files were deleted
	for _, f := range []string{"20241213-100000.zip", "20241214-100000.zip"} {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err == nil {
			t.Errorf("Old backup %s should have been deleted", f)
		}
	}

	// Check newest files still exist
	for _, f := range []string{"20241215-100000.zip", "20241216-100000.zip", "20241217-100000.zip"} {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("Recent backup %s should still exist", f)
		}
	}
}

func TestPruneNoAction(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{
			{File: "backup1.zip"},
			{File: "backup2.zip"},
		},
	}

	// Prune with keepLast >= current count
	deleted, err := m.Prune("/tmp", 5)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if len(deleted) != 0 {
		t.Error("Prune should not delete anything when under limit")
	}
	if len(m.Backups) != 2 {
		t.Error("Backups should be unchanged")
	}
}

func TestComputeSHA256(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file with known content
	testFile := filepath.Join(tempDir, "test.txt")
	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	hash, err := ComputeSHA256(testFile)
	if err != nil {
		t.Fatalf("ComputeSHA256 failed: %v", err)
	}

	// Known SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("SHA256 = %q, expected %q", hash, expected)
	}
}

func TestManifestJSONFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m := &Manifest{
		Project: "test",
		Source:  "/path/to/source",
		Backups: []BackupEntry{
			{
				File:      "backup.zip",
				SHA256:    "hash",
				SizeBytes: 1024,
			},
		},
	}

	if err := m.Save(tempDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read the raw JSON
	data, err := os.ReadFile(ManifestPath(tempDir, "test"))
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	// Verify it's valid JSON with expected structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Manifest is not valid JSON: %v", err)
	}

	if parsed["project"] != "test" {
		t.Error("JSON project field mismatch")
	}
	if parsed["source"] != "/path/to/source" {
		t.Error("JSON source field mismatch")
	}
}

// ============================================================================
// Additional tests for coverage improvement
// ============================================================================

func TestLoadReadError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create project directory
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create manifest.json as a directory (causes read error)
	manifestPath := filepath.Join(projectDir, "manifest.json")
	if err := os.MkdirAll(manifestPath, 0755); err != nil {
		t.Fatalf("Failed to create manifest dir: %v", err)
	}

	// Load should fail
	_, err = Load(tempDir, "test-project")
	if err == nil {
		t.Error("Load should fail when manifest.json is a directory")
	}
}

func TestSaveMkdirAllError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file where the project directory should be
	// This will cause MkdirAll to fail
	projectPath := filepath.Join(tempDir, "test-project")
	if err := os.WriteFile(projectPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{},
	}

	err = m.Save(tempDir)
	if err == nil {
		t.Error("Save should fail when MkdirAll fails")
	}
}

func TestPruneZeroKeepLast(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{
			{File: "backup1.zip"},
			{File: "backup2.zip"},
		},
	}

	// Prune with keepLast = 0
	deleted, err := m.Prune("/tmp", 0)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if len(deleted) != 0 {
		t.Error("Prune with keepLast=0 should not delete anything")
	}
	if len(m.Backups) != 2 {
		t.Error("Backups should be unchanged")
	}
}

func TestPruneNegativeKeepLast(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{
			{File: "backup1.zip"},
			{File: "backup2.zip"},
		},
	}

	// Prune with keepLast = -1
	deleted, err := m.Prune("/tmp", -1)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if len(deleted) != 0 {
		t.Error("Prune with keepLast<0 should not delete anything")
	}
	if len(m.Backups) != 2 {
		t.Error("Backups should be unchanged")
	}
}

func TestPruneFileMissing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create project backup dir
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create manifest with files that don't exist
	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{
			{File: "missing1.zip"},
			{File: "missing2.zip"},
			{File: "missing3.zip"},
		},
	}

	// Prune should handle missing files gracefully
	_, err = m.Prune(tempDir, 1)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Files were "deleted" (or already missing)
	// The behavior continues even when file doesn't exist
	if len(m.Backups) != 1 {
		t.Errorf("Should have 1 backup remaining, got %d", len(m.Backups))
	}
}

func TestComputeSHA256FileNotFound(t *testing.T) {
	_, err := ComputeSHA256("/nonexistent/path/to/file.zip")
	if err == nil {
		t.Error("ComputeSHA256 should fail for non-existent file")
	}
}

func TestComputeSHA256EmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an empty file
	emptyFile := filepath.Join(tempDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	hash, err := ComputeSHA256(emptyFile)
	if err != nil {
		t.Fatalf("ComputeSHA256 failed: %v", err)
	}

	// SHA256 of empty content is a known value
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("SHA256 of empty file = %q, expected %q", hash, expected)
	}
}

func TestManifestPathFormat(t *testing.T) {
	path := ManifestPath("/backups", "my-project")
	expected := filepath.Join("/backups", "my-project", "manifest.json")
	if path != expected {
		t.Errorf("ManifestPath = %q, expected %q", path, expected)
	}
}

func TestPruneRemoveError(t *testing.T) {
	// Skip on non-Unix or if running as root
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create project backup dir with read-only permission
	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create files to delete
	files := []string{"old1.zip", "old2.zip", "new1.zip"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(projectDir, f), []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Make directory read-only so Remove fails
	if err := os.Chmod(projectDir, 0555); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}
	defer os.Chmod(projectDir, 0755) // Restore for cleanup

	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{
			{File: "old1.zip"},
			{File: "old2.zip"},
			{File: "new1.zip"},
		},
	}

	// Prune should handle remove errors gracefully
	deleted, err := m.Prune(tempDir, 1)
	if err != nil {
		t.Fatalf("Prune should not fail: %v", err)
	}

	// Files couldn't be deleted due to permissions, but manifest was still pruned
	if len(deleted) != 0 {
		t.Errorf("Should have 0 deleted (permission denied), got %d", len(deleted))
	}

	// Manifest should still be pruned
	if len(m.Backups) != 1 {
		t.Errorf("Should have 1 backup remaining, got %d", len(m.Backups))
	}
}

func TestMultipleAddBackups(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{},
	}

	// Add multiple backups
	for i := 0; i < 5; i++ {
		m.AddBackup(BackupEntry{
			File:      filepath.Join("backup", string(rune('a'+i))+".zip"),
			SizeBytes: int64(i * 1024),
		})
	}

	if len(m.Backups) != 5 {
		t.Errorf("Expected 5 backups, got %d", len(m.Backups))
	}

	latest := m.LatestBackup()
	if latest == nil {
		t.Fatal("LatestBackup should not be nil")
	}
	// Last added should be the latest
	if latest.SizeBytes != 4*1024 {
		t.Errorf("Latest backup size = %d, expected %d", latest.SizeBytes, 4*1024)
	}
}
