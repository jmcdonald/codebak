package mocks

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcdonald/codebak/internal/config"
	"github.com/jmcdonald/codebak/internal/ports"
)

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

// ============================================================================
// MockFileSystem Tests
// ============================================================================

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

func TestMockFileSystemStat(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockFileSystem)
		path        string
		wantErr     bool
		wantSize    int64
		wantIsDir   bool
	}{
		{
			name: "stat from Stats map",
			setup: func(m *MockFileSystem) {
				m.Stats["/dir"] = &mockFileInfo{name: "dir", isDir: true, size: 4096}
			},
			path:      "/dir",
			wantErr:   false,
			wantIsDir: true,
			wantSize:  4096,
		},
		{
			name: "stat from Files map",
			setup: func(m *MockFileSystem) {
				m.Files["/file.txt"] = []byte("content")
			},
			path:     "/file.txt",
			wantErr:  false,
			wantSize: 7,
		},
		{
			name:    "stat non-existent",
			setup:   func(m *MockFileSystem) {},
			path:    "/nonexistent",
			wantErr: true,
		},
		{
			name: "stat with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error"] = errors.New("stat error")
			},
			path:    "/error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			info, err := m.Stat(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Stat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if info.Size() != tt.wantSize {
					t.Errorf("Size() = %d, want %d", info.Size(), tt.wantSize)
				}
				if info.IsDir() != tt.wantIsDir {
					t.Errorf("IsDir() = %v, want %v", info.IsDir(), tt.wantIsDir)
				}
			}
		})
	}
}

func TestMockFileSystemReadDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		path    string
		wantLen int
		wantErr bool
	}{
		{
			name: "read existing directory",
			setup: func(m *MockFileSystem) {
				m.Dirs["/projects"] = []os.DirEntry{
					&mockDirEntry{name: "p1", isDir: true},
					&mockDirEntry{name: "p2", isDir: true},
				}
			},
			path:    "/projects",
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "read non-existent directory",
			setup:   func(m *MockFileSystem) {},
			path:    "/nonexistent",
			wantErr: true,
		},
		{
			name: "read with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error"] = errors.New("readdir error")
			},
			path:    "/error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			entries, err := m.ReadDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && len(entries) != tt.wantLen {
				t.Errorf("ReadDir() returned %d entries, want %d", len(entries), tt.wantLen)
			}
		})
	}
}

func TestMockFileSystemMkdirAll(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		path    string
		wantErr bool
	}{
		{
			name:    "create directory",
			setup:   func(m *MockFileSystem) {},
			path:    "/new/dir",
			wantErr: false,
		},
		{
			name: "create with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error/dir"] = errors.New("mkdir error")
			},
			path:    "/error/dir",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			err := m.MkdirAll(tt.path, 0755)
			if (err != nil) != tt.wantErr {
				t.Errorf("MkdirAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify directory is marked as existing
				info, statErr := m.Stat(tt.path)
				if statErr != nil {
					t.Errorf("MkdirAll() did not create directory entry")
				} else if !info.IsDir() {
					t.Errorf("MkdirAll() created non-directory entry")
				}
			}
		})
	}
}

func TestMockFileSystemWriteFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		path    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "write new file",
			setup:   func(m *MockFileSystem) {},
			path:    "/new/file.txt",
			data:    []byte("content"),
			wantErr: false,
		},
		{
			name: "write with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error/file.txt"] = errors.New("write error")
			},
			path:    "/error/file.txt",
			data:    []byte("content"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			err := m.WriteFile(tt.path, tt.data, 0644)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify file content
				content, readErr := m.ReadFile(tt.path)
				if readErr != nil {
					t.Errorf("WriteFile() did not store content: %v", readErr)
				} else if string(content) != string(tt.data) {
					t.Errorf("WriteFile() stored %q, want %q", content, tt.data)
				}
			}
		})
	}
}

