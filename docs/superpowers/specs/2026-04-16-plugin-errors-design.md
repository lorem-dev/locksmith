# Plugin Errors, Keychain Configuration, and GPG Pinentry - Design Spec

Date: 2026-04-16

## Overview

Three interconnected bugs are fixed in this spec:

1. Plugin errors are double-wrapped as ugly `rpc error: code = Unknown desc = ...` messages
2. Keychain plugin uses a hardcoded service name and returns unreadable OSStatus codes
3. gopass fails with `Inappropriate ioctl for device` when the daemon runs as a background process without a TTY

---

## Section 1: Error architecture - typed gRPC status codes

### Problem

Errors from vault plugins travel through two gRPC boundaries:

```
Plugin process  ->  daemon (go-plugin gRPC)  ->  CLI (locksmith gRPC)
```

Currently, `VaultGRPCServer` passes errors through without a status code (defaults to `Unknown`). `VaultGRPCClient` receives `status.Error(Unknown, "rpc error: code = Unknown desc = ...")`, wraps it again, and eventually the CLI displays the full double-wrapped string.

### Solution

Introduce `VaultError` in the SDK. It implements `GRPCStatus()` so that when plugins return it, the gRPC server automatically uses the correct status code - no manual re-wrapping needed.

```go
// sdk/errors.go
type VaultError struct {
    Code    codes.Code
    Message string
}

func (e *VaultError) Error() string             { return e.Message }
func (e *VaultError) GRPCStatus() *status.Status { return status.New(e.Code, e.Message) }

// Constructors
func NotFoundError(msg string) error         { return &VaultError{codes.NotFound, msg} }
func PermissionDeniedError(msg string) error { return &VaultError{codes.PermissionDenied, msg} }
func UnavailableError(msg string) error      { return &VaultError{codes.Unavailable, msg} }
func UnauthenticatedError(msg string) error  { return &VaultError{codes.Unauthenticated, msg} }
func InternalError(msg string) error         { return &VaultError{codes.Internal, msg} }
```

Error flow after the fix:

```
Plugin            -> status.Error(codes.NotFound, "keychain: item not found")
VaultGRPCClient   -> &VaultError{Code: NotFound, Message: "keychain: item not found"}
server.go         -> status.Errorf(NotFound, "fetching secret: keychain: item not found")
CLI               -> "Error: keychain: item not found"
```

### Changes

- `sdk/errors.go` - new file with `VaultError` and helper constructors
- `sdk/plugin.go` - `VaultGRPCClient.GetSecret` unwraps gRPC status into `*VaultError` instead of returning raw error
- `internal/daemon/server.go` - replace `fmt.Errorf("fetching secret from vault: %w", err)` with `status.Errorf(code, "fetching secret: %s", msg)`, preserving the code from the incoming error

---

## Section 2: Keychain configuration and error messages

### Problem

- `service` is hardcoded to `"locksmith"` in `provider_darwin.go`
- OSStatus code -25300 is returned as a raw integer, not as a human-readable message
- There is no way to configure a different service per vault or per key

### Solution

**Path format.** The `path` field supports an optional `service/account` shorthand:

- `"notion"` - account only, service comes from vault config or defaults to `"locksmith"`
- `"apple/notion"` - service = `"apple"`, account = `"notion"`, overrides vault config. Only one slash is supported; a path with two or more slashes is rejected with a config validation error at startup.

**Vault config.** The `Vault` struct gains a `service` field:

```yaml
vaults:
  keychain:
    type: keychain
    service: com.example.myapp  # optional default service for this vault

keys:
  notion:
    vault: keychain
    path: notion                # uses vault-level service
  gh-token:
    vault: keychain
    path: github/token          # overrides: service="github", account="token"
```

**Resolution order for service:** path prefix > vault `service:` > `"locksmith"` (backward-compatible fallback).

**OSStatus messages.** On Darwin, `SecCopyErrorMessageString(status, nil)` returns a human-readable CFString. This is called via CGo in `provider_darwin.go`. OSStatus codes are mapped to gRPC codes:

| OSStatus | Meaning              | gRPC code          |
|----------|----------------------|--------------------|
| -25300   | errSecItemNotFound   | NotFound           |
| -25293   | errSecAuthFailed     | PermissionDenied   |
| -25308   | errSecInteractionNotAllowed | PermissionDenied |
| others   | unexpected           | Internal           |

### Changes

- `internal/config/config.go` - add `Service string` to `Vault` struct
- `plugins/keychain/provider_darwin.go` - parse `service/account` path, resolve service, call `SecCopyErrorMessageString` for human-readable errors, return typed `sdk.VaultError`

---

## Section 3: gopass and GPG pinentry in background daemon

### Problem

