# Locksmith — Design Specification

**Date:** 2026-04-13
**Status:** Approved

## Overview

Locksmith is a middleware layer for secure agent access to secret headers when interacting with MCP servers. Secrets are stored in vault providers (macOS Keychain, gopass, 1Password, GNOME Keyring), and authorization is delegated to the vault itself (Touch ID, GPG passphrase, etc.). Locksmith provides both a CLI and a background daemon with per-agent-session secret caching.

## Architecture

### Approach: Plugin Architecture

The daemon (locksmith) loads vault providers as separate plugin processes via [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) (gRPC). Each plugin is its own binary running in an isolated process.

```
┌─────────────────────────────────────────────────┐
│                  locksmith CLI                   │
│  locksmith serve | get | session start/end       │
└──────────────┬──────────────────────────────────┘
               │ Unix socket (gRPC)
┌──────────────▼──────────────────────────────────┐
│              locksmith daemon                    │
│  ┌────────────┐ ┌───────────┐ ┌───────────────┐ │
│  │  Session    │ │  Config   │ │  Plugin       │ │
│  │  Manager    │ │  Loader   │ │  Manager      │ │
│  └────────────┘ └───────────┘ └───┬───────────┘ │
└───────────────────────────────────┼─────────────┘
                                    │ gRPC (per-plugin process)
               ┌────────────────────┼────────────────────┐
               ▼                    ▼                    ▼
      ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
      │  keychain     │    │  gopass       │    │  1password   │
      │  plugin       │    │  plugin       │    │  plugin      │
      │  (CGo)        │    │  (pure Go)    │    │  (future)    │
      └──────────────┘    └──────────────┘    └──────────────┘
```

### Components

- **CLI** — entry point (`cobra`), thin client to the daemon via Unix socket (gRPC — same protocol as plugins, but a separate `LocksmithService` with methods `GetSecret`, `SessionStart`, `SessionEnd`, `SessionList`, `VaultList`, `VaultHealth`)
- **Daemon** — manages sessions, loads config, orchestrates plugins. Listens on a Unix socket, exposes `LocksmithService` for CLI clients. Must work correctly under WSL (Windows Subsystem for Linux) — Unix sockets are supported in WSL2
- **Session Manager** — creates/validates session tokens, TTL 3h (configurable), caches secrets per-session
- **Plugin Manager** — launches vault plugins as separate processes, communicates via gRPC
- **Vault Plugins** — standalone binaries, each implementing a single gRPC interface

## Plugin Protocol (gRPC)

Each vault plugin implements a single gRPC service:

```protobuf
service VaultProvider {
  // Retrieve a secret. The plugin handles authorization itself
  // (Touch ID, GPG passphrase, etc.)
  rpc GetSecret(GetSecretRequest) returns (GetSecretResponse);

  // Check vault availability (is CLI installed, is keychain accessible)
  rpc HealthCheck(Empty) returns (HealthCheckResponse);

  // Plugin metadata
  rpc Info(Empty) returns (PluginInfo);
}

message GetSecretRequest {
  string path = 1;              // key/path in vault
  map<string, string> opts = 2; // extra params (store, account, etc.)
}

message GetSecretResponse {
  bytes secret = 1;             // secret value
  string content_type = 2;      // "text/plain", "application/json", etc.
}

message HealthCheckResponse {
  bool available = 1;
  string message = 2;           // "gopass not found in PATH", etc.
}

message PluginInfo {
  string name = 1;              // "keychain", "gopass"
  string version = 2;
  repeated string platforms = 3; // ["darwin"], ["linux", "darwin"]
}
```

### Plugin Lifecycle

1. Daemon reads config, determines which vault types are in use
2. For each type, launches the corresponding binary (`locksmith-plugin-keychain`, `locksmith-plugin-gopass`)
3. hashicorp/go-plugin establishes a gRPC connection
4. On `GetSecret` — the plugin initiates authorization (Touch ID prompt, GPG passphrase)
5. Daemon caches the result in the session store

### Plugin Discovery

The daemon searches for `locksmith-plugin-*` binaries in:
1. Directory alongside the `locksmith` binary
2. `~/.config/locksmith/plugins/`
3. `$PATH`

