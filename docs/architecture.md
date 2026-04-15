# Locksmith Architecture

## Overview

Locksmith uses a plugin architecture: vault providers run as isolated processes
communicating with the central daemon over gRPC, using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

```
locksmith CLI  ──(gRPC/Unix socket)──▶  locksmith daemon
                                              │
                                    ┌─────────┼─────────┐
                               gRPC ▼    gRPC ▼    gRPC ▼
                           keychain   gopass   1password
                           plugin     plugin   plugin
```

## Components

### Daemon

- Listens on a Unix socket (`~/.config/locksmith/locksmith.sock`)
- Exposes `LocksmithService` gRPC service to CLI clients
- Manages session lifecycle and secret caching
- Launches vault plugins on startup; kills them on shutdown

### Session Manager (`internal/session`)

- Sessions identified by `ls_<32-byte-hex>` tokens
- TTL-based expiry (default 3h, configurable)
- Per-session secret cache: secrets are fetched from vault once per session,
  then served from memory cache for subsequent calls
- On session invalidation: explicit byte-zeroing of cached secrets (`memclr`)

### Plugin Manager (`internal/plugin`)

- Discovers `locksmith-plugin-*` binaries in standard search paths
- Launches each required plugin as a child process via hashicorp/go-plugin
- Plugin processes communicate over gRPC; isolated from daemon memory space

### Vault Plugins

Each plugin is a standalone binary implementing the `VaultProviderService` gRPC service:

- `GetSecret` - fetches a secret; triggers vault authorization (Touch ID, passphrase)
- `HealthCheck` - verifies the vault is installed and accessible
- `Info` - returns plugin name, version, and supported platforms

### CLI

Thin gRPC client to the daemon. Reads `LOCKSMITH_SESSION` from environment.
Returns an error with a helpful hint if the daemon is not running.

## Session Delegation

Sub-agents inherit the parent session by receiving `LOCKSMITH_SESSION` as an
environment variable. The daemon validates the session token on every request.

## Security Properties

- Secrets live only in daemon process memory, never on disk
- Unix socket has `0600` permissions (owner-only)
- Plugin processes are isolated; a compromised plugin cannot access other vaults
- `locksmith get` without a valid session returns an error, not a leaked secret
