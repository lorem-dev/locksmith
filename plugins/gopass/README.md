# locksmith-plugin-gopass

Locksmith vault plugin that retrieves secrets from a [gopass](https://github.com/gopasspw/gopass)
password store. Authorization is delegated to `gpg-agent` (passphrase prompt or
Touch ID / smartcard when configured).

## See also

- [Configuration Reference - gopass](../../docs/configuration.md#gopass) - canonical YAML schema
- [Plugins](../../docs/plugins/README.md) - SDK and plugin authoring
- [GPG Passphrase and Pinentry](../../docs/pinentry.md) - background-daemon prompts

## Requirements

- `gopass` available on `$PATH` (`gopass ls` must succeed against your store).
- A working GPG setup with at least one secret key able to decrypt the store.
- Linux or macOS.

## Installation

The plugin is normally installed automatically by `locksmith init` from
the embedded bundle into `~/.config/locksmith/plugins/`. The build commands
below are for development only.

Build everything from the repository root:

```bash
make build-all
```

This produces `locksmith-plugin-gopass` next to the `locksmith` binary.

To build only this plugin:

```bash
cd plugins/gopass
go build -o locksmith-plugin-gopass .
```

Place the binary in one of:

1. The same directory as the `locksmith` binary.
2. `~/.config/locksmith/plugins/`.
3. Any directory in `$PATH`.

Locksmith discovers plugins automatically by name (`locksmith-plugin-<type>`).

### Custom builds

If you fork or modify this plugin, build it manually as above and place the
binary into `~/.config/locksmith/plugins/`. The discovery logic picks it
up; `vault health` will report any version mismatch via `compat_warnings`.

## Configuration

Minimal configuration in `~/.config/locksmith/config.yaml`:

```yaml
vaults:
  secrets:
    type: gopass

keys:
  notion-token:
    vault: secrets
    path: personal/notion
```

See [`docs/configuration.md`](../../docs/configuration.md#gopass) for the full
field reference.

## Examples

### 1. Single root store

```yaml
vaults:
  secrets:
    type: gopass

keys:
  github-token:
    vault: secrets
    path: dev/github
```

```bash
locksmith get --key github-token
```

### 2. Two named mounts (personal + work)

```yaml
vaults:
  personal:
    type: gopass
  work:
    type: gopass
    store: work          # gopass mount name

keys:
  anthropic-key:
    vault: personal
    path: ai/anthropic
  github-token:
    vault: work
    path: dev/github
```

### 3. Direct access without alias

Skip the `keys:` block and pass `--vault` and `--path` explicitly:

```bash
locksmith get --vault work --path dev/github
```

## Troubleshooting

**`gopass not found in PATH`**
Install gopass and ensure it is on your `$PATH`. `locksmith vault health` is the
fastest way to see this error.

**`gopass at /usr/bin/gopass is not initialized`**
The binary is found but `gopass ls --flat` fails. Run `gopass init` and confirm
the store is set up before retrying.

**`Inappropriate ioctl for device` / GPG passphrase prompt fails**
Happens when the daemon runs in the background with no TTY. Configure
`locksmith-pinentry` per [`docs/pinentry.md`](../../docs/pinentry.md).

**Secret comes back with a trailing newline**
The plugin strips a single trailing newline from `gopass show -o`. If you still
see one, your secret has multiple trailing newlines in storage.

## Source

- [`provider.go`](provider.go) - `GopassProvider`, `GetSecret`, `HealthCheck`, `Info`
- [`main.go`](main.go) - `sdk.Serve` entry point
