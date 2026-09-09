package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

// isolateConfigDir points os.UserConfigDir() at a fresh temp directory,
// regardless of platform (Linux uses XDG_CONFIG_HOME, Windows uses AppData,
// Darwin uses HOME).
func isolateConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	isolateConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("Load() on fresh dir = %+v, want defaults %+v", cfg, Default())
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if _, err := filepath.Abs(path); err != nil {
		t.Fatalf("Path() returned invalid path %q: %v", path, err)
	}

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg2, cfg) {
		t.Errorf("second Load() = %+v, want %+v (file should now exist and round-trip)", cfg2, cfg)
	}
}

func TestLoadFillsMissingFieldsWithDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	partial := Config{RefreshIntervalMS: 5000}
	if err := Save(partial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RefreshIntervalMS != 5000 {
		t.Errorf("RefreshIntervalMS = %d, want 5000 (explicit value should survive)", cfg.RefreshIntervalMS)
	}
	if cfg.MaxWindowSlots != Default().MaxWindowSlots {
		t.Errorf("MaxWindowSlots = %d, want default %d (missing field should be filled in)", cfg.MaxWindowSlots, Default().MaxWindowSlots)
	}
	if cfg.MoveNextMonitorHotkey != Default().MoveNextMonitorHotkey {
		t.Errorf("MoveNextMonitorHotkey = %q, want default %q", cfg.MoveNextMonitorHotkey, Default().MoveNextMonitorHotkey)
	}
}

func TestParseHotkey(t *testing.T) {
	tests := []struct {
		in        string
		wantMods  uint32
		wantVK    uint32
		wantError bool
	}{
		{in: "win+shift+right", wantMods: ModWin | ModShift, wantVK: vkRight},
		{in: "win+shift+left", wantMods: ModWin | ModShift, wantVK: vkLeft},
		{in: "ctrl+alt+m", wantMods: ModControl | ModAlt, wantVK: 'M'},
		{in: "ctrl+alt+shift+9", wantMods: ModControl | ModAlt | ModShift, wantVK: '9'},
		{in: "Win+Shift+Up", wantMods: ModWin | ModShift, wantVK: vkUp},
		{in: "right", wantError: true},      // no modifier
		{in: "win+", wantError: true},       // empty key
		{in: "win+banana", wantError: true}, // unsupported key
		{in: "cmd+shift+down", wantMods: ModWin | ModShift, wantVK: vkDown},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			mods, vk, err := ParseHotkey(tt.in)
			if tt.wantError {
				if err == nil {
					t.Fatalf("ParseHotkey(%q) = (%v, %v, nil), want error", tt.in, mods, vk)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHotkey(%q) unexpected error: %v", tt.in, err)
			}
			if mods != tt.wantMods {
				t.Errorf("ParseHotkey(%q) mods = %#x, want %#x", tt.in, mods, tt.wantMods)
			}
			if vk != tt.wantVK {
				t.Errorf("ParseHotkey(%q) vk = %#x, want %#x", tt.in, vk, tt.wantVK)
			}
		})
	}
}