The daemon is typically launched from `.zshrc`/`.bashrc` as a background process with no TTY. When gopass needs a GPG passphrase, it asks gpg-agent, which launches pinentry. `pinentry-curses` fails with `Inappropriate ioctl for device` because there is no TTY to open.

### Solution

Locksmith provides a `locksmith-pinentry` binary that implements the Assuan protocol (the same protocol all pinentry programs use). When gopass is invoked, the provider sets `PINENTRY_PROGRAM=locksmith-pinentry` in the subprocess environment, directing gpg-agent to use it.

`locksmith-pinentry` detects the available UI at runtime and uses the best option:

1. **TTY available** - prompts directly in the terminal (wraps pinentry-curses behavior)
2. **macOS, no TTY** - uses `osascript` to show a native password dialog
3. **Linux with display** - uses `zenity` if available, otherwise `kdialog`
4. **No UI available** - returns a Assuan `ERR 83886179 Operation cancelled` and exits, causing gopass to fail; the gopass provider maps this to `sdk.UnauthenticatedError("GPG passphrase required but no UI available")`

`locksmith-pinentry` is built and installed to `$GOBIN` as part of `make init`.

### Env passthrough

The gopass provider explicitly passes the following env vars to the gopass subprocess:

- `HOME`, `PATH`, `GNUPGHOME`
- `DISPLAY`, `WAYLAND_DISPLAY` (Linux GUI)
- `GPG_TTY` set to the current TTY (`tty` syscall) if available, otherwise unset

### Limitations (documented in docs/configuration.md)

- **Headless sandbox without display** (CI, agent sandbox): passphrase input is impossible. Pre-unlock the GPG key before starting the daemon (`gpg --card-status` or `echo "" | gpg --passphrase "" --batch -d`), or configure a passphrase-free key.
- **macOS without WindowServer access** (SSH without display forwarding, launchd agent): `osascript` will fail. `locksmith-pinentry` requires either an `-X` forwarded SSH session or the user to be logged into a GUI session.
- **Linux without display and without TTY**: only `zenity`/`kdialog` or TTY mode work; both unavailable means passphrase input fails cleanly with `UnauthenticatedError`.
- **If gpg-agent has already cached the passphrase**: `locksmith-pinentry` is never invoked. This is the expected production path after first unlock.
- **`locksmith-pinentry` must be on PATH**: installed via `make init`. If missing, gopass falls back to the system-configured pinentry (original behavior).

### Changes

- `cmd/locksmith-pinentry/` - new binary implementing Assuan pinentry protocol
- `plugins/gopass/provider.go` - set `PINENTRY_PROGRAM`, explicit env passthrough, map `Inappropriate ioctl` stderr to `UnauthenticatedError` with hint
- `Makefile` - add `locksmith-pinentry` to `make init` build targets
- `docs/configuration.md` - new section "GPG passphrase and background daemons" with full limitations

---

## Section 4: Error display in CLI

### Problem

The CLI currently prints the full gRPC error string, including the `rpc error: code = X desc =` prefix.

### Solution

`internal/cli/get.go` calls `status.FromError(err)` and displays only the `desc` part:

```
Error: keychain: item not found
Hint: check that the key path and service name are correct
```

Hints are shown on a separate line in gray (when the terminal supports color) based on the gRPC status code:

| Code              | Hint                                                                 |
|-------------------|----------------------------------------------------------------------|
| NotFound          | check that the key path and service name are correct                 |
| PermissionDenied  | access denied - check vault permissions                              |
| Unauthenticated   | GPG passphrase required but no UI available - see docs/configuration.md#gpg-pinentry |
| Unavailable       | vault plugin failed to start - re-run with --log-level debug         |
| others            | (no hint, message only)                                              |

If the error is not a gRPC status error (unexpected internal error), the full error chain is printed unchanged.

### Changes

- `internal/cli/get.go` - extract gRPC status, print `desc` only, append hint line where applicable
- `internal/cli/` - add `formatError(err error) string` helper shared across commands

---

## Affected modules

| Module                            | Changes                                      |
|-----------------------------------|----------------------------------------------|
| `github.com/lorem-dev/locksmith/sdk` | `errors.go` (new), `plugin.go` (unwrap) |
| `github.com/lorem-dev/locksmith`  | `config.go`, `server.go`, `cli/get.go`       |
| `github.com/lorem-dev/locksmith-plugin-keychain` | `provider_darwin.go`          |
| `github.com/lorem-dev/locksmith-plugin-gopass`   | `provider.go`                 |
| `cmd/locksmith-pinentry/`         | new binary                                   |

---

## Out of scope

- Supporting vaults other than keychain and gopass for this change
- UI for managing keychain entries (create/delete) via locksmith
- Caching passphrases inside locksmith itself
- Windows support
