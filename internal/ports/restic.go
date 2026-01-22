package ports

import "time"

// Snapshot represents a restic backup snapshot.
type Snapshot struct {
	ID       string    `json:"id"`        // Short snapshot ID
	Time     time.Time `json:"time"`      // Snapshot creation time
	Hostname string    `json:"hostname"`  // Machine hostname
	Paths    []string  `json:"paths"`     // Backed up paths
	Tags     []string  `json:"tags"`      // Snapshot tags
}

// ResticClient abstracts restic operations for testability.
// Production code uses ExecResticClient adapter; tests use MockResticClient.
type ResticClient interface {
	// Init initializes a new restic repository at the given path.
	// Returns an error if the repository already exists or cannot be created.
	Init(repoPath, password string) error

	// Backup creates a new backup of the given paths to the repository.
	// Tags can be used to identify/filter snapshots later.
	// Returns the snapshot ID on success.
	Backup(repoPath, password string, paths []string, tags []string) (string, error)

	// Snapshots returns all snapshots in the repository.
	// Can optionally filter by tags.
	Snapshots(repoPath, password string, tags []string) ([]Snapshot, error)

	// Restore restores a snapshot to the given target directory.
	// Use "latest" as snapshotID to restore the most recent snapshot.
	Restore(repoPath, password, snapshotID, targetDir string) error

	// Forget removes old snapshots according to the retention policy.
	// keepLast specifies how many recent snapshots to keep.
	// If prune is true, also removes unreferenced data from the repository.
	Forget(repoPath, password string, keepLast int, prune bool) error

	// IsInitialized checks if a restic repository exists at the given path.
	IsInitialized(repoPath string) bool
}
