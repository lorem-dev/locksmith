# Bundled Plugin Architecture

Default plugins (`gopass`, `keychain`) and `locksmith-pinentry` ship inside
the `locksmith` binary as a per-platform zip embedded with `//go:embed`. This
document covers the build pipeline, extraction flow, conflict policy, and
the lockstep-versioning principle.

## Why embed, not download

A plugin's version is the version of the `locksmith` that bundled it.
There is no separate plugin release cycle, no registry, no network at init,
and no permanent hosting of historical artefacts. An old `locksmith` binary
resolves to the exact plugin set it was built with - because that set is
inside it.

The cost is binary size: per-platform zip with deflate compression yields
~+15 MB for the default plugin set + pinentry. Acceptable for a single-
binary developer tool.

## Build pipeline (`make build-all`)

```
[ go build pinentry ]
        |
        v
[ go run ./.scripts/build-plugins ]   (gopass, keychain on darwin)
        |
        v
[ go run ./.scripts/build-bundle ]    -> internal/bundled/assets/bundle-<goos>.zip
        |
        v
[ go build cmd/locksmith ]            //go:embed picks up the fresh zip
```

The bundle is a zip containing `manifest.json` plus each binary. The
manifest records the bundled-locksmith version, platform, and per-entry
sha256.

`internal/bundled/assets/` is gitignored. `make init` ensures empty
placeholder zips exist so `go build ./cmd/locksmith` works on a fresh clone.

## Extraction at `locksmith init`

`locksmith init` extracts only the plugins that match the user's chosen
vault types in the wizard, plus pinentry, to:

- `~/.config/locksmith/plugins/locksmith-plugin-<type>` - vault plugins.
- `~/.config/locksmith/bin/locksmith-pinentry` - pinentry.

`gpg-agent.conf` records the absolute path of the extracted pinentry, which
is stable across `brew upgrade` and re-installs because it lives under
`$HOME` rather than next to the `locksmith` binary.

## Conflict policy

When a target file already exists:

- sha256 matches the manifest -> silent skip.
- sha256 differs -> prompt: `Overwrite | Keep | Overwrite all | Keep all`.
- "Keep" prints a warning: `kept; bundled version differs - functionality
  may not work as expected`.
- `--auto` mode defaults to Keep (do not surprise the user with overwrites
  in non-interactive flows).

## Updating extracted plugins

`locksmith plugins update` re-runs extraction for the vaults declared in
`config.yaml`. Same conflict logic as `init`.

- `--dry-run` prints what would change; no filesystem modifications.
- `--force` overwrites everything without prompting.

`plugins update` does NOT remove orphan plugin binaries.

## Custom plugins

Third-party plugins are not bundled. Drop a `locksmith-plugin-<type>` binary
into `~/.config/locksmith/plugins/` and `vault health` will pick it up. The
existing `CompatValidator` surfaces version mismatches as `compat_warnings`.
See [`authoring.md`](authoring.md) for the SDK and [`compatibility.md`](compatibility.md)
for the version-range protocol.
