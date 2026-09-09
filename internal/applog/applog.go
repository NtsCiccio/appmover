// Package applog provides AppMover's debug log: a small, rotating file
// written to a fixed, always-user-writable location outside the
// installation directory.
//
// This matters because AppMover builds with -H=windowsgui (no console),
// so anything written to stderr is invisible — there's no terminal to
// show it — and the executable's own directory may not be writable once
// AppMover is properly installed (e.g. under Program Files). Pure Go, no
// Win32 dependency, so it builds and tests on any platform.
package applog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// MaxSizeBytes is the size at which the log file is rotated: renamed to
// "appmover.log.old" (overwriting any previous one) and started fresh.
// AppMover is a small, low-volume logger, so a single backup is enough
// headroom without ever growing unbounded across a long-running session.
const MaxSizeBytes = 5 * 1024 * 1024 // 5MB

const (
	dirName  = "AppMover"
	fileName = "appmover.log"
)

// Path returns the primary log location: <UserCacheDir>/AppMover/logs/appmover.log.
// UserCacheDir (not UserConfigDir, which internal/config and internal/state
// use) is the right place for this — logs are local, disposable, and
// shouldn't roam between machines the way settings do.
func Path() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dirName, "logs", fileName), nil
}

// fallbackPath returns a secondary location to try if Path() isn't
// writable for some reason: the system temp directory, which is
// essentially always writable by the current user.
func fallbackPath() string {
	return filepath.Join(os.TempDir(), dirName, fileName)
}

// Open opens (creating directories as needed, rotating if the file has
// grown past MaxSizeBytes) AppMover's log file and returns a ready-to-use
// *log.Logger. It tries Path() first, then fallbackPath(); it only
// returns an error if both fail.
func Open() (*log.Logger, error) {
	firstErr := error(nil)

	if primary, err := Path(); err != nil {
		firstErr = err
	} else if l, err := openAt(primary); err == nil {
		return l, nil
	} else {
		firstErr = err
	}

	if l, err := openAt(fallbackPath()); err == nil {
		return l, nil
	} else if firstErr == nil {
		firstErr = err
	}

	return nil, fmt.Errorf("applog: could not open a log file anywhere: %w", firstErr)
}

func openAt(path string) (*log.Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := rotateIfNeeded(path); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return log.New(f, "", log.LstdFlags), nil
}

// rotateIfNeeded renames path to path+".old" (replacing any previous
// backup) if it has grown past MaxSizeBytes. A missing file is not an
// error — there's simply nothing to rotate yet.
func rotateIfNeeded(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() < MaxSizeBytes {
		return nil
	}

	backup := path + ".old"
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(path, backup)
}
