//go:build windows

// Package winlist produces the list of "real" application windows
// currently open, applying the filters above the raw primitives exposed by
// internal/win32.
package winlist

import (
	"os"
	"strings"

	"appmover/internal/vdesktop"
	"appmover/internal/win32"

	"github.com/gonutz/w32/v2"
)

// Window represents an application window visible to the user.
type Window struct {
	Handle  w32.HWND
	Title   string
	PID     uint32
	ExeName string // e.g. "chrome.exe"; empty if it couldn't be determined
}

// Options controls what Open considers a "real" window.
type Options struct {
	// ExcludedProcessNames, case-insensitive, e.g. "chrome.exe".
	ExcludedProcessNames []string
	// ExcludedTitles: windows whose title contains any of these
	// (case-insensitive) substrings are skipped.
	ExcludedTitles []string
	// VDesktop, if non-nil, is used to skip windows that aren't on the
	// currently active virtual desktop. A nil VDesktop (or a query
	// failure) simply doesn't filter by virtual desktop.
	VDesktop *vdesktop.Manager
}

// Open returns the list of open application windows, excluding system
// windows, tool windows, titleless windows, AppMover itself, cloaked
// (DWM-hidden) windows, windows on other virtual desktops, and anything
// matched by opts' exclusion lists.
func Open(opts Options) []Window {
	shellWindow := win32.ShellWindow()
	currentPID := uint32(os.Getpid())

	var windows []Window

	win32.EnumWindows(func(hwnd w32.HWND) bool {
		if hwnd == shellWindow {
			return true // keep enumerating, skip this one
		}
		if !w32.IsWindowVisible(hwnd) {
			return true
		}
		if win32.IsToolWindow(hwnd) {
			return true
		}
		if win32.IsCloaked(hwnd) {
			return true
		}

		pid := win32.WindowProcessID(hwnd)
		if pid == currentPID {
			return true // don't list AppMover itself
		}

		title := win32.WindowText(hwnd)
		if title == "" {
			return true
		}
		if matchesAny(title, opts.ExcludedTitles) {
			return true
		}

		exeName, _ := win32.ProcessExeName(pid)
		if matchesAny(exeName, opts.ExcludedProcessNames) {
			return true
		}

		if opts.VDesktop != nil {
			onCurrent, err := opts.VDesktop.IsOnCurrentDesktop(uintptr(hwnd))
			if err == nil && !onCurrent {
				return true
			}
		}

		windows = append(windows, Window{Handle: hwnd, Title: title, PID: pid, ExeName: exeName})
		return true
	})

	return windows
}

func matchesAny(s string, substrings []string) bool {
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	for _, sub := range substrings {
		if sub == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
