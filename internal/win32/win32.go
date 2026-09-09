//go:build windows

// Package win32 provides the Win32 APIs used to enumerate windows and monitors
// and to move/resize a window. Almost everything comes from
// github.com/gonutz/w32/v2 (actively maintained); GetShellWindow isn't in the
// package, so we declare it here by hand, same approach, no cgo.
package win32

import (
	"syscall"
	"unsafe"

	w32 "github.com/gonutz/w32/v2"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procGetShellWindow = user32.NewProc("GetShellWindow")

	procGetWindowDpiAwarenessContext = user32.NewProc("GetWindowDpiAwarenessContext")
	procAreDpiAwarenessContextsEqual = user32.NewProc("AreDpiAwarenessContextsEqual")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procQueryFullProcessImageNameW   = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")

	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")

	shcore               = syscall.NewLazyDLL("shcore.dll")
	procGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")
)

// dpiAwarenessContextPerMonitorAwareV2 mirrors the Win32 constant
// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, defined as ((DPI_AWARENESS_CONTEXT)-4).
// ^uintptr(3) is -4 in two's complement (-x == ^(x-1)), giving the same bit
// pattern as the real (sign-extended, pointer-sized) constant.
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// Monitor represents a connected display.
type Monitor struct {
	Handle    w32.HMONITOR
	Bounds    w32.RECT // total monitor area
	WorkArea  w32.RECT // usable area, excludes the taskbar
	IsPrimary bool
}

// EnumMonitors returns the list of connected monitors.
func EnumMonitors() []Monitor {
	var monitors []Monitor

	callback := syscall.NewCallback(func(hMonitor, _, _, _ uintptr) uintptr {
		var info w32.MONITORINFO
		info.CbSize = uint32(unsafe.Sizeof(info))

		if !w32.GetMonitorInfo(w32.HMONITOR(hMonitor), &info) {
			return 1 // keep enumerating anyway
		}

		monitors = append(monitors, Monitor{
			Handle:    w32.HMONITOR(hMonitor),
			Bounds:    info.RcMonitor,
			WorkArea:  info.RcWork,
			IsPrimary: info.DwFlags&w32.MONITORINFOF_PRIMARY != 0,
		})

		return 1 // continue the enumeration
	})

	w32.EnumDisplayMonitors(0, nil, callback, 0)
	return monitors
}

// EnumWindows iterates over all top-level system windows, invoking
// callback for each one. Filtering (system windows, tool windows, etc.)
// remains the caller's responsibility — we'll write it in step 3.
func EnumWindows(callback func(hwnd w32.HWND) bool) {
	w32.EnumWindows(callback)
}

// WindowText reads the window's title (empty string if it has none).
func WindowText(hwnd w32.HWND) string {
	return w32.GetWindowText(hwnd)
}

// ShellWindow returns the handle of the system shell window
// (always to be excluded from enumeration). Not wrapped by the library.
func ShellWindow() w32.HWND {
	h, _, _ := procGetShellWindow.Call()
	return w32.HWND(h)
}

// IsToolWindow reports whether the window is a "pure" tool window (to be
// excluded), i.e. it has WS_EX_TOOLWINDOW but not WS_EX_APPWINDOW.
func IsToolWindow(hwnd w32.HWND) bool {
	exStyle := w32.GetWindowLong(hwnd, w32.GWL_EXSTYLE)
	isTool := exStyle&w32.WS_EX_TOOLWINDOW != 0
	isApp := exStyle&w32.WS_EX_APPWINDOW != 0
	return isTool && !isApp
}

// WindowProcessID returns the PID of the process owning the window.
func WindowProcessID(hwnd w32.HWND) uint32 {
	_, pid := w32.GetWindowThreadProcessId(hwnd)
	return uint32(pid)
}

// IsMaximized reports whether the window is currently maximized.
func IsMaximized(hwnd w32.HWND) bool {
	var placement w32.WINDOWPLACEMENT
	if !w32.GetWindowPlacement(hwnd, &placement) {
		return false
	}
	return placement.ShowCmd == w32.SW_SHOWMAXIMIZED
}

// GetWindowRect returns the window's current screen coordinates.
func GetWindowRect(hwnd w32.HWND) (w32.RECT, bool) {
	r := w32.GetWindowRect(hwnd)
	if r == nil {
		return w32.RECT{}, false
	}
	return *r, true
}

// Restore restores a maximized/minimized window to its normal size.
func Restore(hwnd w32.HWND) {
	w32.ShowWindow(hwnd, w32.SW_RESTORE)
}

// Maximize maximizes the window on the monitor it currently sits on.
func Maximize(hwnd w32.HWND) {
	w32.ShowWindow(hwnd, w32.SW_MAXIMIZE)
}

// SetPosition moves and resizes the window without changing its z-order or activating it.
func SetPosition(hwnd w32.HWND, x, y, width, height int) {
	w32.SetWindowPos(hwnd, 0, x, y, width, height, w32.SWP_NOZORDER|w32.SWP_NOACTIVATE)
}

