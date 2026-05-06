# Locksmith Architecture

## Overview

Locksmith uses a plugin architecture: vault providers run as isolated processes
communicating with the central daemon over gRPC, using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

```
locksmith CLI  ──(gRPC/Unix socket)──▶  locksmith daemon
                                              │
                                    ┌─────────┴─────────┐
                               gRPC ▼              gRPC ▼
                           keychain              gopass
                           plugin                plugin
```

## Components

### Daemon

- Listens on a Unix socket (`~/.config/locksmith/locksmith.sock`)
- Exposes `LocksmithService` gRPC service to CLI clients
- Manages session lifecycle and secret caching
- Launches vault plugins on startup; kills them on shutdown

### Session Manager (`internal/session`)

- Sessions identified by `ls_` followed by 64 hex chars (32 random bytes)
- TTL-based expiry (default 3h, configurable)
- Per-session secret cache: secrets are fetched from vault once per session,
  then served from memory cache for subsequent calls
- On session invalidation: explicit byte-zeroing of cached secrets (`memclr`)

### Config Hot-Reload

The daemon supports three reload triggers - all invoke the same `Daemon.Reload()` path:

1. **SIGHUP** - `kill -HUP <daemon-pid>`
2. **`locksmith reload` CLI** - connects to the Unix socket and calls the `ReloadConfig` gRPC method
3. **File watcher** - `fsnotify` watches `~/.config/locksmith/` and fires after 1 second of quiet following a change to `config.yaml`

`Daemon.Reload()` flow:
1. Acquire `reloadMu` (serializes concurrent triggers)
2. Parse and validate the new config; abort with the old config on any error
3. Delta-sync plugin processes: launch new vault types, kill removed ones
4. Atomically replace the active config via `atomic.Pointer[config.Config]`

Each gRPC handler calls `cfgFn()` once at the start to get a consistent snapshot for
its entire execution. In-flight requests see either the old or the new config entirely -
never a mix.

### Plugin Manager (`internal/plugin`)

- Discovers `locksmith-plugin-*` binaries in standard search paths
- Launches each required plugin as a child process via hashicorp/go-plugin
- Plugin processes communicate over gRPC; isolated from daemon memory space

After a plugin is dispensed, the daemon calls `Info()` (5-second timeout) and runs
`CompatValidator` against the response. Warnings (platform mismatch, version range
violation, unreachable `Info()`, missing or invalid version metadata) are stored
alongside the running plugin and surfaced via `locksmith vault health`. No plugin is
blocked from running based on these checks.

### Vault Plugins

Each plugin is a standalone binary implementing the `VaultProviderService` gRPC service:

- `GetSecret` - fetches a secret; triggers vault authorization (Touch ID, passphrase)
- `HealthCheck` - verifies the vault is installed and accessible
- `Info` - returns plugin name, version, and supported platforms

### Bundled plugins and pinentry

Default vault plugins and `locksmith-pinentry` ship embedded in the
`locksmith` binary as a per-platform zip and are extracted at `locksmith
init` to `~/.config/locksmith/`. Plugin version is locked to the host
`locksmith` version. The full pipeline is documented in
[`docs/plugins/architecture.md`](plugins/architecture.md).

### CLI

Thin gRPC client to the daemon. If `LOCKSMITH_SESSION` is set in the environment,
`locksmith get` uses that session. If unset, it auto-starts a session using the
default TTL from config and prints the session ID to stderr so the caller can
optionally export it for reuse. Returns an error with a helpful hint if the daemon
is not running.

## Session Delegation

Sub-agents inherit the parent session by receiving `LOCKSMITH_SESSION` as an
environment variable. The daemon validates the session token on every request.

## Security Properties

- Secrets live only in daemon process memory, never on disk
- Unix socket has `0600` permissions (owner-only)
- Plugin processes are isolated; a compromised plugin cannot access other vaults
- `locksmith get` without a valid session returns an error, not a leaked secret
- Session IDs are masked in daemon log output unless `logging.level: debug`
  is active (see [Debug Logging Security Notice](security/debug-logging.md))

## Agent Integration

Agents interact with the daemon exclusively through the CLI. Session management
follows the protocol described in [Agent Integration](agent-integration.md):
the `locksmith session ensure` command reuses an existing valid session from
`LOCKSMITH_SESSION` or starts a new one. Platform hook templates are embedded
in the binary and installed by `locksmith autostart install`; the root
[`AGENTS.md`](../AGENTS.md) and [Agent Integration](agent-integration.md)
describe the protocol for platforms without hook support.
