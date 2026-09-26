package logrotate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRotateCapsLogAtTenMB confirms the log-rotation invariant: a log file
// that has grown past DefaultMaxBytes must be rotated, leaving a live file
// under the cap and a .1 generation carrying the prior content.
func TestRotateCapsLogAtTenMB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "codebak.log")
	big := strings.Repeat("x", int(DefaultMaxBytes)+1024)
	if err := os.WriteFile(logPath, []byte(big), 0644); err != nil {
		t.Fatalf("Failed to write oversized log: %v", err)
	}

	result, err := Rotate(logPath, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if !result.Rotated {
		t.Fatal("expected Rotated=true for an oversized log")
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("live log should still exist: %v", err)
	}
	if info.Size() >= DefaultMaxBytes {
		t.Errorf("live log size = %d, expected < %d after rotation", info.Size(), DefaultMaxBytes)
	}

	gen1 := logPath + ".1"
	gen1Data, err := os.ReadFile(gen1)
	if err != nil {
		t.Fatalf("codebak.log.1 should exist after rotation: %v", err)
	}
	if string(gen1Data) != big {
		t.Error("codebak.log.1 should carry the pre-rotation content")
	}
}

func TestRotateSmallFileIsNoOp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "codebak.log")
	if err := os.WriteFile(logPath, []byte("small log line\n"), 0644); err != nil {
		t.Fatalf("Failed to write log: %v", err)
	}

	result, err := Rotate(logPath, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if result.Rotated {
		t.Error("a log under the cap must not be rotated")
	}

	if _, err := os.Stat(logPath + ".1"); !os.IsNotExist(err) {
		t.Error("no .1 generation should be created for a log under the cap")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("live log should still exist: %v", err)
	}
	if string(data) != "small log line\n" {
		t.Error("live log content must be untouched when no rotation occurs")
	}
}

func TestRotateMissingFileIsNoOp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "does-not-exist.log")

	result, err := Rotate(logPath, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("Rotate on a missing file should not error: %v", err)
	}
	if result.Rotated {
		t.Error("a missing file cannot be rotated")
	}
}

// TestRotateShiftsGenerationsAndDropsOldest proves generation shifting:
// .1 -> .2 -> .3, and an existing .3 is dropped to make room, never
// silently accumulating beyond MaxGenerations.
func TestRotateShiftsGenerationsAndDropsOldest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "codebak-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "codebak.log")
	big := strings.Repeat("y", int(DefaultMaxBytes)+1024)
	if err := os.WriteFile(logPath, []byte(big), 0644); err != nil {
		t.Fatalf("Failed to write oversized log: %v", err)
	}

	// Pre-seed three generations so this rotation must shift all of them
	// and drop the oldest (.3) to make room.
	if err := os.WriteFile(logPath+".1", []byte("generation one"), 0644); err != nil {
		t.Fatalf("Failed to seed .1: %v", err)
	}
	if err := os.WriteFile(logPath+".2", []byte("generation two"), 0644); err != nil {
		t.Fatalf("Failed to seed .2: %v", err)
	}
	if err := os.WriteFile(logPath+".3", []byte("generation three, oldest"), 0644); err != nil {
		t.Fatalf("Failed to seed .3: %v", err)
	}

	result, err := Rotate(logPath, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}
	if !result.Rotated {
		t.Fatal("expected Rotated=true")
	}
	if result.Deleted != logPath+".3" {
		t.Errorf("Deleted = %q, expected the oldest generation %q to be dropped", result.Deleted, logPath+".3")
	}

	gen1, err := os.ReadFile(logPath + ".1")
	if err != nil {
		t.Fatalf(".1 should exist: %v", err)
	}
	if string(gen1) != big {
		t.Error(".1 should now hold the just-rotated (former live) content")
	}

	gen2, err := os.ReadFile(logPath + ".2")
	if err != nil {
		t.Fatalf(".2 should exist: %v", err)
	}
	if string(gen2) != "generation one" {
		t.Error(".2 should hold what used to be .1")
	}

	gen3, err := os.ReadFile(logPath + ".3")
	if err != nil {
		t.Fatalf(".3 should exist: %v", err)
	}
	if string(gen3) != "generation two" {
		t.Error(".3 should hold what used to be .2, not the original .3 (which was dropped)")
	}

	// The original "generation three, oldest" content must be gone: no
	// generation 4 exists to hold it, and MaxGenerations caps at 3.
	if _, err := os.Stat(logPath + ".4"); !os.IsNotExist(err) {
		t.Error("no .4 generation should ever be created (MaxGenerations = 3)")
	}
}
