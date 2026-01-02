package mocks

import (
	"github.com/jmcdonald/codebak/internal/ports"
)

// MockGitClient implements ports.GitClient for testing.
type MockGitClient struct {
	// Heads maps repository paths to HEAD commit hashes
	Heads map[string]string
	// Repos maps paths to whether they are git repos
	Repos map[string]bool
}

// NewMockGitClient creates a new mock git client.
func NewMockGitClient() *MockGitClient {
	return &MockGitClient{
		Heads: make(map[string]string),
		Repos: make(map[string]bool),
	}
}

// GetHead returns the current HEAD commit hash for the repository.
// Returns empty string if not a git repo or on error.
func (m *MockGitClient) GetHead(repoPath string) string {
	if head, ok := m.Heads[repoPath]; ok {
		return head
	}
	return ""
}

// IsRepo checks if the given path is a git repository.
func (m *MockGitClient) IsRepo(path string) bool {
	if isRepo, ok := m.Repos[path]; ok {
		return isRepo
	}
	return false
}

// Compile-time check that MockGitClient implements ports.GitClient.
var _ ports.GitClient = (*MockGitClient)(nil)
