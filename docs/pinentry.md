# GPG Passphrase and Pinentry

> See also: [Configuration Reference](configuration.md)

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
