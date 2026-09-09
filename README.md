# AppMover

A lightweight Windows system tray utility to move open application windows
between monitors.

AppMover sits in the system tray and lists the currently open windows. For
each window you can pick a target monitor from a submenu; the window is
moved there, centered in the usable work area (taskbar excluded), and its
maximized/restored state is preserved. The menu refreshes itself as windows
open/close/change title, and updates immediately when you plug/unplug a
monitor.

## Features

- **Global hotkeys** — `Win+Shift+Right` / `Win+Shift+Left` move the
  currently focused window to the next/previous monitor without opening
  the tray menu. Configurable, see [Configuration](#configuration).
- **"Last used monitor" memory** — the submenu entry an app was last moved
  to is shown checked, per app (matched by executable name), so you can
  see at a glance where something usually goes. This is purely
  informational: AppMover never moves a window on its own.
- **Start with Windows** — a checkbox in the tray menu toggles autostart
  via the per-user registry Run key (no admin rights needed).
- **Cloaked/UWP window filtering** — windows DWM has hidden from the
  screen (common for suspended/background UWP frame windows on Windows
  10/11) are not listed, even though Win32 still reports them as
  `WS_VISIBLE`.
- **Virtual desktop awareness** — windows that aren't on the currently
  active virtual desktop are filtered out, via the public
  `IVirtualDesktopManager` COM interface.
- **Per-monitor DPI awareness** — AppMover declares Per-Monitor-V2 DPI
  awareness so window positions/sizes are computed in real physical
  pixels on mixed-DPI multi-monitor setups, with a size-scaling fallback
  for target windows that aren't themselves DPI-aware.
- **Event-driven refresh** — the window list updates via
  `SetWinEventHook`/`WM_DISPLAYCHANGE` rather than blind polling; a much
  longer interval poll (configurable, default 10s) remains only as a
  safety net.

## Requirements

- Windows (the app uses the Win32 API directly and only builds with
  `GOOS=windows`)
- Go 1.27+ to build from source

## Building

The project has no cgo dependencies, so it cross-compiles cleanly from any
platform, e.g. from macOS or Linux:

```bash
./build.sh   # or: GOOS=windows GOARCH=amd64 go build -o dist/appmover.exe ./cmd/appmover
```

Run `dist/appmover.exe` on a Windows machine (or VM) to test it — a native
macOS or Linux binary cannot execute a Windows PE binary.

To build and run on Windows directly:

```powershell
go build -o appmover.exe ./cmd/appmover
.\appmover.exe
```

## Releases

Every `v*` tag produces two artifacts (`.github/workflows/release.yml`):

- **`AppMover-portable.exe`** — the plain binary from the build step above,
  run from anywhere, nothing installed.
- **`AppMover-Setup.exe`** — an [Inno Setup](https://jrsoftware.org/isinfo.php)
  installer (`installer/appmover.iss`) that installs per-user to
  `%LocalAppData%\Programs\AppMover` — no admin rights, no UAC prompt —
  with a Start Menu entry, an optional desktop shortcut, and a proper
  uninstaller. It deliberately doesn't offer its own "start with Windows"
  option: use the one already in AppMover's tray menu instead, so there's
  only one place that toggles autostart.

  To build the installer locally you need Windows and
  [Inno Setup](https://jrsoftware.org/isdl.php) (`ISCC.exe`); it's not
  something that can be produced by cross-compiling from macOS/Linux:

  ```powershell
  go build -ldflags="-H=windowsgui" -o dist\appmover.exe .\cmd\appmover
  iscc installer\appmover.iss
  ```

We looked at [Velopack](https://velopack.io/) (a newer installer/auto-update
framework) as an alternative, but its `vpk` CLI requires the full .NET SDK
to run regardless of the target app's language — a disproportionate
toolchain addition for a project that otherwise builds with nothing but
`go build`. Inno Setup is a single small compiler binary, has no runtime
dependency, and is the long-established standard for exactly this kind of
small Windows utility.

## Configuration

On first run AppMover creates a config file at
`%AppData%\AppMover\config.json` with defaults:

```json
{
  "refreshIntervalMs": 10000,
  "moveNextMonitorHotkey": "win+shift+right",
  "movePrevMonitorHotkey": "win+shift+left",
  "maxWindowSlots": 20,
  "excludedProcessNames": [],
  "excludedTitles": []
}
```

- `refreshIntervalMs` — the safety-net poll interval; the menu also
  refreshes immediately on real window/display changes regardless of this
  value.
- `moveNextMonitorHotkey` / `movePrevMonitorHotkey` — `modifier+...+key`,
  where modifiers are any of `win`/`ctrl`/`alt`/`shift` and the key is an
  arrow (`left`/`right`/`up`/`down`) or a single letter/digit.
- `maxWindowSlots` — how many window entries the tray menu pre-allocates;
  beyond that, a trailing "+N more windows not shown" entry appears
  instead of silently truncating the list.
- `excludedProcessNames` / `excludedTitles` — case-insensitive substring
  filters (e.g. `"excludedProcessNames": ["chrome.exe"]`).

Edit the file and restart AppMover to apply changes. Per-app "last used
monitor" memory is stored separately at `%AppData%\AppMover\state.json`.

## Project layout

- `cmd/appmover` — application entry point; sets process DPI awareness
- `internal/tray` — system tray icon, menu, refresh, and wiring for
  hotkeys/autostart/state
- `internal/win32` — thin wrappers around the Win32 APIs
  (window/monitor enumeration, positioning, DPI, cloaking) built on
  [`github.com/gonutz/w32/v2`](https://github.com/gonutz/w32)
- `internal/winlist` — filters raw window handles down to real,
  user-visible application windows
- `internal/movewindow` — the move/resize logic that places a window on a
  target monitor
- `internal/layout` — pure geometry math (centering/DPI scaling), no Win32
  dependency, builds and tests on any platform
- `internal/msgloop` — a dedicated hidden window + Win32 message loop
  (separate from systray's own, which is private to that library) for
  global hotkeys, `WM_DISPLAYCHANGE`, and `SetWinEventHook`-driven refresh
- `internal/vdesktop` — hand-written COM wrapper around
  `IVirtualDesktopManager` for virtual-desktop filtering
- `internal/autostart` — toggles the per-user Run registry key
- `internal/config`, `internal/state`, `internal/applog` — JSON
  settings/memory persistence and the rotating debug log, pure Go, no
  Win32 dependency
- `installer/appmover.iss` — Inno Setup script for `AppMover-Setup.exe`
  (see [Releases](#releases))

## Testing

Packages with no Win32 dependency (`internal/layout`, `internal/config`,
`internal/state`, `internal/applog`) have real unit tests that run on any
platform:

```bash
go test ./internal/layout/... ./internal/config/... ./internal/state/... ./internal/applog/...
```

Everything else is Windows-only by nature (raw syscalls, COM, a native
message loop) and can only be verified by compiling for the target and
exercising it on real Windows:

```bash
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build ./...
```

CI (`.github/workflows/ci.yml`) runs both of the above on every push/PR;
`.github/workflows/release.yml` builds and publishes `appmover.exe` on
`v*` tags.

## Logging

At runtime AppMover writes a debug log to
`%LocalAppData%\AppMover\logs\appmover.log`, rotating it (one `.log.old`
backup) once it passes 5MB. This deliberately isn't next to the
executable: AppMover builds with `-H=windowsgui` (no console, so stderr
would be invisible anyway), and the install directory may not be writable
by a regular user once properly installed. If even that location can't be
opened, it falls back to `%Temp%\AppMover\appmover.log`.

## License

GPL-3.0 — see [LICENSE](LICENSE).

This project depends on:

- [`fyne.io/systray`](https://github.com/fyne-io/systray) — Apache-2.0
- [`github.com/gonutz/w32/v2`](https://github.com/gonutz/w32) — MIT
- `github.com/godbus/dbus/v5`, `golang.org/x/sys` — BSD-3-Clause

All of these are permissive licenses compatible with distribution under
GPL-3.0.
