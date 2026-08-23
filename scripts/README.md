# VibeDock build scripts

Each script performs a native build on its target operating system. Wails uses
platform WebViews and native libraries, so these scripts intentionally do not
cross-compile GUI binaries from another OS.

## Requirements

- Go 1.25 or newer
- Node.js 20 or newer, including npm
- Git (for obtaining the source)
- `vibe-acp` is required at runtime, but not to compile the application

The scripts build the pinned Wails Go dependency directly, so a global `wails3`
installation is not necessary. Frontend dependencies are installed reproducibly
with the committed npm lockfile.

## Commands

```sh
# macOS; creates bin/VibeDock.app and applies an ad-hoc signature
./scripts/build-macos.sh

# Use an Apple Developer ID instead of the ad-hoc signature
MACOS_SIGN_IDENTITY="Developer ID Application: Example (TEAMID)" ./scripts/build-macos.sh

# Ubuntu; install native GTK/WebKit dependencies and build bin/vibedock
./scripts/build-ubuntu.sh --install-deps

# Fedora; install native GTK/WebKit dependencies and build bin/vibedock
./scripts/build-fedora.sh --install-deps
```

From PowerShell on Windows:

```powershell
.\scripts\build-windows.ps1
```

Pass `--skip-tests` on macOS/Linux or `-SkipTests` on Windows for a faster local
rebuild. Release builds should keep the verification step enabled. Windows
requires the Microsoft Edge WebView2 runtime, which is already present on
current Windows 10 and Windows 11 installations.
