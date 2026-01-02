// Package osfs provides a filesystem adapter using the standard library os package.
package osfs

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jmcdonald/codebak/internal/ports"
)

// OSFileSystem implements ports.FileSystem using the standard library.
type OSFileSystem struct{}

// New creates a new OSFileSystem adapter.
func New() *OSFileSystem {
	return &OSFileSystem{}
}

// ReadDir reads the named directory and returns directory entries.
func (f *OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

// Stat returns file info for the named file.
func (f *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// MkdirAll creates a directory along with any necessary parents.
func (f *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFile writes data to the named file, creating it if necessary.
func (f *OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// ReadFile reads the named file and returns the contents.
func (f *OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Remove removes the named file or empty directory.
func (f *OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
func (f *OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Rename renames (moves) oldpath to newpath.
func (f *OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Open opens the named file for reading.
func (f *OSFileSystem) Open(name string) (fs.File, error) {
	return os.Open(name)
}

// Create creates or truncates the named file.
func (f *OSFileSystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

// Walk walks the file tree rooted at root, calling fn for each file or directory.
func (f *OSFileSystem) Walk(root string, fn ports.WalkFunc) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		return fn(path, info, err)
	})
}

// Compile-time check that OSFileSystem implements ports.FileSystem.
var _ ports.FileSystem = (*OSFileSystem)(nil)