func TestMockFileSystemRemove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		path    string
		wantErr bool
	}{
		{
			name: "remove existing file",
			setup: func(m *MockFileSystem) {
				m.Files["/file.txt"] = []byte("content")
				m.Stats["/file.txt"] = &mockFileInfo{name: "file.txt"}
			},
			path:    "/file.txt",
			wantErr: false,
		},
		{
			name:    "remove non-existent file (no error)",
			setup:   func(m *MockFileSystem) {},
			path:    "/nonexistent",
			wantErr: false,
		},
		{
			name: "remove with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error.txt"] = errors.New("remove error")
			},
			path:    "/error.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			err := m.Remove(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Remove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify file is removed
				if _, ok := m.Files[tt.path]; ok {
					t.Error("Remove() did not delete file from Files map")
				}
				if _, ok := m.Stats[tt.path]; ok {
					t.Error("Remove() did not delete entry from Stats map")
				}
			}
		})
	}
}

func TestMockFileSystemRemoveAll(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*MockFileSystem)
		path         string
		wantErr      bool
		checkRemoved []string
	}{
		{
			name: "remove directory tree",
			setup: func(m *MockFileSystem) {
				m.Files["/dir/file1.txt"] = []byte("1")
				m.Files["/dir/sub/file2.txt"] = []byte("2")
				m.Files["/other/file.txt"] = []byte("3")
				m.Stats["/dir"] = &mockFileInfo{name: "dir", isDir: true}
				m.Stats["/dir/sub"] = &mockFileInfo{name: "sub", isDir: true}
			},
			path:         "/dir",
			wantErr:      false,
			checkRemoved: []string{"/dir/file1.txt", "/dir/sub/file2.txt"},
		},
		{
			name: "remove with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error"] = errors.New("removeall error")
			},
			path:    "/error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			err := m.RemoveAll(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				for _, path := range tt.checkRemoved {
					if _, ok := m.Files[path]; ok {
						t.Errorf("RemoveAll() did not remove %s", path)
					}
				}
				// Verify /other/file.txt still exists
				if _, ok := m.Files["/other/file.txt"]; !ok && tt.path == "/dir" {
					t.Error("RemoveAll() removed files outside the target path")
				}
			}
		})
	}
}

func TestMockFileSystemRename(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		oldpath string
		newpath string
		wantErr bool
	}{
		{
			name: "rename file",
			setup: func(m *MockFileSystem) {
				m.Files["/old.txt"] = []byte("content")
				m.Stats["/old.txt"] = &mockFileInfo{name: "old.txt", size: 7}
			},
			oldpath: "/old.txt",
			newpath: "/new.txt",
			wantErr: false,
		},
		{
			name: "rename with error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error.txt"] = errors.New("rename error")
			},
			oldpath: "/error.txt",
			newpath: "/dest.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			err := m.Rename(tt.oldpath, tt.newpath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Rename() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify old path no longer exists
				if _, ok := m.Files[tt.oldpath]; ok {
					t.Error("Rename() did not remove old path")
				}
				// Verify new path exists
				if _, ok := m.Files[tt.newpath]; !ok {
					t.Error("Rename() did not create new path")
				}
				// Verify stats are moved too
				if _, ok := m.Stats[tt.oldpath]; ok {
					t.Error("Rename() did not remove old stats")
				}
				if _, ok := m.Stats[tt.newpath]; !ok {
					t.Error("Rename() did not create new stats")
				}
			}
		})
	}
}

func TestMockFileSystemOpen(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockFileSystem)
		path    string
		wantErr bool
	}{
		{
			name: "open existing file",
			setup: func(m *MockFileSystem) {
				m.Files["/file.txt"] = []byte("content")
			},
			path:    "/file.txt",
			wantErr: false,
		},
		{
			name:    "open non-existent file",
			setup:   func(m *MockFileSystem) {},
			path:    "/nonexistent",
			wantErr: true,
		},
		{
			name: "open with error",
			setup: func(m *MockFileSystem) {
				m.Files["/error.txt"] = []byte("content")
				m.Errors["/error.txt"] = errors.New("open error")
			},
			path:    "/error.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			file, err := m.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify we got a valid file
				if file == nil {
					t.Error("Open() returned nil file without error")
				} else {
					file.Close()
				}
			}
		})
	}
}

