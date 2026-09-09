//go:build windows

// Package vdesktop wraps the public, stable IVirtualDesktopManager COM
// interface (available since Windows 10, documented at
// https://learn.microsoft.com/windows/win32/api/shobjidl_core/nn-shobjidl_core-ivirtualdesktopmanager)
// to answer "is this window on the currently active virtual desktop?".
//
// This is NOT the undocumented internal desktop-manager COM interface that
// tools like VirtualDesktopAccessor use (and that breaks on many Windows
// builds) — IVirtualDesktopManager is a small, public, versioned interface
// meant exactly for this kind of query.
//
// There is no cgo and no external COM library here: the vtable is
// hand-written from the interface's documented method order. This is the
// riskiest hand-written code in AppMover (COM interop written without the
// ability to run it on real Windows) — every call is guarded and fails
// open (see New and IsOnCurrentDesktop) so a mistake here disables the
// feature instead of crashing or misbehaving.
package vdesktop

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var errClosed = errors.New("vdesktop: manager closed")

// guid mirrors the Win32 GUID layout (16 bytes, same field order as the C struct).
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	// CLSID_VirtualDesktopManager
	clsidVirtualDesktopManager = guid{0xAA509086, 0x5CA9, 0x4C25, [8]byte{0x8F, 0x95, 0x58, 0x9D, 0x3C, 0x07, 0xB4, 0x8A}}
	// IID_IVirtualDesktopManager
	iidIVirtualDesktopManager = guid{0xA5CD92FF, 0x29BE, 0x454C, [8]byte{0x8D, 0x04, 0xD8, 0x28, 0x79, 0xFB, 0x3F, 0x1B}}
)

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	sOK                     = 0
)

var (
	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
)

// ivdmVtbl mirrors IVirtualDesktopManager's vtable: the 3 inherited
// IUnknown methods followed by the interface's own 3, in declared order
// (verified against the interface's C# COM-interop projection, which
// preserves native vtable order).
type ivdmVtbl struct {
	QueryInterface                  uintptr
	AddRef                          uintptr
	Release                         uintptr
	IsWindowOnCurrentVirtualDesktop uintptr
	GetWindowDesktopId              uintptr
	MoveWindowToDesktop             uintptr
}

type ivdmObject struct {
	vtbl *ivdmVtbl
}

func (o *ivdmObject) release() {
	syscall.SyscallN(o.vtbl.Release, uintptr(unsafe.Pointer(o)))
}

func (o *ivdmObject) isWindowOnCurrentVirtualDesktop(hwnd uintptr) (bool, error) {
	var onCurrent int32 // BOOL
	hr, _, _ := syscall.SyscallN(
		o.vtbl.IsWindowOnCurrentVirtualDesktop,
		uintptr(unsafe.Pointer(o)),
		hwnd,
		uintptr(unsafe.Pointer(&onCurrent)),
	)
	if int32(hr) != sOK {
		return false, fmt.Errorf("IsWindowOnCurrentVirtualDesktop: hresult %#x", uint32(hr))
	}
	return onCurrent != 0, nil
}

// Manager serializes all COM calls onto one dedicated, locked OS thread —
// required because CoInitializeEx(APARTMENTTHREADED) makes the resulting
// object thread-affine, and Go goroutines otherwise migrate between OS
// threads freely.
type Manager struct {
	requests chan func(*ivdmObject)
	closed   chan struct{}
}

// New initializes COM on a dedicated goroutine/thread and creates the
// VirtualDesktopManager COM object. If anything fails (COM unavailable,
// interface missing on this Windows version, etc.) it returns a non-nil
// error and callers should simply not use virtual-desktop filtering —
// nothing else in AppMover depends on this package.
func New() (*Manager, error) {
	m := &Manager{
		requests: make(chan func(*ivdmObject)),
		closed:   make(chan struct{}),
	}

	ready := make(chan error, 1)
	go m.run(ready)

	if err := <-ready; err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, uintptr(coinitApartmentThreaded))
	// RPC_E_CHANGED_MODE (0x80010106) means this OS thread already has COM
	// initialized in a different mode; treat as fatal for simplicity since
	// nothing else in AppMover uses COM on this thread.
	if int32(hr) != sOK && int32(hr) != 1 /* S_FALSE: already initialized, same mode */ {
		ready <- fmt.Errorf("vdesktop: CoInitializeEx: hresult %#x", uint32(hr))
		return
	}
	defer procCoUninitialize.Call()

	var objPtr unsafe.Pointer
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidVirtualDesktopManager)),
		0,
		uintptr(clsctxInprocServer),
		uintptr(unsafe.Pointer(&iidIVirtualDesktopManager)),
		uintptr(unsafe.Pointer(&objPtr)),
	)
	if int32(hr) != sOK || objPtr == nil {
		ready <- fmt.Errorf("vdesktop: CoCreateInstance(VirtualDesktopManager): hresult %#x", uint32(hr))
		return
	}
	obj := (*ivdmObject)(objPtr)
	defer obj.release()

	ready <- nil

	for {
		select {
		case fn := <-m.requests:
			fn(obj)
		case <-m.closed:
			return
		}
	}
}

// IsOnCurrentDesktop reports whether hwnd is on the currently active
// virtual desktop. On any error (COM call failure, manager closed) it
// fails open, returning (true, err): callers should treat the window as
// visible rather than hide it based on an inconclusive answer.
func (m *Manager) IsOnCurrentDesktop(hwnd uintptr) (bool, error) {
	type result struct {
		onCurrent bool
		err       error
	}
	done := make(chan result, 1)

	req := func(obj *ivdmObject) {
		onCurrent, err := obj.isWindowOnCurrentVirtualDesktop(hwnd)
		done <- result{onCurrent, err}
	}

	select {
	case m.requests <- req:
	case <-m.closed:
		return true, errClosed
	}

	select {
	case r := <-done:
		if r.err != nil {
			return true, r.err
		}
		return r.onCurrent, nil
	case <-m.closed:
		return true, errClosed
	}
}

// Close shuts down the COM thread.
func (m *Manager) Close() {
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
}