## Session Management

### Creating a Session

```
locksmith session start [--ttl 3h] [--keys k1,k2]
→ { "session_id": "ls_abc123...", "expires_at": "2026-04-13T22:40:00Z" }
```

- Session ID — cryptographically random token (`crypto/rand`, 32 bytes, hex-encoded, `ls_` prefix)
- Passed to sub-agents via `LOCKSMITH_SESSION=ls_abc123...`
- Optional `--keys` — restricts the session to specified keys only (least privilege principle)

### Retrieving a Secret

```
locksmith get --key github-token                         # by alias
locksmith get --vault keychain --path github-api-token   # direct access (fallback)
```

CLI reads `LOCKSMITH_SESSION` from env, sends the request to the daemon. If the daemon is not running, CLI returns an error with a hint to run `locksmith serve`.

### Authorization Flow

1. First request for a key in a session → daemon calls `GetSecret` on the plugin → plugin prompts Touch ID / password → secret is cached in daemon memory, bound to the session ID
2. Repeated request for the same key in the same session → daemon returns from cache, no re-authorization
3. Request for multiple keys (`--keys key1,key2`) → daemon warns: "Session requests access to N secrets: key1, key2. Confirm." → vault authorization for each

### Session Termination

- `locksmith session end` — explicit invalidation, secrets wiped from memory. Reads session ID from `LOCKSMITH_SESSION` env, or accepts `--session <id>` flag
- TTL expired — automatic invalidation
- Daemon stopped — all sessions destroyed

### Cache Security

- Secrets are stored only in daemon process memory, never written to disk
- On invalidation — explicit byte zeroing (`memclr`)

## Monorepo Structure

Go workspace with separate modules for the daemon, SDK, and each plugin:

```
locksmith/
├── go.work                     # Go workspace
├── go.work.sum
├── Makefile                    # build, lint, test
├── .golangci.yml               # golangci-lint config
├── cmd/
│   └── locksmith/              # main binary (CLI + daemon)
│       └── main.go
├── internal/                   # daemon internal packages
│   ├── daemon/                 # Unix socket server, request routing
│   ├── session/                # Session Manager
│   ├── config/                 # YAML config parser
│   ├── plugin/                 # Plugin Manager (hashicorp/go-plugin)
│   └── cli/                    # CLI commands (cobra)
├── proto/
│   └── vault/
│       └── v1/
│           └── vault.proto     # plugin gRPC interface
├── sdk/                        # Go module for plugin authors
│   ├── go.mod                  # module github.com/lorem/locksmith-sdk
│   └── plugin.go               # helpers: Serve(), base gRPC scaffolding
├── plugins/                    # default plugins, each its own Go module
│   ├── keychain/
│   │   ├── go.mod              # module github.com/lorem/locksmith-plugin-keychain
│   │   ├── main.go
│   │   └── provider.go         # CGo, Security framework
│   └── gopass/
│       ├── go.mod              # module github.com/lorem/locksmith-plugin-gopass
│       ├── main.go
│       └── provider.go         # exec gopass CLI
├── docs/
│   └── plans/
└── CLAUDE.md
```

### Go Workspace (`go.work`)

```go
go 1.23

use (
    .
    ./sdk
    ./plugins/keychain
    ./plugins/gopass
)
```

### Makefile

```makefile
.PHONY: build build-all lint test clean

build:                          # main binary
	go build -o bin/locksmith ./cmd/locksmith

build-plugins:                  # all plugins
	go build -o bin/locksmith-plugin-keychain ./plugins/keychain
	go build -o bin/locksmith-plugin-gopass ./plugins/gopass

build-all: build build-plugins

lint:
	golangci-lint run ./...

test:
	go test ./...

proto:                          # generate gRPC code from proto
	protoc --go_out=. --go-grpc_out=. proto/vault/v1/vault.proto

clean:
	rm -rf bin/
```

## Configuration

### Config File (`~/.config/locksmith/config.yaml`)

