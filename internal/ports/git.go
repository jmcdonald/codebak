package ports

// GitClient abstracts git operations for testability.
// Production code uses ExecGitClient adapter; tests use MockGitClient.
type GitClient interface {
	// GetHead returns the current HEAD commit hash for the repository.
	// Returns empty string if not a git repo or on error.
	GetHead(repoPath string) string

	// IsRepo checks if the given path is a git repository.
	IsRepo(path string) bool
}
