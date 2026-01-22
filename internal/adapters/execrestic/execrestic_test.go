package execrestic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcdonald/codebak/internal/ports"
)

func TestNew(t *testing.T) {
	t.Run("default restic path", func(t *testing.T) {
		client := New()
		if client.resticPath != "restic" {
			t.Errorf("expected default restic path 'restic', got %q", client.resticPath)
		}
	})

	t.Run("custom restic path", func(t *testing.T) {
		client := New(WithResticPath("/usr/local/bin/restic"))
		if client.resticPath != "/usr/local/bin/restic" {
			t.Errorf("expected custom path, got %q", client.resticPath)
		}
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("not initialized - missing config", func(t *testing.T) {
		tmpDir := t.TempDir()
		client := New()

		if client.IsInitialized(tmpDir) {
			t.Error("expected false for uninitialized repo")
		}
	})

	t.Run("not initialized - config is directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config")
		if err := os.Mkdir(configPath, 0755); err != nil {
			t.Fatal(err)
		}

		client := New()
		if client.IsInitialized(tmpDir) {
			t.Error("expected false when config is a directory")
		}
	})

	t.Run("initialized - config file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config")
		if err := os.WriteFile(configPath, []byte("restic config"), 0644); err != nil {
			t.Fatal(err)
		}

		client := New()
		if !client.IsInitialized(tmpDir) {
			t.Error("expected true when config file exists")
		}
	})
}

func TestBackupEmptyPaths(t *testing.T) {
	client := New()
	_, err := client.Backup("/repo", "password", []string{}, nil)
	if err == nil {
		t.Error("expected error for empty paths")
	}
	if err.Error() != "no paths specified for backup" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestImplementsInterface(t *testing.T) {
	// This test verifies at compile time that ExecResticClient implements the interface.
	// The var _ declaration in the main file does this too, but this makes it explicit in tests.
	var _ ports.ResticClient = (*ExecResticClient)(nil)
}

// Integration tests require restic to be installed.
// Run with: go test -tags=integration ./internal/adapters/execrestic/...

func TestIntegrationInit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if restic is available
	client := New()
	cmd := client.command("version")
	if err := cmd.Run(); err != nil {
		t.Skip("restic not installed, skipping integration test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	password := "test-password"

	// Init should succeed
	err := client.Init(repoPath, password)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify repo was created
	if !client.IsInitialized(repoPath) {
		t.Error("repo should be initialized after Init")
	}

	// Init again should fail
	err = client.Init(repoPath, password)
	if err == nil {
		t.Error("expected error when re-initializing")
	}
}

func TestIntegrationBackupAndSnapshots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := New()
	cmd := client.command("version")
	if err := cmd.Run(); err != nil {
		t.Skip("restic not installed, skipping integration test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	dataDir := filepath.Join(tmpDir, "data")
	password := "test-password"

	// Create test data
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(dataDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Init repo
	if err := client.Init(repoPath, password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Backup
	snapshotID, err := client.Backup(repoPath, password, []string{dataDir}, []string{"test-tag"})
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	if snapshotID == "" {
		t.Error("expected non-empty snapshot ID")
	}

	// List snapshots
	snapshots, err := client.Snapshots(repoPath, password, nil)
	if err != nil {
		t.Fatalf("Snapshots failed: %v", err)
	}
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}

	// Filter by tag
	tagged, err := client.Snapshots(repoPath, password, []string{"test-tag"})
	if err != nil {
		t.Fatalf("Snapshots with tag failed: %v", err)
	}
	if len(tagged) != 1 {
		t.Errorf("expected 1 tagged snapshot, got %d", len(tagged))
	}

	// Filter by non-existent tag
	empty, err := client.Snapshots(repoPath, password, []string{"no-such-tag"})
	if err != nil {
		t.Fatalf("Snapshots with missing tag failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 snapshots with missing tag, got %d", len(empty))
	}
}

func TestIntegrationRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := New()
	cmd := client.command("version")
	if err := cmd.Run(); err != nil {
		t.Skip("restic not installed, skipping integration test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	dataDir := filepath.Join(tmpDir, "data")
	restoreDir := filepath.Join(tmpDir, "restore")
	password := "test-password"

	// Create test data
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(dataDir, "test.txt")
	testContent := []byte("test data for restore")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Init and backup
	if err := client.Init(repoPath, password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	_, err := client.Backup(repoPath, password, []string{dataDir}, nil)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Restore
	if err := client.Restore(repoPath, password, "latest", restoreDir); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify restored file exists (path will be restoreDir + original path)
	// Restic preserves the full path structure
	restoredFile := filepath.Join(restoreDir, dataDir, "test.txt")
	content, err := os.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("restored content mismatch: got %q, want %q", string(content), string(testContent))
	}
}

func TestIntegrationForget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := New()
	cmd := client.command("version")
	if err := cmd.Run(); err != nil {
		t.Skip("restic not installed, skipping integration test")
	}

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")
	dataDir := filepath.Join(tmpDir, "data")
	password := "test-password"

	// Create test data
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(dataDir, "test.txt")

	// Init repo
	if err := client.Init(repoPath, password); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create multiple snapshots
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(testFile, []byte{byte(i)}, 0644); err != nil {
			t.Fatal(err)
		}
		_, err := client.Backup(repoPath, password, []string{dataDir}, nil)
		if err != nil {
			t.Fatalf("Backup %d failed: %v", i, err)
		}
	}

	// Verify we have 3 snapshots
	snapshots, _ := client.Snapshots(repoPath, password, nil)
	if len(snapshots) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(snapshots))
	}

	// Forget all but last 1
	if err := client.Forget(repoPath, password, 1, false); err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	// Verify only 1 remains
	snapshots, _ = client.Snapshots(repoPath, password, nil)
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot after forget, got %d", len(snapshots))
	}
}
