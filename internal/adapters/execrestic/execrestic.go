// Package execrestic provides a restic client adapter using exec.Command.
package execrestic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmcdonald/codebak/internal/ports"
)

// ExecResticClient implements ports.ResticClient using exec.Command.
type ExecResticClient struct {
	// resticPath is the path to the restic binary. Defaults to "restic".
	resticPath string
}

// Option is a functional option for configuring ExecResticClient.
type Option func(*ExecResticClient)

// WithResticPath sets a custom path to the restic binary.
func WithResticPath(path string) Option {
	return func(c *ExecResticClient) {
		c.resticPath = path
	}
}

// New creates a new ExecResticClient adapter.
func New(opts ...Option) *ExecResticClient {
	c := &ExecResticClient{
		resticPath: "restic",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Init initializes a new restic repository at the given path.
func (r *ExecResticClient) Init(repoPath, password string) error {
	cmd := r.command("init", "--repo", repoPath)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		// Check if repo already exists
		if strings.Contains(string(out), "already initialized") ||
			strings.Contains(string(out), "config file already exists") {
			return fmt.Errorf("repository already initialized at %s", repoPath)
		}
		return fmt.Errorf("restic init failed: %w: %s", err, string(out))
	}
	return nil
}

// Backup creates a new backup of the given paths to the repository.
func (r *ExecResticClient) Backup(repoPath, password string, paths []string, tags []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("no paths specified for backup")
	}

	args := []string{"backup", "--repo", repoPath, "--json"}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	args = append(args, paths...)

	cmd := r.command(args...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("restic backup failed: %w: %s", err, string(out))
	}

	// Parse JSON output to get snapshot ID
	// Restic outputs multiple JSON lines, we want the summary at the end
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var summary struct {
			MessageType string `json:"message_type"`
			SnapshotID  string `json:"snapshot_id"`
		}
		if err := json.Unmarshal([]byte(line), &summary); err != nil {
			continue
		}
		if summary.MessageType == "summary" && summary.SnapshotID != "" {
			return summary.SnapshotID, nil
		}
	}

	return "", fmt.Errorf("could not parse snapshot ID from restic output")
}

// Snapshots returns all snapshots in the repository.
func (r *ExecResticClient) Snapshots(repoPath, password string, tags []string) ([]ports.Snapshot, error) {
	args := []string{"snapshots", "--repo", repoPath, "--json"}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	cmd := r.command(args...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("restic snapshots failed: %w: %s", err, string(out))
	}

	// Parse JSON output
	var snapshots []ports.Snapshot
	if err := json.Unmarshal(out, &snapshots); err != nil {
		// Empty repository returns "null"
		if strings.TrimSpace(string(out)) == "null" {
			return []ports.Snapshot{}, nil
		}
		return nil, fmt.Errorf("failed to parse snapshots: %w", err)
	}

	return snapshots, nil
}

// Restore restores a snapshot to the given target directory.
func (r *ExecResticClient) Restore(repoPath, password, snapshotID, targetDir string) error {
	if snapshotID == "" {
		snapshotID = "latest"
	}

	cmd := r.command("restore", "--repo", repoPath, "--target", targetDir, snapshotID)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic restore failed: %w: %s", err, string(out))
	}
	return nil
}

// Forget removes old snapshots according to the retention policy.
func (r *ExecResticClient) Forget(repoPath, password string, keepLast int, prune bool) error {
	args := []string{"forget", "--repo", repoPath, fmt.Sprintf("--keep-last=%d", keepLast)}
	if prune {
		args = append(args, "--prune")
	}

	cmd := r.command(args...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restic forget failed: %w: %s", err, string(out))
	}
	return nil
}

// IsInitialized checks if a restic repository exists at the given path.
func (r *ExecResticClient) IsInitialized(repoPath string) bool {
	// Check for restic config file
	configPath := filepath.Join(repoPath, "config")
	info, err := os.Stat(configPath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// command creates an exec.Cmd for the restic binary.
func (r *ExecResticClient) command(args ...string) *exec.Cmd {
	return exec.Command(r.resticPath, args...)
}

// Compile-time check that ExecResticClient implements ports.ResticClient.
var _ ports.ResticClient = (*ExecResticClient)(nil)