```yaml
defaults:
  session_ttl: 3h
  socket_path: ~/.config/locksmith/locksmith.sock

vaults:
  keychain:
    type: keychain

  my-gopass:
    type: gopass
    store: personal          # optional

keys:
  github-token:
    vault: keychain
    path: "github-api-token"

  anthropic-key:
    vault: my-gopass
    path: "dev/anthropic"
```

### CLI Commands

```
locksmith init                              # interactive setup (TUI)
locksmith init --no-tui                     # plain-text fallback, no TUI forms
locksmith init --auto                       # auto-detect, defaults, minimal prompts
locksmith init --agent claude               # specific agent only, no scanning
locksmith init --skip-agents                # locksmith config only, no agents
locksmith init --update-agents              # reinstall agent instructions

locksmith serve                             # start daemon
locksmith serve --daemon                    # background mode

locksmith session start [--ttl 3h] [--keys k1,k2]
locksmith session end
locksmith session list

locksmith get --key github-token
locksmith get --vault keychain --path github-api-token

locksmith vault list                        # available vault providers
locksmith vault health                      # healthcheck all plugins

locksmith config check                      # validate config
```

### MCP Config Integration

```json
{
  "mcpServers": {
    "my-server": {
      "url": "https://api.example.com",
      "headers": {
        "Authorization": "Bearer $(locksmith get --key github-token)"
      }
    }
  }
}
```

The agent sees `$(locksmith get ...)`, executes the command, and receives the secret. `locksmith get` picks up `LOCKSMITH_SESSION` from env and contacts the daemon.

## `locksmith init` — Interactive Setup

### TUI Library

