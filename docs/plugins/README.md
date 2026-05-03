# Plugins

Locksmith vault providers are standalone Go binaries that implement a small
gRPC interface (`VaultProviderService`). Two kinds of plugins exist:

- **Built-in** - shipped inside the `locksmith` binary as a per-platform zip.
  Currently `gopass` (Linux + macOS) and `keychain` (macOS only). Their
  version is locked to the `locksmith` version that built them; there is
  no separate plugin release cycle, no network resolution, no version drift.
  See [`architecture.md`](architecture.md).
- **Custom** - third-party plugins. Drop a `locksmith-plugin-<type>` binary
  into `~/.config/locksmith/plugins/` and the discovery logic picks it up.
  Compatibility (platform + version range) is reported via `vault health`.
  See [`authoring.md`](authoring.md) and [`compatibility.md`](compatibility.md).

## In this directory

- [architecture.md](architecture.md) - how built-in plugins are bundled,
  extracted, and updated.
- [authoring.md](authoring.md) - SDK quickstart and discovery rules.
- [compatibility.md](compatibility.md) - `Info()`, version range, platform,
  and `compat_warnings` in `vault health`.

## Built-in plugin reference

- [`plugins/gopass/README.md`](../../plugins/gopass/README.md)
- [`plugins/keychain/README.md`](../../plugins/keychain/README.md)
