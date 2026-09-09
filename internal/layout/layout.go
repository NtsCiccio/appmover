// Package layout contains pure geometry math shared by internal/movewindow.
// It has no Win32 dependency so it builds and tests on any platform.
package layout

// Rect is a plain, platform-independent rectangle.
type Rect struct {
	X, Y, W, H int32
}

// Centered computes where to place a window of size (w, h) so it sits
// centered within the given work area, clamping the size down if it
// wouldn't otherwise fit. dpiScale multiplies the window size before
// clamping/centering (pass 1 for no scaling); it is meant to compensate a
// target window that is not itself per-monitor DPI aware and would
// otherwise end up the wrong physical size after crossing monitors with a
// different DPI. Non-positive dpiScale values are treated as 1 (no-op).
func Centered(w, h int32, workX, workY, workW, workH int32, dpiScale float64) Rect {
	if dpiScale > 0 && dpiScale != 1 {
		w = int32(float64(w) * dpiScale)
		h = int32(float64(h) * dpiScale)
	}

	if w > workW {
		w = workW
	}
	if h > workH {
		h = workH
	}

	x := workX + (workW-w)/2
	y := workY + (workH-h)/2

	return Rect{X: x, Y: y, W: w, H: h}
}