[charmbracelet/huh](https://github.com/charmbracelet/huh) — forms with arrow key navigation, checkboxes, selection highlighting. Same UX as Claude Code MCP setup.

Note: OpenHands uses Textual/Rich (Python-only), which is not applicable to a Go project. charmbracelet/huh is the Go ecosystem equivalent and the best fit.

#### Accessible / Non-Interactive Fallback

huh has a built-in accessible mode (`form.WithAccessible(true)`) that drops the full TUI and falls back to simple line-by-line text prompts. This activates automatically when `TERM=dumb` (CI, pipes, non-interactive terminals).

Locksmith will support:
- `locksmith init --no-tui` — force plain-text prompts, no TUI forms
- Automatic detection: if `TERM=dumb` or stdin is not a TTY, fall back to accessible mode
- All init functionality must work identically in both modes

#### WSL Compatibility

huh/bubbletea work correctly in WSL2 terminals. Minor known issue: slight startup delay on some WSL2 setups (bubbletea init). No rendering or input bugs. Recommended terminal: Windows Terminal for best ANSI support.

### Full Interactive Flow

```
$ locksmith init

Welcome to Locksmith!

── Config Location ──────────────────────────
Where to store config?
  ▸ ~/.config/locksmith (default)
    Custom path

── Vault Backends ───────────────────────────
Which vault backends do you use?
  ▸ ◉ macOS Keychain (detected)
    ◉ gopass (v1.15.0)
    ○ 1Password (not installed, coming soon)
    ○ GNOME Keyring (not available on macOS)

  ↑/↓ navigate • space select • enter confirm

Detected gopass stores: personal, work
Default store?
  ▸ personal
    work

── AI Agents ────────────────────────────────
Scanning for installed AI agents...
  ✓ Claude Code (v1.2.0)
  ✓ Codex (v0.9.1)
  ✗ OpenCode — not found

Install locksmith for detected agents?
  ▸ All detected (Claude Code, Codex)
    Select manually
    Skip agent setup

── Sandbox Permissions ──────────────────────
Allow locksmith commands in agent sandboxes?
  locksmith get, session start/end, vault list/health
  ▸ Yes
    No

── Summary ──────────────────────────────────
Config:       ~/.config/locksmith/config.yaml
Vaults:       keychain, gopass (store: personal)
Agents:       Claude Code, Codex
Sandbox:      allowed

  ▸ Apply
    Edit
    Cancel

✓ Config written
✓ Claude Code: CLAUDE.md updated, skill installed, sandbox configured
✓ Codex: AGENTS.md updated, policy configured
✓ Run `locksmith serve` to start
```

### Agent Auto-Detection

| Agent | Detection method |
|-------|-----------------|
| Claude Code | `~/.claude/` exists or `claude` in PATH |
| Codex | `~/.codex/` exists or `codex` in PATH |
| OpenCode | `~/.config/opencode/` exists or `opencode` in PATH |

Detection logic is extensible — each agent is described by a struct with paths and checks. Adding a new agent = adding a new entry.

### Generated Output per Agent

| Agent | Instructions | Location |
|-------|-------------|----------|
| Claude Code | CLAUDE.md section + skill | `~/.claude/CLAUDE.md`, `~/.claude/skills/locksmith.md` |
| Codex | AGENTS.md section | `~/.codex/AGENTS.md` |
| OpenCode | Config instructions | `~/.config/opencode/instructions.md` |
| Custom | Markdown file for manual integration | `~/.config/locksmith/agent-instructions.md` |

### Example Claude Code Skill

```markdown
---
name: locksmith-auth
description: Use when MCP server requires authentication headers
---

When an MCP server config contains `$(locksmith get ...)` in headers:
1. Check if LOCKSMITH_SESSION is set in environment
2. If not — run `locksmith session start` and export the token
3. Use `locksmith get --key <name>` to retrieve secrets
4. Pass retrieved value as the header value
5. Delegate LOCKSMITH_SESSION to sub-agents via environment

Never hardcode secrets. Never cache secrets outside of locksmith.
```

## Sandbox Support

### Problem

In restricted mode (Claude Code sandbox, Codex restricted mode) the agent cannot freely call `locksmith get` — each invocation requires manual user approval, ruining UX.

### Solution: Allowlist

`locksmith init` generates rules that permit locksmith commands without confirmation.

**Claude Code — `settings.json`:**
```json
{
  "permissions": {
    "allow": [
      "Bash(locksmith get *)",
      "Bash(locksmith session start *)",
      "Bash(locksmith session end)",
      "Bash(locksmith vault list)",
      "Bash(locksmith vault health)"
    ]
  }
}
```

Note: `Bash(...)` refers to the Claude Code tool name, not the user's shell. The permission pattern is matched by Claude Code's permission engine before any shell is invoked, so it works identically regardless of the user's shell (bash, zsh, fish, etc.).

**Codex — policy:**
```yaml
allow:
  - "locksmith get *"
  - "locksmith session *"
  - "locksmith vault list"
```

### Security

- Authorization still happens through the vault provider (Touch ID / password) — once per session
- `locksmith get` without an active session returns an error, not a leak
- Secrets never touch disk

## Scope

### Supported Platforms

- macOS (primary — Keychain + Touch ID)
- Linux (gopass, future GNOME Keyring)
- WSL2 (must work correctly — Unix sockets, TUI forms, agent detection)

### v1 (MVP)

- Daemon with Unix socket
- Session management (start, end, list, TTL)
- Plugin protocol (gRPC, hashicorp/go-plugin)
- Plugins: **macOS Keychain** (CGo, Touch ID), **gopass** (exec CLI)
- CLI: serve, get, session, vault, config, init
- YAML configuration with key aliases + fallback to --vault/--path
- `locksmith init` with TUI (charmbracelet/huh), agent auto-detection
- `locksmith init --no-tui` plain-text fallback for non-interactive environments
- Agent integration: **Claude Code**, **Codex**
- Sandbox allowlist
- WSL2 compatibility
- golangci-lint, Makefile

### Future

- Plugins: **1Password** (op CLI), **GNOME Keyring** (secret-tool / D-Bus)
- Agent integration: **OpenCode**, other agents
- `locksmith status` — live TUI dashboard (charmbracelet/bubbletea)
- Secret rotation / expiration notifications

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/hashicorp/go-plugin` | Plugin system (gRPC) |
| `github.com/charmbracelet/huh` | TUI forms for init |
| `google.golang.org/grpc` | gRPC transport |
| `google.golang.org/protobuf` | Protobuf codegen |
| `gopkg.in/yaml.v3` | YAML config |
| `github.com/golangci/golangci-lint` | Linting (dev dependency) |
