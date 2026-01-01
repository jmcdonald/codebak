package ports

// Archiver abstracts zip archive operations for testability.
// Production code uses ZipArchiver adapter; tests use MockArchiver.
type Archiver interface {
	// Create creates a zip archive of sourceDir at destPath.
	// Returns the number of files archived.
	// exclude is a list of patterns to skip (e.g., "node_modules", "*.pyc").
	Create(destPath, sourceDir string, exclude []string) (fileCount int, err error)

	// Extract extracts a zip archive to destDir.
	Extract(zipPath, destDir string) error

	// List returns a map of file paths to their info from the archive.
	// The path key has the project prefix stripped.
	List(zipPath string) (map[string]FileInfo, error)

	// ReadFile reads the contents of a file from inside a zip archive.
	ReadFile(zipPath, filePath, projectName string) (string, error)
}

// FileInfo contains metadata about a file in an archive.
type FileInfo struct {
	Size  int64
	CRC32 uint32
}
