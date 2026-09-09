package main

import (
	"appmover/internal/tray"
	"appmover/internal/win32"

	"fyne.io/systray"
)

func main() {
	// Must happen before any window (including the systray's own) is
	// created, otherwise Windows silently virtualizes window coordinates
	// on mixed-DPI multi-monitor setups.
	win32.SetPerMonitorDPIAware()

	systray.Run(tray.OnReady, tray.OnExit)
}
