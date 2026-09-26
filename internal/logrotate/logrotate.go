// Package logrotate caps codebak's launchd-redirected log file so it does
// not grow without bound.
package logrotate

import (
	"fmt"
	"os"
)

const (
	// DefaultMaxBytes is the size at which the log is rotated.
	DefaultMaxBytes int64 = 10 * 1024 * 1024 // 10 MB

	// MaxGenerations is how many rotated copies (path.1 .. path.N) are kept.
	MaxGenerations = 3
)

// Result describes what Rotate did.
type Result struct {
	Rotated bool   // true if rotation happened
	Deleted string // path of the oldest generation removed to make room, if any
}

// Rotate checks the size of the file at path. If it is at or over maxBytes,
// it shifts existing generations (path.1 -> path.2 -> path.3, dropping any
// path.3 that already existed), copies the current content into path.1,
// then truncates path in place.
//
// Truncating in place (rather than renaming path away) matters: codebak's
// launchd job redirects its own stdout/stderr to path for the lifetime of
// the process (StandardOutPath/StandardErrorPath), so the running process
// already holds that path open. Renaming path out from under it would
// leave the running process still writing into the renamed, once-rotated
// file, while nothing new appears at path until the next launchd
// invocation opens it fresh. Truncating the same inode instead means any
// further writes in the current process land in a now-empty file at the
// expected path.
func Rotate(path string, maxBytes int64) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, nil
		}
		return Result{}, err
	}

	if info.Size() < maxBytes {
		return Result{}, nil
	}

	result := Result{}

	// Drop the oldest generation to make room, if it exists.
	oldest := fmt.Sprintf("%s.%d", path, MaxGenerations)
	if _, statErr := os.Stat(oldest); statErr == nil {
		if err := os.Remove(oldest); err != nil {
			return result, fmt.Errorf("removing oldest log generation %s: %w", oldest, err)
		}
		result.Deleted = oldest
	}

	// Shift the remaining generations up by one, oldest first.
	for gen := MaxGenerations - 1; gen >= 1; gen-- {
		src := fmt.Sprintf("%s.%d", path, gen)
		dst := fmt.Sprintf("%s.%d", path, gen+1)
		if _, statErr := os.Stat(src); statErr == nil {
			if err := os.Rename(src, dst); err != nil {
				return result, fmt.Errorf("rotating %s to %s: %w", src, dst, err)
			}
		}
	}

	// Copy the live log's content into generation 1, then truncate the
	// live file in place so any open file descriptor on it keeps writing
	// to the same path/inode.
	data, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("reading %s before rotation: %w", path, err)
	}

	gen1 := path + ".1"
	if err := os.WriteFile(gen1, data, 0644); err != nil {
		return result, fmt.Errorf("writing %s: %w", gen1, err)
	}

	if err := os.Truncate(path, 0); err != nil {
		return result, fmt.Errorf("truncating %s: %w", path, err)
	}

	result.Rotated = true
	return result, nil
}
