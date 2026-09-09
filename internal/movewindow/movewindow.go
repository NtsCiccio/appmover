//go:build windows

package movewindow

import (
	"appmover/internal/layout"
	"appmover/internal/win32"

	w32 "github.com/gonutz/w32/v2"
)

// MoveToScreen moves the "target" window to the given monitor, centering
// it in the work area (taskbar excluded) and preserving its maximized
// state if it had one.
//
// moved is false, and nothing happens, if target is no longer a valid
// window, or if the process owning target no longer matches expectedPID —
// the latter guards against the rare case where Windows recycled the HWND
// for a different window between the last menu refresh and this click
// (see internal/tray for where expectedPID comes from).
//
// focused is false when the move itself succeeded but Windows refused the
// final SetForegroundWindow call — a known, sometimes-expected occurrence
// for background apps (focus-stealing prevention), not a failure of the
// move itself. Callers should log it, not treat it as an error.
func MoveToScreen(target w32.HWND, expectedPID uint32, monitor win32.Monitor) (moved, focused bool) {
	if win32.WindowProcessID(target) != expectedPID {
		return false, false
	}

	wasMaximized := win32.IsMaximized(target)
	if wasMaximized {
		win32.Restore(target)
	}

	rect, ok := win32.GetWindowRect(target)
	if !ok {
		return false, false
	}

	dpiScale := dpiCompensation(target, monitor)

	dst := layout.Centered(
		rect.Width(), rect.Height(),
		monitor.WorkArea.Left, monitor.WorkArea.Top, monitor.WorkArea.Width(), monitor.WorkArea.Height(),
		dpiScale,
	)

	win32.SetPosition(target, int(dst.X), int(dst.Y), int(dst.W), int(dst.H))

	if wasMaximized {
		win32.Maximize(target)
	}

	focused = win32.Focus(target)
	return true, focused
}

// dpiCompensation returns the size-scaling factor MoveToScreen should
// apply so target doesn't end up the wrong physical size after crossing
// monitors with different DPI. If the window itself declares Per-Monitor-V2
// DPI awareness, Windows already handles this for it, so no compensation
// is applied (scale 1). Falls back to 1 (no-op) if any DPI query fails.
func dpiCompensation(target w32.HWND, dstMonitor win32.Monitor) float64 {
	if win32.IsPerMonitorDPIAware(target) {
		return 1
	}

	srcMonitor := win32.MonitorFromWindow(target)
	srcDPI, srcOK := win32.GetDpiForMonitor(srcMonitor)
	dstDPI, dstOK := win32.GetDpiForMonitor(dstMonitor.Handle)
	if !srcOK || !dstOK || srcDPI == 0 {
		return 1
	}
	return float64(dstDPI) / float64(srcDPI)
}
