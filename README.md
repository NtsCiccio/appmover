<div align="center">

<img src=".github/icon.png" alt="AppMover icon" width="72" height="72">

# AppMover

**Move windows between monitors in one click, right from the system tray.**

[![CI](https://github.com/NtsCiccio/appmover/actions/workflows/ci.yml/badge.svg)](https://github.com/NtsCiccio/appmover/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/NtsCiccio/appmover?label=release)](https://github.com/NtsCiccio/appmover/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Windows](https://img.shields.io/badge/platform-Windows-0078D6?logo=windows&logoColor=white)](#download)

</div>

AppMover lives in the system tray and lists your open windows. Pick one,
pick a monitor, and the window moves there, centered in the usable area and
still maximized if it was before. You don't need to drag it across screens
or have admin rights.

## Features

- ⌨️ **Global hotkeys**: `Win+Shift+Right` / `Win+Shift+Left` send the focused window to the next or previous monitor.
- 🧠 **Remembers where apps go**: the monitor each app was last moved to is checked in its submenu.
- 🚀 **Start with Windows**: one checkbox in the tray menu, no admin rights.
- 🖥️ **Shows only real windows**: hidden, background UWP, and other-virtual-desktop windows aren't listed.
- 🔍 **Mixed-DPI aware**: sizes and positions stay correct across monitors with different scaling.
- 🔔 **Update notice**: a tray menu entry appears when a new release is out.
- 1️⃣ **Single instance**: launching it again while it's running just tells you it's already in the tray.

## Download

Get the latest version from the [Releases page](https://github.com/NtsCiccio/appmover/releases/latest):

| File | What it is |
|---|---|
| **`AppMover-Setup.exe`** *(recommended)* | Installs for your user only (no admin, no UAC prompt), with a Start Menu entry and an uninstaller. |
| **`AppMover-portable.exe`** | The plain exe. Run it from anywhere, nothing gets installed. |

To start AppMover with Windows, use the checkbox in its tray menu. The
installer deliberately doesn't offer this, so the setting lives in one place.

## Configuration

On first run AppMover creates `%AppData%\AppMover\config.json`:

```json
{
  "refreshIntervalMs": 10000,
  "moveNextMonitorHotkey": "win+shift+right",
  "movePrevMonitorHotkey": "win+shift+left",
  "maxWindowSlots": 20,
  "excludedProcessNames": [],
  "excludedTitles": [],
  "disableUpdateCheck": false
}
```

| Key | Description |
|---|---|
| `moveNextMonitorHotkey` / `movePrevMonitorHotkey` | `modifier+...+key`. Modifiers: `win`, `ctrl`, `alt`, `shift`. Key: an arrow (`left`/`right`/`up`/`down`) or one letter/digit. |
| `excludedProcessNames` / `excludedTitles` | Hide matching windows from the menu (case-insensitive substring), e.g. `["chrome.exe"]`. |
| `maxWindowSlots` | Maximum number of windows the menu lists. Any extra windows are summed up in a "+N more" entry. |
| `refreshIntervalMs` | Backup refresh interval. The menu already updates as soon as windows change. |
| `disableUpdateCheck` | `true` stops AppMover from checking GitHub for new releases. |

Restart AppMover after editing the file. If something misbehaves, check the
log at `%LocalAppData%\AppMover\logs\appmover.log`.

## Building from source

The project has no cgo, so it cross-compiles from any OS with Go 1.27+:

```bash
./build.sh   # outputs dist/appmover.exe
```

Building the installer, the project layout, tests, and the release process
are covered in **[DEVELOPMENT.md](DEVELOPMENT.md)**.

## License

GPL-3.0. See [LICENSE](LICENSE). All dependencies use permissive licenses
(Apache-2.0, MIT, BSD-3-Clause) compatible with GPL-3.0. They are listed in
[DEVELOPMENT.md](DEVELOPMENT.md#dependencies).
