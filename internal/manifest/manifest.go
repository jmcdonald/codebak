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

// Prune removes old backups exceeding keepLast limit
// Returns list of deleted files and any error
func (m *Manifest) Prune(backupDir string, keepLast int) ([]string, error) {
	if keepLast <= 0 || len(m.Backups) <= keepLast {
		return nil, nil
	}

	// Backups are ordered oldest to newest
	toRemove := len(m.Backups) - keepLast
	var deleted []string

	for i := 0; i < toRemove; i++ {
		entry := m.Backups[i]
		zipPath := filepath.Join(backupDir, m.Project, entry.File)

		if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
			// Continue on error but track it
			continue
		}
		deleted = append(deleted, entry.File)
	}

	// Update manifest to keep only recent backups
	m.Backups = m.Backups[toRemove:]

	return deleted, nil
}

// ComputeSHA256 calculates SHA256 hash of a file
func ComputeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
