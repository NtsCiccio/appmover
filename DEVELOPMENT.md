# Developing AppMover

Technical notes for building, testing, and releasing AppMover. For what the
app does and how to configure it, see the [README](README.md).

- [Building](#building)
- [Testing](#testing)
- [Releasing](#releasing)
- [Project layout](#project-layout)
- [How it works](#how-it-works)
- [Dependencies](#dependencies)

## Building

AppMover calls the Win32 API directly and only builds with `GOOS=windows`.
It has no cgo dependencies, so it cross-compiles from macOS or Linux:

```bash
./build.sh   # or: GOOS=windows GOARCH=amd64 go build -o dist/appmover.exe ./cmd/appmover
```

You need a Windows machine or VM to run the result. A Windows PE binary
won't execute on macOS or Linux.

On Windows directly:

```powershell
go build -o appmover.exe ./cmd/appmover
.\appmover.exe
```

Release builds add `-ldflags="-H=windowsgui"` (no console window) and set
the version with `-X appmover/internal/version.Version=1.2.3`. A plain
`go build` leaves the version at `"dev"`, which turns off the update check.

### Editor setup on macOS/Linux

Every file in `internal/win32` and the other Windows-only packages has
`//go:build windows`, so gopls reports broken imports unless it analyzes
the code as Windows. In VS Code, `.vscode/settings.json`:

```json
{
  "gopls": {
    "build.env": { "GOOS": "windows" }
  }
}
```

### Installer

`AppMover-Setup.exe` is built with [Inno Setup](https://jrsoftware.org/isinfo.php)
from `installer/appmover.iss`. It needs Windows and
[`ISCC.exe`](https://jrsoftware.org/isdl.php) (it can't be cross-compiled):

```powershell
go build -ldflags="-H=windowsgui" -o dist\appmover.exe .\cmd\appmover
iscc installer\appmover.iss
```

The installer sets `AppMutex=AppMover-SingleInstance`, the same mutex the app
holds while it runs, so setup and uninstall ask you to close a running
AppMover first.

> [!NOTE]
> We considered [Velopack](https://velopack.io/), but its `vpk` CLI needs the
> full .NET SDK whatever language the app is written in. That's a lot of
> extra toolchain for a project that otherwise builds with `go build`. Inno
> Setup is a single small compiler with no runtime dependency, and it's the
> usual choice for small Windows utilities like this one.

### App icon

`cmd/appmover/rsrc_windows_amd64.syso` is generated and committed. It embeds
`internal/tray/icon.ico` as the exe's resource, so Explorer, the taskbar,
shortcuts, and the installer all show the same icon as the tray. If you
change the icon, regenerate it:

```bash
go run github.com/tc-hib/go-winres@latest simply --icon internal/tray/icon.ico --arch amd64 --manifest none --file-description "AppMover" --product-name "AppMover" --out cmd/appmover/rsrc
```

## Testing

Packages with no Win32 dependency have unit tests that run on any OS:

```bash
go test ./internal/layout/... ./internal/config/... ./internal/state/... ./internal/applog/... ./internal/update/...
```

The rest of the code uses raw syscalls, COM, and a native message loop. It
can only be checked by compiling for Windows and running it there:

```bash
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=amd64 go build ./...
```

CI (`.github/workflows/ci.yml`) runs all of the above on every push and PR.

## Releasing

Pushing a `v*` tag triggers `.github/workflows/release.yml`. It builds
`AppMover-portable.exe` and `AppMover-Setup.exe` along with their `.sha256`
checksums, then publishes a GitHub Release with generated notes:

```bash
git push origin main
git tag vX.Y.Z
git push origin vX.Y.Z
```

The version comes from the tag name (`v1.2.3` → `1.2.3`), so no file needs
editing. Running copies pick up the new release at their next update check.

## Project layout

| Path | Purpose |
|---|---|
| `cmd/appmover` | Entry point: DPI awareness, single-instance check, tray startup. |
| `internal/tray` | Tray icon and menu, refresh, and the hotkey/autostart/state wiring. |
| `internal/win32` | Thin wrappers over Win32 (windows, monitors, positioning, DPI, cloaking, single-instance mutex), built on [`gonutz/w32`](https://github.com/gonutz/w32). |
| `internal/winlist` | Narrows the raw window handles down to real, user-visible app windows. |
| `internal/movewindow` | Places a window on a target monitor. |
| `internal/layout` | Geometry for centering and DPI scaling, with no Win32 dependency. |
| `internal/msgloop` | Hidden window with its own message loop for hotkeys, `WM_DISPLAYCHANGE`, and `SetWinEventHook`. The systray library keeps its loop private, so AppMover runs this separate one. |
| `internal/vdesktop` | Hand-written COM wrapper for `IVirtualDesktopManager`. |
| `internal/autostart` | Toggles the per-user `Run` registry key. |
| `internal/config`, `internal/state`, `internal/applog` | JSON config, per-app memory, and the rotating log. Pure Go. |
| `internal/update`, `internal/version` | Checks GitHub Releases for a newer version. |
| `installer/appmover.iss` | Inno Setup script. |

## How it works

- **Window filtering.** Windows that DWM has cloaked, such as suspended UWP
  frames on Windows 10/11, are skipped even though Win32 still marks them
  `WS_VISIBLE`. Windows that aren't on the active virtual desktop are
  dropped through the public `IVirtualDesktopManager` interface.
- **DPI.** The process declares Per-Monitor-V2 DPI awareness before any
  window exists. Positions are then computed in physical pixels, and sizes
  are scaled for windows that aren't DPI-aware themselves.
- **Refresh.** The menu updates on `SetWinEventHook` and `WM_DISPLAYCHANGE`
  events. The `refreshIntervalMs` poll is only a backup.
- **Last used monitor.** Stored per executable name in
  `%AppData%\AppMover\state.json`. It's only shown in the menu: AppMover
  never moves a window by itself.
- **Single instance.** At startup the app creates the named mutex
  `Local\AppMover-SingleInstance`, which belongs to the current user's
  session. If another copy already owns it, the new process shows a notice
  and exits.
- **Logging.** `%LocalAppData%\AppMover\logs\appmover.log`, rotated to a single
  `.log.old` file at 5 MB. The log isn't kept next to the exe because the
  app has no console (`-H=windowsgui`) and a regular user may not be able
  to write to the install directory. If that folder can't be opened, the
  log goes to `%Temp%\AppMover\appmover.log` instead.

## Dependencies

| Module | License |
|---|---|
| [`fyne.io/systray`](https://github.com/fyne-io/systray) | Apache-2.0 |
| [`github.com/gonutz/w32/v2`](https://github.com/gonutz/w32) | MIT |
| `github.com/godbus/dbus/v5`, `golang.org/x/sys` | BSD-3-Clause |

All of them are compatible with distribution under GPL-3.0.
