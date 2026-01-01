// Package mocks provides mock implementations for testing.
package mocks

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mcdonaldj/codebak/internal/ports"
)

// MockFileSystem implements ports.FileSystem for testing.
type MockFileSystem struct {
	// Files maps paths to file contents for ReadFile/WriteFile
	Files map[string][]byte
	// Dirs maps paths to directory entries for ReadDir
	Dirs map[string][]os.DirEntry
	// Stats maps paths to FileInfo for Stat
	Stats map[string]os.FileInfo
	// Errors maps paths to errors (for simulating failures)
	Errors map[string]error
	// WalkEntries contains entries to return during Walk
	WalkEntries []WalkEntry
}

// WalkEntry represents a file or directory entry for Walk testing.
type WalkEntry struct {
	Path string
	Info os.FileInfo
	Err  error
}

// NewMockFileSystem creates a new mock filesystem.
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		Files:  make(map[string][]byte),
		Dirs:   make(map[string][]os.DirEntry),
		Stats:  make(map[string]os.FileInfo),
		Errors: make(map[string]error),
	}
}

// ReadDir reads the named directory and returns directory entries.
func (m *MockFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	if err, ok := m.Errors[name]; ok {
		return nil, err
	}
	if entries, ok := m.Dirs[name]; ok {
		return entries, nil
	}
	return nil, os.ErrNotExist
}

// Stat returns file info for the named file.
func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if err, ok := m.Errors[name]; ok {
		return nil, err
	}
	if info, ok := m.Stats[name]; ok {
		return info, nil
	}
	// Check if we have file content (implies file exists)
	if _, ok := m.Files[name]; ok {
		return &mockFileInfo{name: filepath.Base(name), size: int64(len(m.Files[name]))}, nil
	}
	return nil, os.ErrNotExist
}

// MkdirAll creates a directory along with any necessary parents.
func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err, ok := m.Errors[path]; ok {
		return err
	}
	// Mark directory as existing
	m.Stats[path] = &mockFileInfo{name: filepath.Base(path), isDir: true}
	return nil
}

// WriteFile writes data to the named file, creating it if necessary.
func (m *MockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	if err, ok := m.Errors[name]; ok {
		return err
	}
	m.Files[name] = data
	return nil
}

// ReadFile reads the named file and returns the contents.
func (m *MockFileSystem) ReadFile(name string) ([]byte, error) {
	if err, ok := m.Errors[name]; ok {
		return nil, err
	}
	if content, ok := m.Files[name]; ok {
		return content, nil
	}
	return nil, os.ErrNotExist
}

// Remove removes the named file or empty directory.
func (m *MockFileSystem) Remove(name string) error {
	if err, ok := m.Errors[name]; ok {
		return err
	}
	delete(m.Files, name)
	delete(m.Stats, name)
	return nil
}

// RemoveAll removes path and any children it contains.
func (m *MockFileSystem) RemoveAll(path string) error {
	if err, ok := m.Errors[path]; ok {
		return err
	}
	// Remove all entries with this prefix
	for k := range m.Files {
		if strings.HasPrefix(k, path) {
			delete(m.Files, k)
		}
	}
	for k := range m.Stats {
		if strings.HasPrefix(k, path) {
			delete(m.Stats, k)
		}
	}
	return nil
}

// Rename renames (moves) oldpath to newpath.
func (m *MockFileSystem) Rename(oldpath, newpath string) error {
	if err, ok := m.Errors[oldpath]; ok {
		return err
	}
	if content, ok := m.Files[oldpath]; ok {
		m.Files[newpath] = content
		delete(m.Files, oldpath)
	}
	if info, ok := m.Stats[oldpath]; ok {
		m.Stats[newpath] = info
		delete(m.Stats, oldpath)
	}
	return nil
}

// Open opens the named file for reading.
func (m *MockFileSystem) Open(name string) (fs.File, error) {
	if err, ok := m.Errors[name]; ok {
		return nil, err
	}
	if _, ok := m.Files[name]; !ok {
		return nil, os.ErrNotExist
	}
	return &mockFile{name: name, content: m.Files[name]}, nil
}

// Create creates or truncates the named file.
func (m *MockFileSystem) Create(name string) (*os.File, error) {
	// Note: For full mock functionality, consider using a temp file
	// This is a simplified mock that just returns nil without error
	if err, ok := m.Errors[name]; ok {
		return nil, err
	}
	m.Files[name] = []byte{}
	return nil, errors.New("mock Create returns nil *os.File - use WriteFile instead")
}

// Walk walks the file tree rooted at root, calling fn for each file or directory.
func (m *MockFileSystem) Walk(root string, fn ports.WalkFunc) error {
	for _, entry := range m.WalkEntries {
		if strings.HasPrefix(entry.Path, root) {
			if err := fn(entry.Path, entry.Info, entry.Err); err != nil {
				if err == filepath.SkipDir || err == filepath.SkipAll {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

// mockFileInfo implements os.FileInfo for testing.
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

// mockFile implements fs.File for testing.
type mockFile struct {
	name    string
	content []byte
	offset  int
}

func (f *mockFile) Stat() (fs.FileInfo, error) {
	return &mockFileInfo{name: f.name, size: int64(len(f.content))}, nil
}

func (f *mockFile) Read(p []byte) (int, error) {
	if f.offset >= len(f.content) {
		return 0, errors.New("EOF")
	}
	n := copy(p, f.content[f.offset:])
	f.offset += n
	return n, nil
}

func (f *mockFile) Close() error { return nil }

// Compile-time check that MockFileSystem implements ports.FileSystem.
var _ ports.FileSystem = (*MockFileSystem)(nil)
