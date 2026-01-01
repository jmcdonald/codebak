// Package execgit provides a git client adapter using exec.Command.
package execgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mcdonaldj/codebak/internal/ports"
)

// ExecGitClient implements ports.GitClient using exec.Command.
type ExecGitClient struct{}

// New creates a new ExecGitClient adapter.
func New() *ExecGitClient {
	return &ExecGitClient{}
}

// GetHead returns the current HEAD commit hash for the repository.
// Returns empty string if not a git repo or on error.
func (g *ExecGitClient) GetHead(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsRepo checks if the given path is a git repository.
func (g *ExecGitClient) IsRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Compile-time check that ExecGitClient implements ports.GitClient.
var _ ports.GitClient = (*ExecGitClient)(nil)
