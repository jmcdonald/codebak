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

	// None of these files exist on disk: they are phantom entries and must
	// never count toward keepLast, so nothing is a candidate for removal
	// and the manifest is left untouched.
	deleted, err := m.Prune(tempDir, 1)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if len(deleted) != 0 {
		t.Errorf("phantom entries must never be reported as deleted, got %v", deleted)
	}
	if len(m.Backups) != 3 {
		t.Errorf("phantom-only manifest must be left untouched, got %d backups", len(m.Backups))
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

	// A failed real removal must leave the manifest entry in place rather
	// than dropping it and orphaning the file on disk.
	if len(m.Backups) != 3 {
		t.Errorf("manifest entries must survive a failed removal, got %d backups", len(m.Backups))
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

// ============================================================================
// Retention manifest tests: dry-run mode and the total-size cap.
// ============================================================================

func TestPruneWithOptionsDryRunDeletesNothing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	files := []string{
		"20241213-100000.zip",
		"20241214-100000.zip",
		"20241215-100000.zip",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(projectDir, f), []byte("dummy"), 0644); err != nil {
			t.Fatalf("Failed to create backup file: %v", err)
		}
	}

	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{
			{File: "20241213-100000.zip"},
			{File: "20241214-100000.zip"},
			{File: "20241215-100000.zip"},
		},
	}

	wouldDelete, err := m.PruneWithOptions(tempDir, 1, true)
	if err != nil {
		t.Fatalf("PruneWithOptions dry-run failed: %v", err)
	}

	if len(wouldDelete) != 2 {
		t.Errorf("wouldDelete count = %d, expected 2", len(wouldDelete))
	}

	// Manifest must be untouched.
	if len(m.Backups) != 3 {
		t.Errorf("dry-run must not mutate the manifest, got %d backups", len(m.Backups))
	}

	// No file may have been deleted.
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(projectDir, f)); err != nil {
			t.Errorf("dry-run must not delete %s from disk: %v", f, err)
		}
	}
}

func TestPruneByTotalSizeRemovesOldestFirst(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Three backups, 100 bytes each on disk. The cap is measured from the
	// real file on disk, not from SizeBytes, so SizeBytes is deliberately
	// left wrong here to prove the on-disk stat is what actually drives
	// the decision (fix for the phantom-entry over-deletion bug).
	entries := []BackupEntry{
		{File: "oldest.zip", SizeBytes: 10 * 1024 * 1024 * 1024},
		{File: "middle.zip", SizeBytes: 10 * 1024 * 1024 * 1024},
		{File: "newest.zip", SizeBytes: 10 * 1024 * 1024 * 1024},
	}
	content := make([]byte, 100)
	for _, e := range entries {
		if err := os.WriteFile(filepath.Join(projectDir, e.File), content, 0644); err != nil {
			t.Fatalf("Failed to create backup file: %v", err)
		}
	}

	m := &Manifest{Project: "test-project", Backups: entries}

	// Cap at 200 bytes: on-disk total is 300 bytes, so exactly the oldest
	// (100 bytes) must go to bring the remaining two (200 bytes) at or
	// under the cap.
	maxBytes := int64(200)
	deleted, err := m.PruneByTotalSize(tempDir, maxBytes, false)
	if err != nil {
		t.Fatalf("PruneByTotalSize failed: %v", err)
	}

	if len(deleted) != 1 || deleted[0] != "oldest.zip" {
		t.Errorf("deleted = %v, expected [oldest.zip]", deleted)
	}
	if len(m.Backups) != 2 {
		t.Fatalf("remaining backups = %d, expected 2", len(m.Backups))
	}
	if m.Backups[len(m.Backups)-1].File != "newest.zip" {
		t.Error("newest.zip must never be removed by size-based pruning")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "oldest.zip")); err == nil {
		t.Error("oldest.zip should have been deleted from disk")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "newest.zip")); err != nil {
		t.Error("newest.zip should still exist on disk")
	}
}

