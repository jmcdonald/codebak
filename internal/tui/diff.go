package tui

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mcdonaldj/codebak/internal/config"
)

// FileChange represents a change between two versions
type FileChange struct {
	Path   string
	Status rune // 'M' modified, 'A' added, 'D' deleted
	Size1  int64
	Size2  int64
}

// DiffResult contains the comparison between two backup versions
type DiffResult struct {
	Version1 string
	Version2 string
	Changes  []FileChange
	Added    int
	Modified int
	Deleted  int
}

// ComputeDiff compares two backup versions and returns the differences
func ComputeDiff(cfg *config.Config, project, version1, version2 string) (*DiffResult, error) {
	backupDir := config.ExpandPath(cfg.BackupDir)

	zip1Path := filepath.Join(backupDir, project, version1)
	zip2Path := filepath.Join(backupDir, project, version2)

	files1, err := listZipFiles(zip1Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", version1, err)
	}

	files2, err := listZipFiles(zip2Path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", version2, err)
	}

	result := &DiffResult{
		Version1: strings.TrimSuffix(version1, ".zip"),
		Version2: strings.TrimSuffix(version2, ".zip"),
	}

	// Find all unique paths
	allPaths := make(map[string]bool)
	for path := range files1 {
		allPaths[path] = true
	}
	for path := range files2 {
		allPaths[path] = true
	}

	// Compare files
	for path := range allPaths {
		info1, in1 := files1[path]
		info2, in2 := files2[path]

		var change FileChange
		change.Path = path

		switch {
		case in1 && !in2:
			// File was deleted (exists in v1, not in v2)
			change.Status = 'D'
			change.Size1 = info1.size
			result.Deleted++
		case !in1 && in2:
			// File was added (not in v1, exists in v2)
			change.Status = 'A'
			change.Size2 = info2.size
			result.Added++
		case info1.crc32 != info2.crc32 || info1.size != info2.size:
			// File was modified
			change.Status = 'M'
			change.Size1 = info1.size
			change.Size2 = info2.size
			result.Modified++
		default:
			// Unchanged, skip
			continue
		}

		result.Changes = append(result.Changes, change)
	}

	// Sort changes: D, M, A then by path
	sort.Slice(result.Changes, func(i, j int) bool {
		if result.Changes[i].Status != result.Changes[j].Status {
			order := map[rune]int{'M': 0, 'A': 1, 'D': 2}
			return order[result.Changes[i].Status] < order[result.Changes[j].Status]
		}
		return result.Changes[i].Path < result.Changes[j].Path
	})

	return result, nil
}

type fileInfo struct {
	size  int64
	crc32 uint32
}

func listZipFiles(zipPath string) (map[string]fileInfo, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	files := make(map[string]fileInfo)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Strip project prefix from path (first component)
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		path := parts[1]
		files[path] = fileInfo{
			size:  int64(f.UncompressedSize64),
			crc32: f.CRC32,
		}
	}

	return files, nil
}