// Focus brings the window to the foreground. It reports whether Windows
// honored the request: SetForegroundWindow is allowed to silently refuse
// when the calling process isn't allowed to steal focus (a normal,
// expected occurrence for a background tray app in some situations), so
// callers should log a failure rather than treat it as fatal.
func Focus(hwnd w32.HWND) bool {
	return w32.SetForegroundWindow(hwnd)
}

// dwmwaCloaked is DWMWA_CLOAKED, the DWM window attribute that reports
// whether a window is cloaked (hidden from the screen by the shell even
// though it's WS_VISIBLE) — notably true for suspended/background UWP
// frame windows on Windows 10/11.
const dwmwaCloaked = 14

// IsCloaked reports whether the window is cloaked by DWM, i.e. carries
// WS_VISIBLE but isn't actually shown on screen. IsWindowVisible alone
// doesn't catch this. Defaults to false (not cloaked) if the query fails.
func IsCloaked(hwnd w32.HWND) bool {
	var cloaked uint32
	ret, _, _ := procDwmGetWindowAttribute.Call(
		uintptr(hwnd),
		uintptr(dwmwaCloaked),
		uintptr(unsafe.Pointer(&cloaked)),
		unsafe.Sizeof(cloaked),
	)
	const sOK = 0
	return ret == sOK && cloaked != 0
}

// MonitorFromWindow returns the handle of the monitor the window is
// currently (mostly) on, defaulting to the nearest monitor if the window
// straddles more than one or sits fully off-screen.
func MonitorFromWindow(hwnd w32.HWND) w32.HMONITOR {
	return w32.MonitorFromWindow(hwnd, w32.MONITOR_DEFAULTTONEAREST)
}

const mdtEffectiveDPI = 0 // MDT_EFFECTIVE_DPI

// GetDpiForMonitor returns the effective DPI of the monitor, or (96, false)
// — Windows' default DPI — if the query fails.
func GetDpiForMonitor(h w32.HMONITOR) (dpi uint32, ok bool) {
	var dpiX, dpiY uint32
	ret, _, _ := procGetDpiForMonitor.Call(
		uintptr(h),
		uintptr(mdtEffectiveDPI),
		uintptr(unsafe.Pointer(&dpiX)),
		uintptr(unsafe.Pointer(&dpiY)),
	)
	const sOK = 0
	if ret != sOK || dpiX == 0 {
		return 96, false
	}
	return dpiX, true
}

// IsPerMonitorDPIAware reports whether the window itself declares
// Per-Monitor-V2 DPI awareness, i.e. whether Windows expects *it* to
// handle rescaling itself when it crosses monitors with a different DPI
// (in which case AppMover shouldn't also apply its own DPI compensation).
// Requires Windows 10 1703+; returns false (assume not aware — the safer
// default for the fallback scaling logic) if the APIs aren't available.
func IsPerMonitorDPIAware(hwnd w32.HWND) bool {
	ctx, _, _ := procGetWindowDpiAwarenessContext.Call(uintptr(hwnd))
	if ctx == 0 {
		return false
	}
	equal, _, _ := procAreDpiAwarenessContextsEqual.Call(ctx, dpiAwarenessContextPerMonitorAwareV2)
	return equal != 0
}

// GetDpiForWindow returns the DPI Windows is currently rendering the
// window at, or (96, false) if the query fails (e.g. pre-Windows 10).
func GetDpiForWindow(hwnd w32.HWND) (dpi uint32, ok bool) {
	ret, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	if ret == 0 {
		return 96, false
	}
	return uint32(ret), true
}

// ProcessExeName returns the base file name (e.g. "chrome.exe") of the
// executable running under pid, or ("", false) if it can't be determined
// (the process may have exited, or AppMover may lack permission to query
// it — e.g. an elevated process while AppMover runs unelevated).
func ProcessExeName(pid uint32) (string, bool) {
	h := w32.OpenProcess(w32.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if h == 0 {
		return "", false
	}
	defer w32.CloseHandle(h)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(h),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", false
	}

	full := syscall.UTF16ToString(buf[:size])
	if full == "" {
		return "", false
	}

	// Base name only (path separators can be '\' or, rarely, '/').
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '\\' || full[i] == '/' {
			return full[i+1:], true
		}
	}
	return full, true
}

var procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")

// SetPerMonitorDPIAware opts the whole process into Per-Monitor-V2 DPI
// awareness. Must be called once, early in main(), before any window is
// created (including the systray's). Without it, Windows silently
// virtualizes window coordinates on machines with mixed-DPI monitors,
// making GetWindowRect/SetWindowPos report/apply the wrong physical
// pixels. Requires Windows 10 1703+; returns false (no-op) otherwise.
//
// This is the very first Win32 call AppMover makes, before the tray (and
// its logger) exist — unlike every other LazyProc call in this file, a
// missing DLL export here would panic with no diagnostics at all before
// the app ever shows an icon, so this one explicitly checks Find() first
// instead of relying on Call()'s normal (panicking) behavior.
func SetPerMonitorDPIAware() bool {
	if err := procSetProcessDpiAwarenessContext.Find(); err != nil {
		return false
	}
	ret, _, _ := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	return ret != 0
}