func TestPruneByTotalSizeNeverRemovesLastEntry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// A single backup that alone exceeds the cap must survive: there is
	// nothing older to prune, and we never delete the only/newest copy.
	if err := os.WriteFile(filepath.Join(projectDir, "only.zip"), []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}
	m := &Manifest{
		Project: "test-project",
		Backups: []BackupEntry{
			{File: "only.zip", SizeBytes: 100 * 1024 * 1024 * 1024},
		},
	}

	deleted, err := m.PruneByTotalSize(tempDir, 50*1024*1024*1024, false)
	if err != nil {
		t.Fatalf("PruneByTotalSize failed: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected no deletions when only one backup exists, got %v", deleted)
	}
	if len(m.Backups) != 1 {
		t.Error("the sole backup must not be removed")
	}
}

func TestPruneByTotalSizeDryRunDeletesNothing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	entries := []BackupEntry{
		{File: "oldest.zip", SizeBytes: 10 * 1024 * 1024 * 1024},
		{File: "newest.zip", SizeBytes: 10 * 1024 * 1024 * 1024},
	}
	content := make([]byte, 100)
	for _, e := range entries {
		if err := os.WriteFile(filepath.Join(projectDir, e.File), content, 0644); err != nil {
			t.Fatalf("Failed to create backup file: %v", err)
		}
	}
	m := &Manifest{Project: "test-project", Backups: entries}

	// On-disk total is 200 bytes; a 50-byte cap would normally demand
	// removing both, but the newest must always survive.
	wouldDelete, err := m.PruneByTotalSize(tempDir, 50, true)
	if err != nil {
		t.Fatalf("PruneByTotalSize dry-run failed: %v", err)
	}
	if len(wouldDelete) != 1 || wouldDelete[0] != "oldest.zip" {
		t.Errorf("wouldDelete = %v, expected [oldest.zip]", wouldDelete)
	}
	if len(m.Backups) != 2 {
		t.Error("dry-run must not mutate the manifest")
	}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(projectDir, e.File)); err != nil {
			t.Errorf("dry-run must not delete %s from disk: %v", e.File, err)
		}
	}
}

func TestPruneByTotalSizeDisabledWhenZero(t *testing.T) {
	m := &Manifest{
		Project: "test",
		Backups: []BackupEntry{
			{File: "a.zip", SizeBytes: 999 * 1024 * 1024 * 1024},
			{File: "b.zip", SizeBytes: 999 * 1024 * 1024 * 1024},
		},
	}

	deleted, err := m.PruneByTotalSize("/tmp", 0, false)
	if err != nil {
		t.Fatalf("PruneByTotalSize failed: %v", err)
	}
	if len(deleted) != 0 {
		t.Error("a cap of 0 must mean disabled, not zero-tolerance")
	}
	if len(m.Backups) != 2 {
		t.Error("backups must be unchanged when the cap is disabled")
	}
}

