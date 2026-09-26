package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

type BackupEntry struct {
	File      string    `json:"file"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
	GitHead   string    `json:"git_head,omitempty"`
	FileCount int       `json:"file_count"`
	Excluded  []string  `json:"excluded"`
}

type Manifest struct {
	Project string        `json:"project"`
	Source  string        `json:"source"`
	Backups []BackupEntry `json:"backups"`
}

func ManifestPath(backupDir, project string) string {
	return filepath.Join(backupDir, project, "manifest.json")
}

func Load(backupDir, project string) (*Manifest, error) {
	path := ManifestPath(backupDir, project)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Project: project,
				Backups: []BackupEntry{},
			}, nil
		}
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *Manifest) Save(backupDir string) error {
	path := ManifestPath(backupDir, m.Project)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *Manifest) AddBackup(entry BackupEntry) {
	m.Backups = append(m.Backups, entry)
}

func (m *Manifest) LatestBackup() *BackupEntry {
	if len(m.Backups) == 0 {
		return nil
	}
	return &m.Backups[len(m.Backups)-1]
}

// Prune removes old backups exceeding keepLast limit.
// Returns list of deleted files and any error.
func (m *Manifest) Prune(backupDir string, keepLast int) ([]string, error) {
	return m.PruneWithOptions(backupDir, keepLast, false)
}

// PruneWithOptions removes old backups exceeding keepLast limit. When
// dryRun is true, nothing is deleted and the manifest is not modified: the
// returned list instead names the files that WOULD be deleted, so callers
// can log exactly what a real run would remove.
//
// keepLast counts only backups whose zip file is actually present on disk.
// A manifest can carry "phantom" entries whose file was already removed by
// an earlier run or by hand; those entries never occupy a keep slot and are
// never picked as a candidate for removal, so they can never push a real,
// present backup out and get it deleted. This does not reconcile or
// rewrite phantom entries out of the manifest; that reconciliation is a
// separate, not-yet-built feature.
func (m *Manifest) PruneWithOptions(backupDir string, keepLast int, dryRun bool) ([]string, error) {
	if keepLast <= 0 || len(m.Backups) == 0 {
		return nil, nil
	}

	// Backups are ordered oldest to newest. Walk once, statting each file,
	// to find which entries are actually present on disk.
	var present []int // indices into m.Backups, oldest to newest present
	for i, entry := range m.Backups {
		zipPath := filepath.Join(backupDir, m.Project, entry.File)
		if _, err := os.Stat(zipPath); err != nil {
			continue
		}
		present = append(present, i)
	}

	toRemove := len(present) - keepLast
	if toRemove <= 0 {
		return nil, nil
	}

	var deleted []string
	removedIndex := make(map[int]bool)

	for pi := 0; pi < toRemove; pi++ {
		index := present[pi]
		file := m.Backups[index].File
		zipPath := filepath.Join(backupDir, m.Project, file)

		if dryRun {
			deleted = append(deleted, file)
			removedIndex[index] = true
			continue
		}

		if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
			// Real error removing the file: leave the manifest entry in
			// place so the file is never orphaned.
			continue
		}
		deleted = append(deleted, file)
		removedIndex[index] = true
	}

	if !dryRun && len(removedIndex) > 0 {
		survivors := m.Backups[:0:0]
		for i, entry := range m.Backups {
			if removedIndex[i] {
				continue
			}
			survivors = append(survivors, entry)
		}
		m.Backups = survivors
	}

	return deleted, nil
}

// PruneByTotalSize removes the oldest backups until the project's total
// on-disk size is at or under maxTotalBytes, in addition to (whichever is
// stricter than) count-based Prune. The newest backup is never removed,
// even if it alone exceeds the cap. When dryRun is true, nothing is
// deleted and the manifest is not modified: the returned list names the
// files that WOULD be deleted.
//
// Size comes from a stat of the on-disk file, never from the manifest's
// recorded size_bytes. A manifest can carry "phantom" entries whose zip
// was already deleted by an earlier run or by hand; those entries are
// ignored for both the running total and the keep-at-least-one count, so
// a phantom entry's stale size can never make a real, present file look
// deletable. This does not reconcile or rewrite phantom entries out of
// the manifest; that reconciliation is a separate, not-yet-built feature.
func (m *Manifest) PruneByTotalSize(backupDir string, maxTotalBytes int64, dryRun bool) ([]string, error) {
	if maxTotalBytes <= 0 || len(m.Backups) == 0 {
		return nil, nil
	}

	type presentEntry struct {
		index int   // position in m.Backups
		size  int64 // on-disk size
	}

	// Backups are ordered oldest to newest. Walk them once, statting each
	// file. Entries whose file is missing are skipped entirely: they add
	// nothing to total and never occupy a "keep" slot.
	var present []presentEntry
	var total int64
	for i, entry := range m.Backups {
		zipPath := filepath.Join(backupDir, m.Project, entry.File)
		info, err := os.Stat(zipPath)
		if err != nil {
			continue
		}
		present = append(present, presentEntry{index: i, size: info.Size()})
		total += info.Size()
	}

	if len(present) <= 1 {
		return nil, nil
	}

	var deleted []string
	removedIndex := make(map[int]bool)

	// Walk the present (on-disk) entries oldest first. len(present) minus
	// however many are already marked removed is how many present backups
	// remain; stop once that reaches one, so the newest present backup is
	// never removed. An entry whose real deletion fails for a reason other
	// than not-exist stays in place (not marked removed) rather than being
	// silently dropped from the manifest, so the loop simply moves on to
	// the next present entry.
	for pi := 0; pi < len(present) && total > maxTotalBytes && len(present)-len(removedIndex) > 1; pi++ {
		entry := present[pi]
		file := m.Backups[entry.index].File
		zipPath := filepath.Join(backupDir, m.Project, file)

		if dryRun {
			deleted = append(deleted, file)
			total -= entry.size
			removedIndex[entry.index] = true
			continue
		}

		if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
			// Real error removing the file (e.g. permission denied): leave
			// the manifest entry in place so the file is never orphaned.
			continue
		}

		deleted = append(deleted, file)
		total -= entry.size
		removedIndex[entry.index] = true
	}

	if !dryRun && len(removedIndex) > 0 {
		survivors := m.Backups[:0:0]
		for i, entry := range m.Backups {
			if removedIndex[i] {
				continue
			}
			survivors = append(survivors, entry)
		}
		m.Backups = survivors
	}

	return deleted, nil
}

// ComputeSHA256 calculates SHA256 hash of a file
func ComputeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
