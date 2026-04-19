# Locksmith Configuration Reference

## Configuration file

Default location: `~/.config/locksmith/config.yaml`

Override with `--config <path>` or the `LOCKSMITH_CONFIG` environment variable.

## Top-level structure

```yaml
defaults:
  session_ttl: 3h           # default session TTL (e.g. 1h, 30m)
  socket_path: ~/.config/locksmith/locksmith.sock

logging:
  level: info               # debug | info | warn | error
  format: text              # text | json
  file: ~/.config/locksmith/logs/daemon.log  # optional log file

vaults:
  <name>:
    type: <plugin-type>
    # ... plugin-specific fields

keys:
  <alias>:
    vault: <vault-name>
    path: <secret-path>
```

## Logging configuration

### logging.level

**Optional.** Log level for daemon output. Default: `info`.

- `debug` - verbose output; session IDs are logged in plaintext (see security note below)
- `info` - standard operational messages
- `warn` - warnings and errors only
- `error` - errors only

### logging.format

**Optional.** Output format. Default: `text`.

- `text` - human-readable plaintext logs
- `json` - structured JSON for log aggregators

### logging.file

**Optional.** Path to the log file. If set, all log output is written to this
file instead of stdout. Supports `~` expansion. The parent directory is created
automatically with mode `0700` if it does not exist.

The file is rotated when it reaches 50 MB and files older than 3 days are
deleted automatically.

Recommended when running as a background daemon via the shell hook.

> **Security note:** If `logging.level` is `debug`, session IDs are written to
> the log in plaintext. See [Debug Logging Security Notice](security/debug-logging.md).

## Direct access (without alias)

```bash
locksmith get --vault keychain --path my-account
locksmith get --vault my-gopass --path dev/key
```

---

## Vault plugins

### keychain (macOS only)

Retrieves secrets from the macOS Keychain using the Security framework.
Authorization (Touch ID or password) is triggered by the OS on each access.

**Configuration:**

```yaml
vaults:
  keychain:
    type: keychain
    service: com.example.myapp  # optional: default Keychain service name
```

**Key path format:**

```yaml
keys:
  # Plain account - uses vault-level service (or "locksmith" if unset)
  notion-token:
    vault: keychain
    path: notion

  # service/account - overrides vault-level service for this key only
  github-token:
    vault: keychain
    path: github/mytoken

  # No service configured - falls back to "locksmith" for backward compatibility
  legacy-key:
    vault: keychain
    path: my-old-account
```

**Service resolution order:** path prefix `service/account` > vault `service:` > `"locksmith"` (backward-compatible default).

**Full example:**

```yaml
vaults:
  work:
    type: keychain
    service: com.acme.work    # default service for all keys in this vault

keys:
  slack:
    vault: work
    path: slack               # service="com.acme.work", account="slack"
  github:
    vault: work
    path: github/token        # service="github", account="token" (overrides vault-level)
  legacy:
    vault: work
    path: legacy-tool         # service="locksmith" if vault has no service: set
```

**Notes:**
- Only available on macOS (darwin/amd64 and darwin/arm64).
- Passwords are stored and retrieved using `SecItemCopyMatching` via CGo.
- Error messages come directly from `SecCopyErrorMessageString` for readability.
- A path with more than one `/` is rejected at startup (e.g. `a/b/c` is invalid;
  use `a/b` for service=`a`, account=`b`).

---

### gopass

