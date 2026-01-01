package tui

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mcdonaldj/codebak/internal/config"
	"github.com/sergi/go-diff/diffmatchpatch"
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
	defer func() { _ = r.Close() }()

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

// DiffLine represents a single line in the diff output
type DiffLine struct {
	LineNum1 int    // Line number in version 1 (0 if added)
	LineNum2 int    // Line number in version 2 (0 if deleted)
	Type     rune   // '+' added, '-' deleted, ' ' unchanged
	Content  string // Line content
}

// FileDiffResult contains the line-by-line diff of a single file
type FileDiffResult struct {
	Path     string
	Version1 string
	Version2 string
	Lines    []DiffLine
	IsBinary bool
	Error    string
}

// ReadZipFile extracts the contents of a file from a zip archive
func ReadZipFile(zipPath, filePath, projectName string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	// Look for the file with project prefix
	targetPath := projectName + "/" + filePath

	for _, f := range r.File {
		if f.Name == targetPath {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("file not found: %s", filePath)
}

// IsBinaryContent checks if content appears to be binary
func IsBinaryContent(content string) bool {
	if len(content) == 0 {
		return false
	}
	// Check first 8000 bytes for null bytes or invalid UTF-8
	checkLen := len(content)
	if checkLen > 8000 {
		checkLen = 8000
	}
	sample := content[:checkLen]

	// Check for null bytes (common in binary files)
	if strings.Contains(sample, "\x00") {
		return true
	}

	// Check if it's valid UTF-8
	if !utf8.ValidString(sample) {
		return true
	}

	return false
}

// ComputeFileDiff computes the line-by-line diff between two versions of a file
func ComputeFileDiff(cfg *config.Config, project, version1, version2, filePath string, status rune) (*FileDiffResult, error) {
	backupDir := config.ExpandPath(cfg.BackupDir)

	zip1Path := filepath.Join(backupDir, project, version1+".zip")
	zip2Path := filepath.Join(backupDir, project, version2+".zip")

	result := &FileDiffResult{
		Path:     filePath,
		Version1: version1,
		Version2: version2,
	}

	var content1, content2 string
	var err error

	// Read contents based on file status
	switch status {
	case 'A': // Added - only exists in v2
		content1 = ""
		content2, err = ReadZipFile(zip2Path, filePath, project)
		if err != nil {
			result.Error = fmt.Sprintf("Error reading file: %v", err)
			return result, nil
		}
	case 'D': // Deleted - only exists in v1
		content1, err = ReadZipFile(zip1Path, filePath, project)
		if err != nil {
			result.Error = fmt.Sprintf("Error reading file: %v", err)
			return result, nil
		}
		content2 = ""
	case 'M': // Modified - exists in both
		content1, err = ReadZipFile(zip1Path, filePath, project)
		if err != nil {
			result.Error = fmt.Sprintf("Error reading v1: %v", err)
			return result, nil
		}
		content2, err = ReadZipFile(zip2Path, filePath, project)
		if err != nil {
			result.Error = fmt.Sprintf("Error reading v2: %v", err)
			return result, nil
		}
	}

	// Check for binary content
	if IsBinaryContent(content1) || IsBinaryContent(content2) {
		result.IsBinary = true
		return result, nil
	}

	// Compute diff using go-diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(content1, content2, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Convert to line-based diff
	result.Lines = convertToLineDiff(content1, content2, diffs)

	return result, nil
}

// convertToLineDiff converts character-based diffs to line-based
func convertToLineDiff(content1, content2 string, diffs []diffmatchpatch.Diff) []DiffLine {
	var lines []DiffLine

	lines1 := strings.Split(content1, "\n")
	lines2 := strings.Split(content2, "\n")

	// Simple line-by-line comparison for cleaner output
	i, j := 0, 0
	for i < len(lines1) || j < len(lines2) {
		if i < len(lines1) && j < len(lines2) && lines1[i] == lines2[j] {
			// Lines match
			lines = append(lines, DiffLine{
				LineNum1: i + 1,
				LineNum2: j + 1,
				Type:     ' ',
				Content:  lines1[i],
			})
			i++
			j++
		} else if i < len(lines1) && (j >= len(lines2) || !containsLine(lines2[j:], lines1[i])) {
			// Line deleted from v1
			lines = append(lines, DiffLine{
				LineNum1: i + 1,
				LineNum2: 0,
				Type:     '-',
				Content:  lines1[i],
			})
			i++
		} else if j < len(lines2) {
			// Line added in v2
			lines = append(lines, DiffLine{
				LineNum1: 0,
				LineNum2: j + 1,
				Type:     '+',
				Content:  lines2[j],
			})
			j++
		}
	}

	return lines
}

func containsLine(lines []string, target string) bool {
	for _, line := range lines {
		if line == target {
			return true
		}
	}
	return false
}
