# locksmith-plugin-keychain

Locksmith vault plugin that retrieves secrets from the macOS Keychain via the
Security framework. Authorization (Touch ID or login password) is triggered by
the OS on each access.

## See also

- [Configuration Reference - keychain](../../docs/configuration.md#keychain-macos-only) - canonical YAML schema
- [Plugins](../../docs/plugins/README.md) - SDK and plugin authoring

## Requirements

- macOS only (`darwin/amd64`, `darwin/arm64`). The plugin builds on Linux and
  Windows, but the resulting binary returns "platform not supported" at startup.
- Built with CGo (links the Security framework). `make build-all` handles this
  on macOS.

## Installation

The plugin is normally installed automatically by `locksmith init` from
the embedded bundle into `~/.config/locksmith/plugins/`. The build commands
below are for development only.

Build everything from the repository root on macOS:

```bash
make build-all
```

This produces `locksmith-plugin-keychain` next to the `locksmith` binary.

To build only this plugin:

```bash
cd plugins/keychain
go build -o locksmith-plugin-keychain .
```

Place the binary in one of:

1. The same directory as the `locksmith` binary.
2. `~/.config/locksmith/plugins/`.
3. Any directory in `$PATH`.

Locksmith discovers plugins automatically by name.

### Custom builds

If you fork or modify this plugin, build it manually as above and place the
binary into `~/.config/locksmith/plugins/`. The discovery logic picks it
up; `vault health` will report any version mismatch via `compat_warnings`.

## Configuration

Minimal configuration in `~/.config/locksmith/config.yaml`:

```yaml
vaults:
  keychain:
    type: keychain

keys:
  notion-token:
    vault: keychain
    path: notion
```

See [`docs/configuration.md`](../../docs/configuration.md#keychain-macos-only)
for the full field reference.

## Examples

### 1. Single account, default service

The vault has no `service:` set; the plugin falls back to `locksmith` as the
service name. The Keychain entry is `service=locksmith`, `account=notion`.

```yaml
vaults:
  keychain:
    type: keychain

keys:
  notion-token:
    vault: keychain
    path: notion
```

### 2. Vault-level service for all keys

Every key in the `work` vault uses `com.acme.work` as its Keychain service:

```yaml
vaults:
  work:
    type: keychain
    service: com.acme.work

keys:
  slack:
    vault: work
    path: slack            # service=com.acme.work, account=slack
  jira:
    vault: work
    path: jira             # service=com.acme.work, account=jira
```

### 3. Per-key service override via path prefix

A path of the form `service/account` overrides the vault-level `service:`:

```yaml
vaults:
  mixed:
    type: keychain
    service: com.acme.work

keys:
  github:
    vault: mixed
    path: github/token     # service=github, account=token
  slack:
    vault: mixed
    path: slack            # service=com.acme.work, account=slack
```

Resolution order: `service/account` in path > vault `service:` > `"locksmith"`.

## Touch ID and biometric access

By default, macOS Keychain items created via the Keychain Access GUI
(or `security add-generic-password` without explicit ACL flags) are
plain generic passwords. macOS shows a **password prompt** when an
app reads them - even on Macs with Touch ID enrolled - because the
item itself has no biometric access control.

Locksmith can re-store existing items, or write new ones, with the
`kSecAccessControlUserPresence` ACL plus
`kSecAttrAccessibleWhenUnlockedThisDeviceOnly`. Such items prompt
for Touch ID first, fall back to the device passcode if biometry is
unavailable, and are not synced via iCloud Keychain.

### Migrate existing items

After upgrading to v0.4.0+:

```sh
locksmith vault migrate --all
```

This reads each `keychain`-typed key in `config.yaml` (one last
password prompt per item), deletes the existing keychain entry, and
re-stores it with biometric ACL. Subsequent reads prompt for Touch
ID.

A single alias can be migrated with `locksmith vault migrate
<alias>`. `--dry-run` lists what would be migrated without writing.

### Add new items via locksmith

```sh
locksmith vault set github-token
echo -n "ghp_xxx" | locksmith vault set github-token --force
locksmith vault set github-token --from-file ~/secrets/gh.txt
```

`set` reads the secret value from `--from-file`, piped stdin, or an
interactive TTY prompt (with confirmation). New items get the
biometric ACL automatically; no Keychain Access GUI step needed.

### Caveats

- Keychain Access GUI does not preserve the biometric ACL when you
  edit an item. If you edit a migrated item in the GUI, the ACL is
  lost and the next locksmith read prompts for a password again.
  Re-run `locksmith vault migrate <alias>` to restore the ACL.
- `kSecAttrAccessibleWhenUnlockedThisDeviceOnly` means items do not
  sync to other Macs via iCloud Keychain. This is intentional for
  security; if you need cross-device secrets, use a vault designed
  for sync (e.g. 1Password, gopass with a synced store).

## Troubleshooting

**`keychain: The specified item could not be found in the keychain.` (errSecItemNotFound, -25300)**
The service/account combination does not exist. Open Keychain Access.app and
search for the entry, or create it via the CLI:

```bash
security add-generic-password -s <service> -a <account> -w '<secret>'
```

**`keychain: Authentication failed.` (errSecAuthFailed, -25293)**
Touch ID or login password was denied, or biometrics are not enrolled. Re-run
the command and complete the prompt.

**Path with more than one `/`**
Only one `/` is allowed in a key path. Use `service/account`, not
`service/sub/account` - the latter is rejected at startup.

**Plugin does not start on Linux or Windows**
The keychain plugin is macOS-only. Use a different vault type (e.g. gopass) on
non-macOS systems.

## Source

- [`provider_darwin.go`](provider_darwin.go) - macOS implementation via CGo and the Security framework
- [`provider_stub.go`](provider_stub.go) - stub for non-darwin builds
- [`main.go`](main.go) - `sdk.Serve` entry point