func TestMockFileSystemCreate(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*MockFileSystem)
		path     string
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name:    "create new file",
			setup:   func(m *MockFileSystem) {},
			path:    "/new.txt",
			wantErr: true, // Create returns an error by design
			errCheck: func(err error) bool {
				return err.Error() == "mock Create returns nil *os.File - use WriteFile instead"
			},
		},
		{
			name: "create with injected error",
			setup: func(m *MockFileSystem) {
				m.Errors["/error.txt"] = errors.New("custom error")
			},
			path:    "/error.txt",
			wantErr: true,
			errCheck: func(err error) bool {
				return err.Error() == "custom error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			_, err := m.Create(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("Create() error = %v, did not match expected", err)
			}
			// For non-error injection case, verify file was added to Files map
			if tt.path == "/new.txt" {
				if _, ok := m.Files[tt.path]; !ok {
					t.Error("Create() did not add entry to Files map")
				}
			}
		})
	}
}

func TestMockFileSystemWalkEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockFileSystem)
		root        string
		walkFunc    func(path string, info os.FileInfo, err error) error
		wantErr     bool
		wantVisited int
	}{
		{
			name: "walk with SkipDir",
			setup: func(m *MockFileSystem) {
				m.WalkEntries = []WalkEntry{
					{Path: "/project/dir1", Info: &mockFileInfo{name: "dir1", isDir: true}},
					{Path: "/project/file.txt", Info: &mockFileInfo{name: "file.txt"}},
				}
			},
			root: "/project",
			walkFunc: func(path string, info os.FileInfo, err error) error {
				return filepath.SkipDir
			},
			wantErr:     false,
			wantVisited: 1,
		},
		{
			name: "walk with SkipAll",
			setup: func(m *MockFileSystem) {
				m.WalkEntries = []WalkEntry{
					{Path: "/project/file1.txt", Info: &mockFileInfo{name: "file1.txt"}},
					{Path: "/project/file2.txt", Info: &mockFileInfo{name: "file2.txt"}},
				}
			},
			root: "/project",
			walkFunc: func(path string, info os.FileInfo, err error) error {
				return filepath.SkipAll
			},
			wantErr:     false,
			wantVisited: 1,
		},
		{
			name: "walk with error",
			setup: func(m *MockFileSystem) {
				m.WalkEntries = []WalkEntry{
					{Path: "/project/file.txt", Info: &mockFileInfo{name: "file.txt"}},
				}
			},
			root: "/project",
			walkFunc: func(path string, info os.FileInfo, err error) error {
				return errors.New("walk error")
			},
			wantErr:     true,
			wantVisited: 1,
		},
		{
			name: "walk filters by root",
			setup: func(m *MockFileSystem) {
				m.WalkEntries = []WalkEntry{
					{Path: "/project/file.txt", Info: &mockFileInfo{name: "file.txt"}},
					{Path: "/other/file.txt", Info: &mockFileInfo{name: "file.txt"}},
				}
			},
			root: "/project",
			walkFunc: func(path string, info os.FileInfo, err error) error {
				return nil
			},
			wantErr:     false,
			wantVisited: 1,
		},
		{
			name: "walk with entry error",
			setup: func(m *MockFileSystem) {
				m.WalkEntries = []WalkEntry{
					{Path: "/project/file.txt", Info: &mockFileInfo{name: "file.txt"}, Err: errors.New("entry error")},
				}
			},
			root: "/project",
			walkFunc: func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				return nil
			},
			wantErr:     true,
			wantVisited: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockFileSystem()
			tt.setup(m)

			visited := 0
			originalWalkFunc := tt.walkFunc
			wrappedWalkFunc := func(path string, info os.FileInfo, err error) error {
				visited++
				return originalWalkFunc(path, info, err)
			}

			err := m.Walk(tt.root, wrappedWalkFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("Walk() error = %v, wantErr %v", err, tt.wantErr)
			}
			if visited != tt.wantVisited {
				t.Errorf("Walk() visited %d paths, want %d", visited, tt.wantVisited)
			}
		})
	}
}

// ============================================================================
// mockFileInfo Tests
// ============================================================================

func TestMockFileInfo(t *testing.T) {
	now := time.Now()
	info := &mockFileInfo{
		name:    "test.txt",
		size:    1024,
		mode:    0644,
		modTime: now,
		isDir:   false,
	}

	if info.Name() != "test.txt" {
		t.Errorf("Name() = %q, want %q", info.Name(), "test.txt")
	}
	if info.Size() != 1024 {
		t.Errorf("Size() = %d, want %d", info.Size(), 1024)
	}
	if info.Mode() != 0644 {
		t.Errorf("Mode() = %o, want %o", info.Mode(), 0644)
	}
	if !info.ModTime().Equal(now) {
		t.Errorf("ModTime() = %v, want %v", info.ModTime(), now)
	}
	if info.IsDir() {
		t.Error("IsDir() = true, want false")
	}
	if info.Sys() != nil {
		t.Error("Sys() should return nil")
	}

	// Test directory
	dirInfo := &mockFileInfo{name: "dir", isDir: true}
	if !dirInfo.IsDir() {
		t.Error("IsDir() = false, want true for directory")
	}
}

