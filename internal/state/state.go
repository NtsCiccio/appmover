// Package state persists small bits of AppMover's runtime memory across
// restarts (currently: which monitor each app was last moved to). Pure Go,
// no Win32 dependency, so it builds and tests on any platform.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// State is AppMover's persisted runtime memory.
type State struct {
	mu sync.Mutex

	// LastMonitorByProcess maps a lowercased executable name (e.g.
	// "chrome.exe") to the index into win32.EnumMonitors() it was last
	// moved to.
	LastMonitorByProcess map[string]int `json:"lastMonitorByProcess"`
}

// Path returns the state file location: <UserConfigDir>/AppMover/state.json.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "AppMover", "state.json"), nil
}

// Load reads the state file, returning an empty State if it doesn't exist
// yet or can't be parsed (a corrupt state file shouldn't stop AppMover from
// starting).
func Load() (*State, error) {
	s := &State{LastMonitorByProcess: map[string]int{}}

	path, err := Path()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return &State{LastMonitorByProcess: map[string]int{}}, nil
	}
	if s.LastMonitorByProcess == nil {
		s.LastMonitorByProcess = map[string]int{}
	}
	return s, nil
}

// Save writes the state to disk, creating its directory if needed.
func (s *State) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get returns the last-used monitor index for exeName, if known.
func (s *State) Get(exeName string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.LastMonitorByProcess[normalize(exeName)]
	return i, ok
}

// Set records monitorIndex as the last-used monitor for exeName.
func (s *State) Set(exeName string, monitorIndex int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastMonitorByProcess[normalize(exeName)] = monitorIndex
}

func normalize(exeName string) string {
	return strings.ToLower(strings.TrimSpace(exeName))
}
