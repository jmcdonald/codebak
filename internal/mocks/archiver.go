package mocks

import (
	"github.com/jmcdonald/codebak/internal/ports"
)

// MockArchiver implements ports.Archiver for testing.
type MockArchiver struct {
	// CreateCalls records calls to Create
	CreateCalls []CreateCall
	// ExtractCalls records calls to Extract
	ExtractCalls []ExtractCall
	// ListResults maps zip paths to file listings
	ListResults map[string]map[string]ports.FileInfo
	// ReadResults maps "zipPath:filePath" to content
	ReadResults map[string]string
	// Errors maps method calls to errors
	Errors map[string]error
	// CreateResult is the default file count to return
	CreateResult int
}

// CreateCall records parameters of a Create call.
type CreateCall struct {
	DestPath  string
	SourceDir string
	Exclude   []string
}

// ExtractCall records parameters of an Extract call.
type ExtractCall struct {
	ZipPath string
	DestDir string
}

// NewMockArchiver creates a new mock archiver.
func NewMockArchiver() *MockArchiver {
	return &MockArchiver{
		ListResults:  make(map[string]map[string]ports.FileInfo),
		ReadResults:  make(map[string]string),
		Errors:       make(map[string]error),
		CreateResult: 1, // Default to 1 file
	}
}

// Create creates a zip archive of sourceDir at destPath.
// Returns the number of files archived.
func (m *MockArchiver) Create(destPath, sourceDir string, exclude []string) (int, error) {
	m.CreateCalls = append(m.CreateCalls, CreateCall{
		DestPath:  destPath,
		SourceDir: sourceDir,
		Exclude:   exclude,
	})
	if err, ok := m.Errors["Create"]; ok {
		return 0, err
	}
	return m.CreateResult, nil
}

// Extract extracts a zip archive to destDir.
func (m *MockArchiver) Extract(zipPath, destDir string) error {
	m.ExtractCalls = append(m.ExtractCalls, ExtractCall{
		ZipPath: zipPath,
		DestDir: destDir,
	})
	if err, ok := m.Errors["Extract"]; ok {
		return err
	}
	return nil
}

// List returns a map of file paths to their info from the archive.
func (m *MockArchiver) List(zipPath string) (map[string]ports.FileInfo, error) {
	if err, ok := m.Errors["List"]; ok {
		return nil, err
	}
	if result, ok := m.ListResults[zipPath]; ok {
		return result, nil
	}
	return make(map[string]ports.FileInfo), nil
}

// ReadFile reads the contents of a file from inside a zip archive.
func (m *MockArchiver) ReadFile(zipPath, filePath, projectName string) (string, error) {
	key := zipPath + ":" + filePath
	if err, ok := m.Errors["ReadFile"]; ok {
		return "", err
	}
	if content, ok := m.ReadResults[key]; ok {
		return content, nil
	}
	return "", nil
}

// Compile-time check that MockArchiver implements ports.Archiver.
var _ ports.Archiver = (*MockArchiver)(nil)
