// Package ports defines interfaces (contracts) for external dependencies.
// These enable dependency injection and testability via mock implementations.
package ports

import (
	"io/fs"
	"os"
)

// FileSystem abstracts filesystem operations for testability.
// Production code uses OSFileSystem adapter; tests use MockFileSystem.
type FileSystem interface {
	// ReadDir reads the named directory and returns directory entries.
	ReadDir(name string) ([]os.DirEntry, error)

	// Stat returns file info for the named file.
	Stat(name string) (os.FileInfo, error)

	// MkdirAll creates a directory along with any necessary parents.
	MkdirAll(path string, perm os.FileMode) error

	// WriteFile writes data to the named file, creating it if necessary.
	WriteFile(name string, data []byte, perm os.FileMode) error

	// ReadFile reads the named file and returns the contents.
	ReadFile(name string) ([]byte, error)

	// Remove removes the named file or empty directory.
	Remove(name string) error

	// RemoveAll removes path and any children it contains.
	RemoveAll(path string) error

	// Rename renames (moves) oldpath to newpath.
	Rename(oldpath, newpath string) error

	// Open opens the named file for reading.
	Open(name string) (fs.File, error)

	// Create creates or truncates the named file.
	Create(name string) (*os.File, error)

	// Walk walks the file tree rooted at root, calling fn for each file or directory.
	Walk(root string, fn WalkFunc) error
}

// WalkFunc is the type of function called by Walk.
type WalkFunc func(path string, info os.FileInfo, err error) error