// ============================================================================
// mockFile Tests
// ============================================================================

func TestMockFile(t *testing.T) {
	content := []byte("hello world")
	f := &mockFile{name: "test.txt", content: content}

	// Test Stat
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("Stat().Name() = %q, want %q", info.Name(), "test.txt")
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("Stat().Size() = %d, want %d", info.Size(), len(content))
	}

	// Test Read
	buf := make([]byte, 5)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 5 {
		t.Errorf("Read() = %d, want 5", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Read() content = %q, want %q", buf, "hello")
	}

	// Read remaining
	buf = make([]byte, 10)
	n, err = f.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 6 {
		t.Errorf("Read() = %d, want 6", n)
	}
	if string(buf[:n]) != " world" {
		t.Errorf("Read() content = %q, want %q", buf[:n], " world")
	}

	// Read at EOF
	n, err = f.Read(buf)
	if err == nil || err.Error() != "EOF" {
		t.Errorf("Read() at EOF should return EOF error, got: %v", err)
	}
	if n != 0 {
		t.Errorf("Read() at EOF = %d, want 0", n)
	}

	// Test Close
	if err := f.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// ============================================================================
// mockDirEntry Tests
// ============================================================================

func TestMockDirEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   *mockDirEntry
		wantDir bool
		wantTyp fs.FileMode
	}{
		{
			name:    "directory entry",
			entry:   &mockDirEntry{name: "subdir", isDir: true},
			wantDir: true,
			wantTyp: fs.ModeDir,
		},
		{
			name:    "file entry",
			entry:   &mockDirEntry{name: "file.txt", isDir: false},
			wantDir: false,
			wantTyp: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.entry.Name() != tt.entry.name {
				t.Errorf("Name() = %q, want %q", tt.entry.Name(), tt.entry.name)
			}
			if tt.entry.IsDir() != tt.wantDir {
				t.Errorf("IsDir() = %v, want %v", tt.entry.IsDir(), tt.wantDir)
			}
			if tt.entry.Type() != tt.wantTyp {
				t.Errorf("Type() = %v, want %v", tt.entry.Type(), tt.wantTyp)
			}
			// Info returns nil, nil by default
			info, err := tt.entry.Info()
			if err != nil {
				t.Errorf("Info() error = %v", err)
			}
			if info != nil {
				t.Errorf("Info() = %v, want nil", info)
			}
		})
	}
}

// ============================================================================
// MockGitClient Tests
// ============================================================================

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

