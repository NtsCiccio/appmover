// Package config loads and saves AppMover's user-editable settings. It has
// no Win32 dependency so it builds and tests on any platform; the numeric
// hotkey modifier/virtual-key constants below match the real Win32
// MOD_*/VK_* values so callers on Windows can pass them straight to
// RegisterHotKey without translation.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Win32 MOD_* values (winuser.h), duplicated here so this package stays
// platform-independent while still producing values RegisterHotKey expects.
const (
	ModAlt     = 0x0001
	ModControl = 0x0002
	ModShift   = 0x0004
	ModWin     = 0x0008
)

// Win32 VK_* values needed for the keys we support.
const (
	vkLeft  = 0x25
	vkUp    = 0x26
	vkRight = 0x27
	vkDown  = 0x28
)

// Config holds AppMover's user-editable settings.
type Config struct {
	RefreshIntervalMS     int      `json:"refreshIntervalMs"`
	MoveNextMonitorHotkey string   `json:"moveNextMonitorHotkey"`
	MovePrevMonitorHotkey string   `json:"movePrevMonitorHotkey"`
	MaxWindowSlots        int      `json:"maxWindowSlots"`
	ExcludedProcessNames  []string `json:"excludedProcessNames"`
	ExcludedTitles        []string `json:"excludedTitles"`
}

// Default returns the settings AppMover ships with.
func Default() Config {
	return Config{
		RefreshIntervalMS:     10000,
		MoveNextMonitorHotkey: "win+shift+right",
		MovePrevMonitorHotkey: "win+shift+left",
		MaxWindowSlots:        20,
		ExcludedProcessNames:  []string{},
		ExcludedTitles:        []string{},
	}
}

// Path returns the config file location: <UserConfigDir>/AppMover/config.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "AppMover", "config.json"), nil
}

// Load reads the config file, creating it with default values if it
// doesn't exist yet. Any field left at its zero value in an existing file
// falls back to the default (so a hand-edited partial config still works).
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Default(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		return cfg, Save(cfg)
	}
	if err != nil {
		return Default(), err
	}

	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	def := Default()
	if cfg.RefreshIntervalMS <= 0 {
		cfg.RefreshIntervalMS = def.RefreshIntervalMS
	}
	if cfg.MoveNextMonitorHotkey == "" {
		cfg.MoveNextMonitorHotkey = def.MoveNextMonitorHotkey
	}
	if cfg.MovePrevMonitorHotkey == "" {
		cfg.MovePrevMonitorHotkey = def.MovePrevMonitorHotkey
	}
	if cfg.MaxWindowSlots <= 0 {
		cfg.MaxWindowSlots = def.MaxWindowSlots
	}
	if cfg.ExcludedProcessNames == nil {
		cfg.ExcludedProcessNames = def.ExcludedProcessNames
	}
	if cfg.ExcludedTitles == nil {
		cfg.ExcludedTitles = def.ExcludedTitles
	}
}

// Save writes cfg to the config file, creating its directory if needed.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ParseHotkey parses a hotkey string like "win+shift+right" or "ctrl+alt+m"
// into Win32 MOD_* modifier flags and a VK_* virtual-key code.
func ParseHotkey(s string) (modifiers uint32, vk uint32, err error) {
	parts := strings.Split(s, "+")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("hotkey %q needs at least one modifier and one key", s)
	}

	keyParts, modParts := parts[len(parts)-1], parts[:len(parts)-1]

	for _, m := range modParts {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "win", "super", "cmd":
			modifiers |= ModWin
		case "ctrl", "control":
			modifiers |= ModControl
		case "shift":
			modifiers |= ModShift
		case "alt":
			modifiers |= ModAlt
		default:
			return 0, 0, fmt.Errorf("hotkey %q: unknown modifier %q", s, m)
		}
	}

	switch strings.ToLower(strings.TrimSpace(keyParts)) {
	case "left":
		vk = vkLeft
	case "right":
		vk = vkRight
	case "up":
		vk = vkUp
	case "down":
		vk = vkDown
	default:
		key := strings.ToUpper(strings.TrimSpace(keyParts))
		if len(key) == 1 && ((key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')) {
			vk = uint32(key[0]) // VK codes for 'A'-'Z'/'0'-'9' match their ASCII value
		} else {
			return 0, 0, fmt.Errorf("hotkey %q: unsupported key %q", s, keyParts)
		}
	}

	if modifiers == 0 {
		return 0, 0, fmt.Errorf("hotkey %q needs at least one modifier", s)
	}

	return modifiers, vk, nil
}