// TestPruneByTotalSizePhantomEntriesNeverDeleteRealFile exercises a
// manifest with three "phantom" entries whose zip files were already
// deleted by an earlier run, each recorded at 100 GB, plus one real 4 KB
// zip that is actually on disk, under a 50 GB cap. Before this behavior
// was fixed, PruneByTotalSize summed the recorded size_bytes (300 GB)
// regardless of whether the file existed, decided the cap was blown, and
// deleted the one real file to "make room" for phantom entries that were
// not consuming any disk space at all. This must fail against the old
// (size_bytes-summing) behavior and pass once the cap is driven by
// on-disk stat only.
func TestPruneByTotalSizePhantomEntriesNeverDeleteRealFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	const oneHundredGB = 100 * 1024 * 1024 * 1024

	// Backups are ordered oldest to newest. The real zip is the OLDEST
	// entry here on purpose: the bug this test guards against deleted the
	// real (older) zip to make room for newer phantom entries whose
	// recorded size_bytes inflated the total, even though those phantom
	// files no longer exist on disk at all.
	entries := []BackupEntry{
		// Real: actually present on disk, 4 KB, and the oldest entry.
		{File: "real.zip", SizeBytes: 4096},
		// Phantom: recorded in the manifest, but the zip is gone.
		{File: "phantom-1.zip", SizeBytes: oneHundredGB},
		{File: "phantom-2.zip", SizeBytes: oneHundredGB},
		{File: "phantom-3.zip", SizeBytes: oneHundredGB},
	}

	realContent := make([]byte, 4096)
	if err := os.WriteFile(filepath.Join(projectDir, "real.zip"), realContent, 0644); err != nil {
		t.Fatalf("Failed to create real.zip: %v", err)
	}
	// Deliberately do NOT create phantom-1/2/3.zip: their manifest entries
	// outlive the file, exactly like a store where an earlier prune or a
	// manual delete removed the zip but never touched the manifest.

	m := &Manifest{Project: "test-project", Backups: entries}

	maxBytes := int64(50) * 1024 * 1024 * 1024 // 50 GB
	deleted, err := m.PruneByTotalSize(tempDir, maxBytes, false)
	if err != nil {
		t.Fatalf("PruneByTotalSize failed: %v", err)
	}

	for _, f := range deleted {
		if f == "real.zip" {
			t.Fatalf("real.zip must never be deleted: phantom entries recorded 300 GB but on-disk usage is 4 KB, well under the 50 GB cap; deleted = %v", deleted)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, "real.zip")); err != nil {
		t.Errorf("real.zip should still exist on disk after pruning: %v", err)
	}

	foundReal := false
	for _, e := range m.Backups {
		if e.File == "real.zip" {
			foundReal = true
		}
	}
	if !foundReal {
		t.Error("real.zip must still be present in the manifest after pruning")
	}
}

// TestPruneWithOptionsPhantomEntriesNeverDeleteRealFile exercises one real
// 4 KB zip (the oldest entry) plus four newer "phantom" entries whose zips
// are already gone, under keep_last 5. A sixth entry (a newly written real
// backup) represents the backup just completed by this run, so the
// manifest has 6 entries against keep_last 5. Before this behavior was
// fixed, len(m.Backups) counted the phantom entries as if they were real,
// so PruneWithOptions treated the count as 6-over-5 and evicted the
// oldest entry, the one real older backup, leaving only two real backups
// on disk. A fixture with exactly 5 entries against keep_last 5 would
// never even attempt a prune (len(m.Backups) <= keepLast short-circuits),
// so the sixth entry is required to actually exercise the old bug.
// Phantom entries must never occupy a keep slot or be picked as a
// removal candidate.
func TestPruneWithOptionsPhantomEntriesNeverDeleteRealFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectDir := filepath.Join(tempDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Backups are ordered oldest to newest. The real zip is the OLDEST
	// entry: a project whose old real backup predates four later runs
	// that got pruned (or hand-deleted)
	// without their manifest entries ever being cleaned up. newest.zip is
	// the backup just written by this run, giving 6 total entries against
	// keep_last 5.
	entries := []BackupEntry{
		{File: "real.zip"},
		{File: "phantom-1.zip"},
		{File: "phantom-2.zip"},
		{File: "phantom-3.zip"},
		{File: "phantom-4.zip"},
		{File: "newest.zip"},
	}

	if err := os.WriteFile(filepath.Join(projectDir, "real.zip"), []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create real.zip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "newest.zip"), []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create newest.zip: %v", err)
	}
	// phantom-1..4.zip are deliberately never created.

	m := &Manifest{Project: "test-project", Backups: entries}

	deleted, err := m.PruneWithOptions(tempDir, 5, false)
	if err != nil {
		t.Fatalf("PruneWithOptions failed: %v", err)
	}

	for _, f := range deleted {
		if f == "real.zip" {
			t.Fatalf("real.zip must never be deleted: only two real backups exist, well under keep_last 5; deleted = %v", deleted)
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, "real.zip")); err != nil {
		t.Errorf("real.zip should still exist on disk after pruning: %v", err)
	}

	foundReal := false
	for _, e := range m.Backups {
		if e.File == "real.zip" {
			foundReal = true
		}
	}
	if !foundReal {
		t.Error("real.zip must still be present in the manifest after pruning")
	}
}
