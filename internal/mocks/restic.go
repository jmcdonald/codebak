package mocks

import (
	"fmt"
	"time"

	"github.com/jmcdonald/codebak/internal/ports"
)

// MockResticClient implements ports.ResticClient for testing.
type MockResticClient struct {
	// InitializedRepos tracks which repos have been initialized
	InitializedRepos map[string]bool
	// Snapshots stores snapshots by repo path
	SnapshotsByRepo map[string][]ports.Snapshot
	// RestoredSnapshots tracks restore calls: repoPath -> snapshotID -> targetDir
	RestoredSnapshots map[string]map[string]string
	// ForgetCalls tracks forget calls: repoPath -> keepLast
	ForgetCalls map[string]int
	// NextSnapshotID is returned by the next Backup call
	NextSnapshotID string
	// Errors allows simulating errors for specific operations
	Errors struct {
		Init      error
		Backup    error
		Snapshots error
		Restore   error
		Forget    error
	}
}

// NewMockResticClient creates a new mock restic client.
func NewMockResticClient() *MockResticClient {
	return &MockResticClient{
		InitializedRepos:  make(map[string]bool),
		SnapshotsByRepo:   make(map[string][]ports.Snapshot),
		RestoredSnapshots: make(map[string]map[string]string),
		ForgetCalls:       make(map[string]int),
		NextSnapshotID:    "abc12345",
	}
}

// Init initializes a new restic repository at the given path.
func (m *MockResticClient) Init(repoPath, password string) error {
	if m.Errors.Init != nil {
		return m.Errors.Init
	}
	if m.InitializedRepos[repoPath] {
		return fmt.Errorf("repository already initialized at %s", repoPath)
	}
	m.InitializedRepos[repoPath] = true
	return nil
}

// Backup creates a new backup of the given paths to the repository.
func (m *MockResticClient) Backup(repoPath, password string, paths []string, tags []string) (string, error) {
	if m.Errors.Backup != nil {
		return "", m.Errors.Backup
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no paths specified for backup")
	}

	snapshotID := m.NextSnapshotID
	snapshot := ports.Snapshot{
		ID:       snapshotID,
		Time:     time.Now(),
		Hostname: "test-host",
		Paths:    paths,
		Tags:     tags,
	}

	m.SnapshotsByRepo[repoPath] = append(m.SnapshotsByRepo[repoPath], snapshot)
	return snapshotID, nil
}

// Snapshots returns all snapshots in the repository.
func (m *MockResticClient) Snapshots(repoPath, password string, tags []string) ([]ports.Snapshot, error) {
	if m.Errors.Snapshots != nil {
		return nil, m.Errors.Snapshots
	}

	snapshots := m.SnapshotsByRepo[repoPath]
	if snapshots == nil {
		return []ports.Snapshot{}, nil
	}

	// Filter by tags if specified
	if len(tags) == 0 {
		return snapshots, nil
	}

	var filtered []ports.Snapshot
	for _, snap := range snapshots {
		if matchesTags(snap.Tags, tags) {
			filtered = append(filtered, snap)
		}
	}
	return filtered, nil
}

// Restore restores a snapshot to the given target directory.
func (m *MockResticClient) Restore(repoPath, password, snapshotID, targetDir string) error {
	if m.Errors.Restore != nil {
		return m.Errors.Restore
	}

	if m.RestoredSnapshots[repoPath] == nil {
		m.RestoredSnapshots[repoPath] = make(map[string]string)
	}
	m.RestoredSnapshots[repoPath][snapshotID] = targetDir
	return nil
}

// Forget removes old snapshots according to the retention policy.
func (m *MockResticClient) Forget(repoPath, password string, keepLast int, prune bool) error {
	if m.Errors.Forget != nil {
		return m.Errors.Forget
	}

	m.ForgetCalls[repoPath] = keepLast

	// Simulate keeping only the last N snapshots
	snapshots := m.SnapshotsByRepo[repoPath]
	if len(snapshots) > keepLast {
		m.SnapshotsByRepo[repoPath] = snapshots[len(snapshots)-keepLast:]
	}
	return nil
}

// IsInitialized checks if a restic repository exists at the given path.
func (m *MockResticClient) IsInitialized(repoPath string) bool {
	return m.InitializedRepos[repoPath]
}

// matchesTags checks if any snapshot tag matches any filter tag.
func matchesTags(snapshotTags, filterTags []string) bool {
	for _, st := range snapshotTags {
		for _, ft := range filterTags {
			if st == ft {
				return true
			}
		}
	}
	return false
}

// Compile-time check that MockResticClient implements ports.ResticClient.
var _ ports.ResticClient = (*MockResticClient)(nil)