func TestMockGitClientIsRepo(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*MockGitClient)
		path   string
		want   bool
	}{
		{
			name: "is repo - true",
			setup: func(g *MockGitClient) {
				g.Repos["/my-repo"] = true
			},
			path: "/my-repo",
			want: true,
		},
		{
			name: "is repo - false",
			setup: func(g *MockGitClient) {
				g.Repos["/not-repo"] = false
			},
			path: "/not-repo",
			want: false,
		},
		{
			name:  "is repo - not in map",
			setup: func(g *MockGitClient) {},
			path:  "/unknown",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewMockGitClient()
			tt.setup(g)

			if got := g.IsRepo(tt.path); got != tt.want {
				t.Errorf("IsRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockGitClientGetHead(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*MockGitClient)
		path  string
		want  string
	}{
		{
			name: "head exists",
			setup: func(g *MockGitClient) {
				g.Heads["/repo"] = "abc123"
			},
			path: "/repo",
			want: "abc123",
		},
		{
			name:  "head not set",
			setup: func(g *MockGitClient) {},
			path:  "/repo",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewMockGitClient()
			tt.setup(g)

			if got := g.GetHead(tt.path); got != tt.want {
				t.Errorf("GetHead() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// MockArchiver Tests
// ============================================================================

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

func TestMockArchiverExtract(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockArchiver)
		zipPath string
		destDir string
		wantErr bool
	}{
		{
			name:    "extract success",
			setup:   func(a *MockArchiver) {},
			zipPath: "/backup.zip",
			destDir: "/dest",
			wantErr: false,
		},
		{
			name: "extract with error",
			setup: func(a *MockArchiver) {
				a.Errors["Extract"] = errors.New("extract error")
			},
			zipPath: "/backup.zip",
			destDir: "/dest",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewMockArchiver()
			tt.setup(a)

			err := a.Extract(tt.zipPath, tt.destDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Verify call was recorded
			if len(a.ExtractCalls) != 1 {
				t.Errorf("ExtractCalls = %d, want 1", len(a.ExtractCalls))
			}
		})
	}
}

func TestMockArchiverList(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*MockArchiver)
		zipPath  string
		wantLen  int
		wantErr  bool
	}{
		{
			name: "list with results",
			setup: func(a *MockArchiver) {
				a.ListResults["/backup.zip"] = map[string]ports.FileInfo{
					"file1.txt": {Size: 100},
					"file2.txt": {Size: 200},
				}
			},
			zipPath: "/backup.zip",
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "list empty archive",
			setup:   func(a *MockArchiver) {},
			zipPath: "/empty.zip",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "list with error",
			setup: func(a *MockArchiver) {
				a.Errors["List"] = errors.New("list error")
			},
			zipPath: "/backup.zip",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewMockArchiver()
			tt.setup(a)

			files, err := a.List(tt.zipPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(files) != tt.wantLen {
				t.Errorf("List() returned %d files, want %d", len(files), tt.wantLen)
			}
		})
	}
}

func TestMockArchiverReadFile(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockArchiver)
		zipPath     string
		filePath    string
		projectName string
		want        string
		wantErr     bool
	}{
		{
			name: "read existing file",
			setup: func(a *MockArchiver) {
				a.ReadResults["/backup.zip:file.txt"] = "file content"
			},
			zipPath:     "/backup.zip",
			filePath:    "file.txt",
			projectName: "project",
			want:        "file content",
			wantErr:     false,
		},
		{
			name:        "read non-existent file",
			setup:       func(a *MockArchiver) {},
			zipPath:     "/backup.zip",
			filePath:    "missing.txt",
			projectName: "project",
			want:        "",
			wantErr:     false,
		},
		{
			name: "read with error",
			setup: func(a *MockArchiver) {
				a.Errors["ReadFile"] = errors.New("read error")
			},
			zipPath:     "/backup.zip",
			filePath:    "file.txt",
			projectName: "project",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewMockArchiver()
			tt.setup(a)

			content, err := a.ReadFile(tt.zipPath, tt.filePath, tt.projectName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && content != tt.want {
				t.Errorf("ReadFile() = %q, want %q", content, tt.want)
			}
		})
	}
}

// ============================================================================
// MockLaunchdService Tests
// ============================================================================

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

func TestMockLaunchdServicePaths(t *testing.T) {
	launchd := NewMockLaunchdService()

	// Test default paths
	if launchd.PlistPath() != "/tmp/mock.plist" {
		t.Errorf("PlistPath() = %q, want %q", launchd.PlistPath(), "/tmp/mock.plist")
	}
	if launchd.LogPath() != "/tmp/mock.log" {
		t.Errorf("LogPath() = %q, want %q", launchd.LogPath(), "/tmp/mock.log")
	}

	// Test custom paths
	launchd.PlistPathResult = "/custom/path.plist"
	launchd.LogPathResult = "/custom/log.log"

	if launchd.PlistPath() != "/custom/path.plist" {
		t.Errorf("PlistPath() = %q, want %q", launchd.PlistPath(), "/custom/path.plist")
	}
	if launchd.LogPath() != "/custom/log.log" {
		t.Errorf("LogPath() = %q, want %q", launchd.LogPath(), "/custom/log.log")
	}
}

func TestMockLaunchdServiceUninstallError(t *testing.T) {
	launchd := NewMockLaunchdService()
	launchd.Errors["Uninstall"] = errors.New("uninstall error")

	err := launchd.Uninstall()
	if err == nil || err.Error() != "uninstall error" {
		t.Errorf("Uninstall() error = %v, want 'uninstall error'", err)
	}
}

func TestMockLaunchdServiceInstallCall(t *testing.T) {
	launchd := NewMockLaunchdService()

	err := launchd.Install("/bin/codebak", "/etc/config.yaml", 14, 30)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if len(launchd.InstallCalls) != 1 {
		t.Fatalf("InstallCalls = %d, want 1", len(launchd.InstallCalls))
	}

	call := launchd.InstallCalls[0]
	if call.ExecPath != "/bin/codebak" {
		t.Errorf("ExecPath = %q, want %q", call.ExecPath, "/bin/codebak")
	}
	if call.ConfigPath != "/etc/config.yaml" {
		t.Errorf("ConfigPath = %q, want %q", call.ConfigPath, "/etc/config.yaml")
	}
	if call.Hour != 14 {
		t.Errorf("Hour = %d, want %d", call.Hour, 14)
	}
	if call.Minute != 30 {
		t.Errorf("Minute = %d, want %d", call.Minute, 30)
	}
}

// ============================================================================
// MockTUIService Tests
// ============================================================================

func TestNewMockTUIService(t *testing.T) {
	svc := NewMockTUIService()

	if svc == nil {
		t.Fatal("NewMockTUIService() returned nil")
	}
	if svc.ConfigResult == nil {
		t.Error("ConfigResult should be initialized")
	}
	if svc.Versions == nil {
		t.Error("Versions map should be initialized")
	}
	if svc.BackupResults == nil {
		t.Error("BackupResults map should be initialized")
	}
	if svc.VerifyErrors == nil {
		t.Error("VerifyErrors map should be initialized")
	}
}

func TestMockTUIServiceLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockTUIService)
		wantErr bool
	}{
		{
			name: "load config success",
			setup: func(s *MockTUIService) {
				s.ConfigResult = &config.Config{
					BackupDir: "/backups",
				}
			},
			wantErr: false,
		},
		{
			name: "load config error",
			setup: func(s *MockTUIService) {
				s.ConfigError = errors.New("config error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockTUIService()
			tt.setup(svc)

			cfg, err := svc.LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && cfg == nil {
				t.Error("LoadConfig() returned nil config without error")
			}
			if svc.LoadConfigCalls != 1 {
				t.Errorf("LoadConfigCalls = %d, want 1", svc.LoadConfigCalls)
			}
		})
	}
}

