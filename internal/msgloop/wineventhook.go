//go:build windows

package msgloop

import "syscall"

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procSetWinEventHook  = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent   = user32.NewProc("UnhookWinEvent")
)

const (
	winEventOutOfContext   = 0x0000
	winEventSkipOwnProcess = 0x0002

	eventObjectCreate     = 0x8000
	eventObjectDestroy    = 0x8001
	eventObjectShow       = 0x8002
	eventObjectHide       = 0x8003
	eventObjectNameChange = 0x800C

	objidWindow = 0
	childidSelf = 0
)

// installWinEventHook subscribes to the window-lifecycle events that mean
// "the set of open windows or a title probably changed", filtered down
// inside winEventProc to just those (the subscribed range also carries
// high-frequency events like EVENT_OBJECT_LOCATIONCHANGE that we ignore,
// or every window drag/resize — including our own moves — would trigger a
// refresh).
func (l *Loop) installWinEventHook() {
	l.winEventProc = syscall.NewCallback(l.onWinEvent)
	hook, _, _ := procSetWinEventHook.Call(
		uintptr(eventObjectCreate),
		uintptr(eventObjectNameChange),
		0,
		l.winEventProc,
		0, 0,
		uintptr(winEventOutOfContext|winEventSkipOwnProcess),
	)
	l.winEventHook = hook
}

func (l *Loop) uninstallWinEventHook() {
	if l.winEventHook != 0 {
		procUnhookWinEvent.Call(l.winEventHook)
		l.winEventHook = 0
	}
}

// onWinEvent is the WINEVENTPROC callback. It must return quickly and must
// not block: it runs on the message-loop thread, invoked by Windows while
// that thread pumps its message queue.
func (l *Loop) onWinEvent(hWinEventHook uintptr, event uint32, hwnd uintptr, idObject, idChild int32, idEventThread, idEventTime uint32) uintptr {
	if idObject != objidWindow || idChild != childidSelf {
		return 0
	}
	switch event {
	case eventObjectCreate, eventObjectDestroy, eventObjectShow, eventObjectHide, eventObjectNameChange:
		nonBlockingSend(l.rawWindowEvents, struct{}{})
	}
	return 0
}
