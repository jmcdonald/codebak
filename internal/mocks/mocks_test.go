package mocks

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/mcdonaldj/codebak/internal/ports"
)

func TestMockFileSystem(t *testing.T) {
	mockFS := NewMockFileSystem()

	// Test WriteFile and ReadFile
	mockFS.WriteFile("/test/file.txt", []byte("hello"), 0644)
	content, err := mockFS.ReadFile("/test/file.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content = %q, expected %q", string(content), "hello")
	}

	// Test Stat after WriteFile
	info, err := mockFS.Stat("/test/file.txt")
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != 5 {
		t.Errorf("size = %d, expected 5", info.Size())
	}

	// Test ReadFile for non-existent file
	_, err = mockFS.ReadFile("/nonexistent")
	if err == nil {
		t.Error("ReadFile should fail for non-existent file")
	}

	// Test error injection
	mockFS.Errors["/error/path"] = errors.New("injected error")
	_, err = mockFS.ReadFile("/error/path")
	if err == nil || err.Error() != "injected error" {
		t.Errorf("Expected injected error, got: %v", err)
	}
}

func TestMockFileSystemDirEntry(t *testing.T) {
	mockFS := NewMockFileSystem()

	// Setup directory entries using os.DirEntry slice
	mockFS.Dirs["/projects"] = []os.DirEntry{
		&mockDirEntry{name: "project-a", isDir: true},
		&mockDirEntry{name: "project-b", isDir: true},
		&mockDirEntry{name: ".hidden", isDir: true},
	}

	entries, err := mockFS.ReadDir("/projects")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("ReadDir returned %d entries, expected 3", len(entries))
	}
}

func TestMockFileSystemWalk(t *testing.T) {
	mockFS := NewMockFileSystem()

	// Setup walk entries
	mockFS.WalkEntries = []WalkEntry{
		{Path: "/project/file1.txt", Info: &mockFileInfo{name: "file1.txt", modTime: time.Now()}},
		{Path: "/project/file2.txt", Info: &mockFileInfo{name: "file2.txt", modTime: time.Now()}},
	}

	var visited []string
	err := mockFS.Walk("/project", func(path string, info os.FileInfo, err error) error {
		visited = append(visited, path)
		return nil
	})

	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if len(visited) != 2 {
		t.Errorf("Walk visited %d paths, expected 2", len(visited))
	}
}

func TestMockGitClient(t *testing.T) {
	git := NewMockGitClient()

	// Test GetHead for non-repo
	head := git.GetHead("/not-a-repo")
	if head != "" {
		t.Errorf("GetHead should return empty for non-repo, got %q", head)
	}

	// Setup and test
	git.Repos["/my-repo"] = true
	git.Heads["/my-repo"] = "abc123def456"

	if !git.IsRepo("/my-repo") {
		t.Error("IsRepo should return true for configured repo")
	}

	head = git.GetHead("/my-repo")
	if head != "abc123def456" {
		t.Errorf("GetHead = %q, expected %q", head, "abc123def456")
	}
}

func TestMockArchiver(t *testing.T) {
	archiver := NewMockArchiver()
	archiver.CreateResult = 5

	// Test Create
	count, err := archiver.Create("/backup.zip", "/source", []string{"node_modules"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Create returned %d, expected 5", count)
	}
	if len(archiver.CreateCalls) != 1 {
		t.Errorf("CreateCalls = %d, expected 1", len(archiver.CreateCalls))
	}

	// Test Extract
	err = archiver.Extract("/backup.zip", "/dest")
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(archiver.ExtractCalls) != 1 {
		t.Errorf("ExtractCalls = %d, expected 1", len(archiver.ExtractCalls))
	}

	// Test List
	archiver.ListResults["/backup.zip"] = map[string]ports.FileInfo{
		"file1.txt": {Size: 100, CRC32: 12345},
	}
	files, err := archiver.List("/backup.zip")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("List returned %d files, expected 1", len(files))
	}

	// Test error injection
	archiver.Errors["Create"] = errors.New("disk full")
	_, err = archiver.Create("/another.zip", "/source", nil)
	if err == nil || err.Error() != "disk full" {
		t.Errorf("Expected 'disk full' error, got: %v", err)
	}
}

func TestMockLaunchdService(t *testing.T) {
	launchd := NewMockLaunchdService()

	// Test initial state
	if launchd.IsInstalled() {
		t.Error("Should not be installed initially")
	}
	if launchd.Status() != "not installed" {
		t.Errorf("Status = %q, expected %q", launchd.Status(), "not installed")
	}

	// Test Install
	err := launchd.Install("/usr/local/bin/codebak", "/config.yaml", 3, 0)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if !launchd.IsInstalled() {
		t.Error("Should be installed after Install()")
	}
	if launchd.Status() != "loaded" {
		t.Errorf("Status = %q, expected %q", launchd.Status(), "loaded")
	}
	if len(launchd.InstallCalls) != 1 {
		t.Errorf("InstallCalls = %d, expected 1", len(launchd.InstallCalls))
	}

	// Test Uninstall
	err = launchd.Uninstall()
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if launchd.IsInstalled() {
		t.Error("Should not be installed after Uninstall()")
	}

	// Test error injection
	launchd.Errors["Install"] = errors.New("permission denied")
	err = launchd.Install("/path", "/config", 3, 0)
	if err == nil || err.Error() != "permission denied" {
		t.Errorf("Expected 'permission denied' error, got: %v", err)
	}
}

// mockDirEntry implements os.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (d *mockDirEntry) Name() string      { return d.name }
func (d *mockDirEntry) IsDir() bool       { return d.isDir }
func (d *mockDirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}
func (d *mockDirEntry) Info() (fs.FileInfo, error) { return nil, nil }