func TestMockTUIServiceListProjects(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*MockTUIService)
		wantLen  int
		wantErr  bool
	}{
		{
			name: "list projects success",
			setup: func(s *MockTUIService) {
				s.Projects = []ports.TUIProjectInfo{
					{Name: "project1", Path: "/p1"},
					{Name: "project2", Path: "/p2"},
				}
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "list projects empty",
			setup:   func(s *MockTUIService) {},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "list projects error",
			setup: func(s *MockTUIService) {
				s.ProjectsError = errors.New("projects error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockTUIService()
			tt.setup(svc)

			projects, err := svc.ListProjects(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListProjects() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(projects) != tt.wantLen {
				t.Errorf("ListProjects() returned %d projects, want %d", len(projects), tt.wantLen)
			}
			if svc.ListProjectsCalls != 1 {
				t.Errorf("ListProjectsCalls = %d, want 1", svc.ListProjectsCalls)
			}
		})
	}
}

func TestMockTUIServiceListVersions(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		setup   func(*MockTUIService)
		project string
		wantLen int
		wantErr bool
	}{
		{
			name: "list versions success",
			setup: func(s *MockTUIService) {
				s.Versions["project1"] = []ports.TUIVersionInfo{
					{File: "v1.zip", Size: 1024, CreatedAt: now},
					{File: "v2.zip", Size: 2048, CreatedAt: now},
				}
			},
			project: "project1",
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "list versions empty",
			setup:   func(s *MockTUIService) {},
			project: "project2",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "list versions error",
			setup: func(s *MockTUIService) {
				s.VersionsError = errors.New("versions error")
			},
			project: "project1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockTUIService()
			tt.setup(svc)

			versions, err := svc.ListVersions(nil, tt.project)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVersions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(versions) != tt.wantLen {
				t.Errorf("ListVersions() returned %d versions, want %d", len(versions), tt.wantLen)
			}
			// Verify call tracking
			found := false
			for _, p := range svc.ListVersionsCalls {
				if p == tt.project {
					found = true
					break
				}
			}
			if !found {
				t.Error("ListVersionsCalls did not record the call")
			}
		})
	}
}

