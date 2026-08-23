# VibeDock

A fast, native desktop client for [Mistral Vibe](https://github.com/mistralai/mistral-vibe), built with Go, Wails v3, Svelte 5, and SQLite.

## What is implemented

- Project and conversation navigation with persistent SQLite metadata
- Separate Chat, Work, and Code workspaces: quick conversations, multi-step knowledge work, and local folder-backed coding projects
- Reusable Libraries with local documents and webpages, multiple per conversation, persisted in SQLite and delivered to Vibe as structured ACP resource links
- Native multi-file Library import with app-managed copies, plus direct handoff to Mistral's cloud Libraries and Work dashboard
- Native project-terminal launch from Code conversations
- Configurable one-click editor launch for Code projects (VSCodium, VS Code, Cursor, Zed, or Sublime Text)
- Streaming chat UI with reasoning, tool calls, permission prompts, and cancellation states
- Native project-folder picker
- Live model, thinking-level, and agent-mode selectors populated by `vibe-acp` and applied to the active Vibe session
- Changes panel, custom MCP server configuration, and a searchable Vibe connector inventory that separates Mistral account connection from local Vibe access
- Per-connector “Enabled in Vibe” controls that safely update Vibe's own configuration and reload idle ACP sessions
- An ACP process boundary for `vibe-acp`, with browser-mode demo data for frontend work
- Keyboard shortcuts: `⌘N` new conversation, `⌘K` project search, `⌘,` MCP and connectors, `⌘⇧P` command menu

## Development

Requirements: Go 1.25+, Node 20+, `vibe-acp`, and the Wails v3 CLI.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.6
wails3 task install:frontend
wails3 task dev
```

The frontend can also run by itself with `cd frontend && npm run dev`. It automatically uses a local demo adapter when the Wails runtime is not present.

## Test and build

```sh
wails3 task test
wails3 task build
# macOS application bundle
wails3 task package:darwin
```

The production executable is written to `bin/vibedock`; the macOS packaging task creates `bin/VibeDock.app`. Wails v3 and its JavaScript runtime are pinned in `go.mod` and `frontend/package.json`; `npm ci` uses the committed lockfile.

Native build scripts are also provided for supported development platforms:

```sh
./scripts/build-macos.sh
./scripts/build-ubuntu.sh --install-deps
./scripts/build-fedora.sh --install-deps
```

On Windows, run `.\scripts\build-windows.ps1` from PowerShell. The scripts check
the Go and Node versions, run the backend/frontend verification, and produce the
native platform artifact. See [scripts/README.md](scripts/README.md) for options,
system packages, signing, and output locations.

Application metadata is stored in the platform user config directory under `VibeDock/vibe.db`. Existing `Vibe Desktop` data is migrated automatically on first launch. Credentials are deliberately not stored in SQLite; ACP inherits the already-configured Vibe environment.
