//go:build windows

// Package msgloop runs a dedicated hidden window with its own Win32
// message loop, separate from the one fyne.io/systray creates internally
// (which is private to that library and exposes no way to hook additional
// messages). RegisterHotKey/WM_HOTKEY and SetWinEventHook(WINEVENT_OUTOFCONTEXT)
// both require a message loop pumping on the same OS thread that
// registered them, and WM_DISPLAYCHANGE requires an actual top-level
// window — this package provides all three, mirroring the same hidden
// "create it, then SW_HIDE it" pattern systray itself uses.
package msgloop

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	w32 "github.com/gonutz/w32/v2"
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmDisplayChange = 0x007E
	wmHotkey        = 0x0312
)

// Event is a coalesced signal sent on Loop.Events.
type Event int

const (
	// EventDisplayChanged fires on WM_DISPLAYCHANGE: monitor
	// configuration (resolution, count, arrangement) changed.
	EventDisplayChanged Event = iota
	// EventWindowsChanged fires (debounced) when the set of open windows
	// or their titles/visibility likely changed, per SetWinEventHook.
	EventWindowsChanged
)

// eventDebounce coalesces bursts of raw window events (e.g. an app
// creating several windows at startup) into a single refresh signal.
const eventDebounce = 150 * time.Millisecond

// Loop owns a hidden window + message pump running for the lifetime of the
// process on a dedicated, locked OS thread.
type Loop struct {
	hwnd      w32.HWND
	className *uint16
	instance  w32.HINSTANCE

	// Events receives coalesced EventDisplayChanged/EventWindowsChanged
	// signals. Sends are non-blocking (buffered, coalescing) so a slow
	// consumer never stalls the Win32 message pump.
	Events chan Event
	// Hotkeys receives the ids of triggered hotkeys registered via
	// RegisterHotKey.
	Hotkeys chan int

	rawWindowEvents chan struct{}
	winEventHook    uintptr
	winEventProc    uintptr

	closeOnce sync.Once
	closed    chan struct{}
}

// Start creates the hidden window, installs the WinEventHook, and begins
// pumping messages in a background goroutine. It blocks until the window
// is ready (or creation failed).
func Start() (*Loop, error) {
	l := &Loop{
		Events:          make(chan Event, 4),
		Hotkeys:         make(chan int, 4),
		rawWindowEvents: make(chan struct{}, 1),
		closed:          make(chan struct{}),
	}

	ready := make(chan error, 1)
	go l.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}

	go l.debounceWindowEvents()

	return l, nil
}

func (l *Loop) run(ready chan<- error) {
	// The window and every Win32 call that touches it must happen on the
	// same OS thread for the whole lifetime of the message loop.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const className = "AppMoverMsgLoop"

	classNamePtr, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		ready <- fmt.Errorf("msgloop: class name: %w", err)
		return
	}
	l.className = classNamePtr

	instance := w32.GetModuleHandle("")
	l.instance = instance

	wcex := w32.WNDCLASSEX{
		Style:     0,
		WndProc:   syscall.NewCallback(l.wndProc),
		Instance:  instance,
		ClassName: classNamePtr,
	}
	wcex.Size = uint32(unsafe.Sizeof(wcex))

	if atom := w32.RegisterClassEx(&wcex); atom == 0 {
		ready <- fmt.Errorf("msgloop: RegisterClassEx failed: error %d", w32.GetLastError())
		return
	}

	windowNamePtr, _ := syscall.UTF16PtrFromString("")
	hwnd := w32.CreateWindowEx(
		0,
		classNamePtr,
		windowNamePtr,
		w32.WS_OVERLAPPEDWINDOW,
		w32.CW_USEDEFAULT, w32.CW_USEDEFAULT, w32.CW_USEDEFAULT, w32.CW_USEDEFAULT,
		0, 0, instance, nil,
	)
	if hwnd == 0 {
		w32.UnregisterClass(className, instance)
		ready <- fmt.Errorf("msgloop: CreateWindowEx failed: error %d", w32.GetLastError())
		return
	}
	l.hwnd = hwnd

	w32.ShowWindow(hwnd, w32.SW_HIDE)
	w32.UpdateWindow(hwnd)

	l.installWinEventHook()

	ready <- nil

	var msg w32.MSG
	for {
		r := w32.GetMessage(&msg, 0, 0, 0)
		if r <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		w32.TranslateMessage(&msg)
		w32.DispatchMessage(&msg)
	}

	l.uninstallWinEventHook()
	w32.UnregisterClass(className, instance)
	close(l.closed)
}

func (l *Loop) wndProc(hwnd w32.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmDisplayChange:
		nonBlockingSend(l.Events, EventDisplayChanged)
	case wmHotkey:
		nonBlockingSend(l.Hotkeys, int(wParam))
	case wmClose:
		w32.DestroyWindow(hwnd)
	case wmDestroy:
		w32.PostQuitMessage(0)
	default:
		return w32.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	return 0
}

func nonBlockingSend[T any](ch chan T, v T) {
	select {
	case ch <- v:
	default:
	}
}

// debounceWindowEvents coalesces bursts of raw WinEventHook callbacks into
// a single, rate-limited EventWindowsChanged signal.
func (l *Loop) debounceWindowEvents() {
	var timer *time.Timer
	var fire <-chan time.Time

	for {
		select {
		case <-l.rawWindowEvents:
			if timer == nil {
				timer = time.NewTimer(eventDebounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(eventDebounce)
			}
			fire = timer.C
		case <-fire:
			nonBlockingSend(l.Events, EventWindowsChanged)
			fire = nil
		case <-l.closed:
			return
		}
	}
}

// RegisterHotKey registers a system-wide hotkey; triggering it sends id on
// l.Hotkeys. modifiers/vk are the Win32 MOD_*/VK_* values (see
// internal/config.ParseHotkey).
func (l *Loop) RegisterHotKey(id int, modifiers, vk uint32) error {
	ok, _, err := procRegisterHotKey.Call(uintptr(l.hwnd), uintptr(id), uintptr(modifiers), uintptr(vk))
	if ok == 0 {
		return fmt.Errorf("RegisterHotKey(id=%d): %w", id, err)
	}
	return nil
}

// UnregisterHotKey undoes a previous RegisterHotKey.
func (l *Loop) UnregisterHotKey(id int) {
	procUnregisterHotKey.Call(uintptr(l.hwnd), uintptr(id))
}

// Close asks the message loop to shut down and waits for it to do so.
func (l *Loop) Close() {
	l.closeOnce.Do(func() {
		w32.PostMessage(l.hwnd, wmClose, 0, 0)
		<-l.closed
	})
}
