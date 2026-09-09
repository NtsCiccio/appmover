package state

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateStateDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
}

func TestLoadEmptyWhenMissing(t *testing.T) {
	isolateStateDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(s.LastMonitorByProcess) != 0 {
		t.Errorf("fresh Load() = %+v, want empty map", s.LastMonitorByProcess)
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	isolateStateDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s.Set("Chrome.exe", 1)
	s.Set("slack.exe", 0)

	if got, ok := s.Get("chrome.exe"); !ok || got != 1 {
		t.Errorf("Get(chrome.exe) = (%d, %v), want (1, true) — lookup should be case-insensitive", got, ok)
	}
	if _, ok := s.Get("notepad.exe"); ok {
		t.Errorf("Get(notepad.exe) found a value, want not-found for a never-set key")
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	s2, err := Load()
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if got, ok := s2.Get("chrome.exe"); !ok || got != 1 {
		t.Errorf("after reload, Get(chrome.exe) = (%d, %v), want (1, true)", got, ok)
	}
	if got, ok := s2.Get("slack.exe"); !ok || got != 0 {
		t.Errorf("after reload, Get(slack.exe) = (%d, %v), want (0, true)", got, ok)
	}
}

func TestLoadRecoversFromCorruptFile(t *testing.T) {
	isolateStateDir(t)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() on corrupt file returned error = %v, want nil (should recover)", err)
	}
	if len(s.LastMonitorByProcess) != 0 {
		t.Errorf("Load() on corrupt file = %+v, want empty map", s.LastMonitorByProcess)
	}
}
