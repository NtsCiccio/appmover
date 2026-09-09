package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolateCacheDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("LocalAppData", dir)
	t.Setenv("HOME", dir)
}

func TestOpenCreatesLogFile(t *testing.T) {
	isolateCacheDir(t)

	logger, err := Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	logger.Println("hello")

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Errorf("log file content = %q, want it to contain %q", data, "hello")
	}
}

func TestRotateIfNeededRotatesOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appmover.log")

	if err := os.WriteFile(path, make([]byte, MaxSizeBytes+1), 0644); err != nil {
		t.Fatalf("seeding oversized file: %v", err)
	}

	if err := rotateIfNeeded(path); err != nil {
		t.Fatalf("rotateIfNeeded() error = %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("original file should have been renamed away, stat err = %v", err)
	}
	if _, err := os.Stat(path + ".old"); err != nil {
		t.Errorf("expected a .old backup, stat err = %v", err)
	}
}

func TestRotateIfNeededLeavesSmallFileAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appmover.log")

	if err := os.WriteFile(path, []byte("small"), 0644); err != nil {
		t.Fatalf("seeding file: %v", err)
	}

	if err := rotateIfNeeded(path); err != nil {
		t.Fatalf("rotateIfNeeded() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != "small" {
		t.Errorf("file content = %q, want untouched %q", data, "small")
	}
	if _, err := os.Stat(path + ".old"); !os.IsNotExist(err) {
		t.Errorf("no .old backup should have been created, stat err = %v", err)
	}
}

func TestRotateIfNeededMissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.log")

	if err := rotateIfNeeded(path); err != nil {
		t.Errorf("rotateIfNeeded() on a missing file error = %v, want nil", err)
	}
}

func TestOpenTwiceAppends(t *testing.T) {
	isolateCacheDir(t)

	l1, err := Open()
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	l1.Println("first")

	l2, err := Open()
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	l2.Println("second")

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("log file content = %q, want both entries present (append, not truncate)", data)
	}
}
