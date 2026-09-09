; Inno Setup script for AppMover.
;
; Produces a per-user installer (no admin rights needed, no UAC prompt) —
; consistent with the app itself, which is entirely per-user by design
; (HKCU autostart toggle, %AppData%/%LocalAppData% config/state/logs).
;
; This is one of two release artifacts: the plain appmover.exe (portable,
; run from anywhere, no installer) and this Setup.exe (Start Menu shortcut,
; optional desktop shortcut, proper uninstaller). See .github/workflows/release.yml.
;
; The installer intentionally does NOT offer a "start with Windows" task:
; AppMover already manages that itself via a checkbox in its own tray menu
; (internal/autostart), and having two separate mechanisms toggle the same
; registry value would just be confusing.
;
; Build: expects the Go binary already built at ..\dist\appmover.exe
; (see ..\build.sh). Compile with ISCC (or the Minionguyjpro/Inno-Setup-Action
; in CI) from this directory, or pass /DMyAppVersion=X.Y.Z to stamp a real
; version instead of the dev default.

#define MyAppName "AppMover"
#ifndef MyAppVersion
  #define MyAppVersion "0.0.0-dev"
#endif
#define MyAppExeName "appmover.exe"

[Setup]
; {{GUID} is Inno Setup's documented literal-brace syntax for a fixed
; AppId — the leading "{{" is the compiler's escape for a literal "{"
; (not the preprocessor: a value substituted via #define would still be
; re-parsed as a "{constant}" reference and fail with "unknown
; constant", which is exactly what happened before this comment existed).
AppId={{6FEAD021-D263-40CC-A110-84CB573446E1}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
LicenseFile=..\LICENSE
SetupIconFile=..\internal\tray\icon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
OutputDir=output
OutputBaseFilename=AppMover-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "..\dist\appmover.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Best-effort: quit a running instance before uninstalling so the exe
; isn't locked. AppMover has no other IPC, so this is a plain taskkill.
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM {#MyAppExeName}"; Flags: runhidden skipifdoesntexist; RunOnceId: "KillAppMover"
