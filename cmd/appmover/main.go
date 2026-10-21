package main

import (
	"os"

	"appmover/internal/tray"
	"appmover/internal/win32"

	"fyne.io/systray"
	w32 "github.com/gonutz/w32/v2"
)

func main() {
	// Must happen before any window (including the systray's own) is
	// created, otherwise Windows silently virtualizes window coordinates
	// on mixed-DPI multi-monitor setups.
	win32.SetPerMonitorDPIAware()

	// A second launch (e.g. double-clicking the exe again) must not start
	// another tray icon with its own hotkeys and hooks fighting the first.
	if !win32.AcquireSingleInstance() {
		w32.MessageBox(0, "AppMover is already running.\nLook for its icon in the system tray.",
			"AppMover", w32.MB_OK|w32.MB_ICONINFORMATION)
		os.Exit(0)
	}

	systray.Run(tray.OnReady, tray.OnExit)
}
