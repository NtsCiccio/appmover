//go:build windows

package win32

import (
	"errors"

	"golang.org/x/sys/windows"
)

// singleInstanceMutexName identifies the running AppMover instance. The
// "Local\" prefix scopes it to the current logon session, so two users
// logged in at the same time (fast user switching, RDP) can each run
// their own copy.
const singleInstanceMutexName = `Local\AppMover-SingleInstance`

// singleInstanceMutex holds the handle for the whole life of the process.
// It is never closed on purpose: Windows releases it when the process
// exits, and that is what lets the next launch start normally.
var singleInstanceMutex windows.Handle

// AcquireSingleInstance reports whether this is the only AppMover running
// in the current session. If another instance already holds the mutex it
// returns false, and the caller should exit right away.
//
// Like SetPerMonitorDPIAware, this runs before the tray and its logger
// exist. If the mutex can't be created for some other reason, it returns
// true: starting a duplicate is better than not starting at all.
func AcquireSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(singleInstanceMutexName)

	if err != nil {
		return true
	}

	h, err := windows.CreateMutex(nil, false, name)

	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}

	if err != nil {
		return true
	}

	singleInstanceMutex = h
	return true

}