func TestMockTUIServiceRunBackup(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*MockTUIService)
		project     string
		wantSize    int64
		wantSkipped bool
		wantErr     bool
	}{
		{
			name: "run backup success",
			setup: func(s *MockTUIService) {
				s.BackupResults["project1"] = ports.TUIBackupResult{
					Size: 5000,
				}
			},
			project:  "project1",
			wantSize: 5000,
		},
		{
			name: "run backup skipped",
			setup: func(s *MockTUIService) {
				s.BackupResults["project2"] = ports.TUIBackupResult{
					Skipped: true,
					Reason:  "no changes",
				}
			},
			project:     "project2",
			wantSkipped: true,
		},
		{
			name: "run backup error",
			setup: func(s *MockTUIService) {
				s.BackupResults["project3"] = ports.TUIBackupResult{
					Error: errors.New("backup failed"),
				}
			},
			project: "project3",
			wantErr: true,
		},
		{
			name:     "run backup default",
			setup:    func(s *MockTUIService) {},
			project:  "unknown",
			wantSize: 1024, // Default size
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockTUIService()
			tt.setup(svc)

			result := svc.RunBackup(nil, tt.project)
			if tt.wantErr && result.Error == nil {
				t.Error("RunBackup() should have returned error")
			}
			if !tt.wantErr && !tt.wantSkipped && result.Size != tt.wantSize {
				t.Errorf("RunBackup() size = %d, want %d", result.Size, tt.wantSize)
			}
			if result.Skipped != tt.wantSkipped {
				t.Errorf("RunBackup() skipped = %v, want %v", result.Skipped, tt.wantSkipped)
			}
			// Verify call tracking
			found := false
			for _, p := range svc.RunBackupCalls {
				if p == tt.project {
					found = true
					break
				}
			}
			if !found {
				t.Error("RunBackupCalls did not record the call")
			}
		})
	}
}

func TestMockTUIServiceVerifyBackup(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockTUIService)
		project string
		wantErr bool
	}{
		{
			name:    "verify success",
			setup:   func(s *MockTUIService) {},
			project: "project1",
			wantErr: false,
		},
		{
			name: "verify error",
			setup: func(s *MockTUIService) {
				s.VerifyErrors["project2"] = errors.New("verify failed")
			},
			project: "project2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewMockTUIService()
			tt.setup(svc)

			err := svc.VerifyBackup(nil, tt.project)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyBackup() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Verify call tracking
			found := false
			for _, p := range svc.VerifyBackupCalls {
				if p == tt.project {
					found = true
					break
				}
			}
			if !found {
				t.Error("VerifyBackupCalls did not record the call")
			}
		})
	}
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestInterfaceCompliance(t *testing.T) {
	// These compile-time checks ensure our mocks implement the interfaces
	var _ ports.FileSystem = (*MockFileSystem)(nil)
	var _ ports.GitClient = (*MockGitClient)(nil)
	var _ ports.Archiver = (*MockArchiver)(nil)
	var _ ports.LaunchdService = (*MockLaunchdService)(nil)
	var _ ports.TUIService = (*MockTUIService)(nil)
	var _ fs.File = (*mockFile)(nil)
	var _ os.FileInfo = (*mockFileInfo)(nil)
	var _ os.DirEntry = (*mockDirEntry)(nil)
}

// Helper to ensure Read properly implements io.Reader behavior
func TestMockFileImplementsReader(t *testing.T) {
	content := []byte("test content for reader")
	f := &mockFile{name: "test.txt", content: content}

	// Read all content using io.ReadAll pattern
	var result []byte
	buf := make([]byte, 4)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	if string(result) != string(content) {
		t.Errorf("Read() accumulated %q, want %q", result, content)
	}
}

// Test that io.Copy works with mockFile
func TestMockFileWithIOCopy(t *testing.T) {
	content := []byte("content for io.Copy test")
	f := &mockFile{name: "test.txt", content: content}

	// Create a buffer to copy to
	var buf []byte
	for {
		b := make([]byte, 8)
		n, err := f.Read(b)
		if n > 0 {
			buf = append(buf, b[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			// Note: io.Copy would handle io.EOF differently
			break
		}
	}

	if string(buf) != string(content) {
		t.Errorf("io.Copy result = %q, want %q", buf, content)
	}
}
