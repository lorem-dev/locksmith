# Locksmith Plugins

Locksmith ships with built-in vault plugins (`gopass`, `keychain`) and
`locksmith-pinentry` embedded inside the `locksmith` binary as a per-platform
zip. One `go install` is enough - no separate plugin install step.

## Lockstep versioning

A plugin's version is the version of the `locksmith` binary that bundled it.
There is no separate plugin release cycle, no network resolution, no version
drift for old binaries. To update plugins, update `locksmith` and run
`locksmith plugins update`.

## Where things land

`locksmith init` extracts only the plugins matching the vault types you
selected in the wizard, plus pinentry, to:

- `~/.config/locksmith/plugins/locksmith-plugin-<type>` - vault plugins.
- `~/.config/locksmith/bin/locksmith-pinentry` - pinentry.

`gpg-agent.conf` records the absolute pinentry path, which stays valid
across `brew upgrade` and re-installs.

## Conflict policy

If a target file already exists:

- sha256 matches the manifest -> silent skip.
- sha256 differs -> prompt: `Overwrite | Keep | Overwrite all | Keep all`.
- "Keep" prints a warning that functionality may not work as expected.
- `--auto` mode defaults to Keep.

## Updating

```bash
locksmith plugins update            # interactive prompts on conflict
locksmith plugins update --dry-run  # print what would change
locksmith plugins update --force    # overwrite without prompting
```

`plugins update` reads `config.yaml` to determine which plugins are needed.
Pinentry is always updated. Orphan plugin binaries are not removed.

## Custom (third-party) plugins

Custom plugins are not bundled. To install one:

1. Build or download `locksmith-plugin-<type>`.
2. Drop it into `~/.config/locksmith/plugins/`.
3. Reference the new vault type in `config.yaml`.

`vault health` runs the existing `CompatValidator` and surfaces platform or
version mismatches as `compat_warnings`. You are responsible for choosing a
version compatible with your `locksmith`. See
[`docs/plugins/authoring.md`](docs/plugins/authoring.md) and
[`docs/plugins/compatibility.md`](docs/plugins/compatibility.md).

## More

- [docs/plugins/architecture.md](docs/plugins/architecture.md) - full bundle
  pipeline and extraction flow.
- [docs/plugins/authoring.md](docs/plugins/authoring.md) - SDK quickstart.
- [docs/plugins/compatibility.md](docs/plugins/compatibility.md) - version
  protocol.
- [plugins/gopass/README.md](plugins/gopass/README.md) - built-in gopass
  plugin guide.
- [plugins/keychain/README.md](plugins/keychain/README.md) - built-in
  keychain plugin guide.