Retrieves secrets from a [gopass](https://github.com/gopasspw/gopass) password store.

**Configuration:**

```yaml
vaults:
  secrets:
    type: gopass
    store: work              # optional: gopass mount name (default: root store)

keys:
  notion-token:
    vault: secrets
    path: personal/notion    # gopass path within the store
```

**Full example:**

```yaml
vaults:
  personal:
    type: gopass             # uses root store
  work:
    type: gopass
    store: work              # uses "work" gopass mount

keys:
  github-token:
    vault: work
    path: dev/github-api
  anthropic-key:
    vault: personal
    path: ai/anthropic
```

**Notes:**
- Requires `gopass` installed and configured (`gopass ls` must succeed).
- `store:` is passed as the gopass mount name; omit to use the default root store.
- GPG passphrase prompts in background daemons require `locksmith-pinentry` -
  see "GPG passphrase and background daemons" below.

---

## Vault types

| type | Description |
|------|-------------|
| `keychain` | macOS Keychain (CGo, Touch ID) |
| `gopass` | gopass password manager (shells out to `gopass` CLI) |

---

## Re-running locksmith init

Running `locksmith init` on a machine that already has a config file at the chosen
path will detect the file and validate it. You will be offered three options:

- **Continue with existing config** - skip rewriting the config file; proceed with
  agent and sandbox setup only.
- **Overwrite with new config** - run the full wizard and replace the file.
- **Exit setup** - cancel without any changes.

In `--auto` mode the choice is made automatically: a valid config is kept as-is;
an invalid config is silently replaced.

---

## GPG passphrase and background daemons

When the locksmith daemon runs as a background process (launched from `.zshrc`,
`.bashrc`, or a launchd plist), it has no TTY. This causes `pinentry-curses` to
fail with `Inappropriate ioctl for device` when gopass needs a GPG passphrase.

**Solution:** `locksmith-pinentry` is a custom pinentry binary that detects the
available UI and uses the best option automatically:

| Environment | Method |
|---|---|
| TTY available | reads directly from `/dev/tty` |
| macOS, no TTY | native `osascript` password dialog |
| Linux, display available | `zenity --password` or `kdialog --password` |
| No UI available | returns cancellation; locksmith returns `Unauthenticated` error |

**Setup:**

Run `make init` once after cloning to build and install `locksmith-pinentry`.
Then run `locksmith init` - if you select the gopass vault, you will be asked:

```
Configure locksmith-pinentry for GPG passphrase prompts?
Required for gopass vault when locksmith runs as a background daemon (no TTY).
[y/n]
```

If you choose **yes** and already have a `pinentry-program` line in
`~/.gnupg/gpg-agent.conf`, the existing line is **commented out** (not
deleted) and the new path is written below it. For example, if your existing
config had:

```
pinentry-program /opt/homebrew/bin/pinentry-mac
```

After `locksmith init` it becomes:

```
#pinentry-program /opt/homebrew/bin/pinentry-mac
pinentry-program /Users/you/go/bin/locksmith-pinentry
```

You can restore the original setting at any time by editing
`~/.gnupg/gpg-agent.conf` directly.

In `--auto` mode this step is skipped and `~/.gnupg/gpg-agent.conf` is never
modified.

**Limitations:**

- **Headless sandbox without display** (CI, agent sandbox without `$DISPLAY`):
  passphrase input is impossible. Options:
  - Pre-unlock the GPG key before starting the daemon (use a passphrase-free key,
    or run `gpg --card-status` to cache the passphrase via an existing GUI session).
  - Use a passphrase-free GPG setup for the secrets store.
  - Run the daemon in a user session with display access.

- **macOS without WindowServer access** (pure SSH, `sudo` context, headless launchd):
  `osascript` will fail silently. `locksmith-pinentry` requires the user to be
  logged into a GUI session or use `-X` forwarding for the SSH session.

- **Linux without display and without TTY**: both `zenity`/`kdialog` and TTY
  mode will be unavailable. Passphrase input fails cleanly with an
  `Unauthenticated` error and a hint pointing to this section.

- **gpg-agent passphrase caching**: if gpg-agent has already cached the
  passphrase (within its TTL), `locksmith-pinentry` is never invoked. This is
  the expected production path after the first unlock.

- **locksmith-pinentry must be on PATH**: installed via `make init`. If missing,
  gopass falls back to the system-configured pinentry (original behavior).

---

## locksmith config pinentry

Configures `locksmith-pinentry` as the pinentry program for `gpg-agent`, independently
of `locksmith init`. Use this if you:

- Ran `locksmith init --auto` (which skips the GPG pinentry step), or
- Want to reconfigure after changing your GPG setup.

```bash
locksmith config pinentry [--auto] [--no-tui]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--auto` | Configure without prompting (equivalent to answering "yes") |
| `--no-tui` | Use plain-text prompts instead of the TUI |

**Requires** `locksmith-pinentry` to be installed - run `make init` once after cloning.

The command comments out any existing `pinentry-program` line in
`~/.gnupg/gpg-agent.conf`, writes the new path, and restarts `gpg-agent`.
See "GPG passphrase and background daemons" above for full context.

---

## Shell autostart

To start the locksmith daemon automatically when you open a terminal, add a
shell hook. The `locksmith init` wizard offers to do this for you.

To add it manually, append the following to your shell config file:

**bash / zsh / ash** (`~/.bashrc`, `~/.zshrc`, or `~/.profile`):

```sh
# locksmith daemon autostart
if command -v locksmith >/dev/null 2>&1; then locksmith _autostart 2>/dev/null; fi
```

**fish** (`~/.config/fish/config.fish`):

```fish
# locksmith daemon autostart
if command -v locksmith >/dev/null 2>&1; locksmith _autostart 2>/dev/null; end
```

The hook is idempotent: if the daemon is already running, `_autostart` exits
immediately without spawning a second process.
