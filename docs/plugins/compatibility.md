# Plugin Compatibility

When locksmith launches a plugin it calls `Info()` and checks compatibility.
Issues are reported as soft warnings - the plugin continues to run - and can
be inspected with `locksmith vault health`.

## Platforms

Set `Platforms` to the list of `runtime.GOOS` values your plugin supports. Use the
constants from `sdk/platform` to avoid typos:

    import "github.com/lorem-dev/locksmith/sdk/platform"
    Platforms: []string{platform.Darwin, platform.Linux}

If `Platforms` is empty the check is skipped.

## Locksmith version range

`MinLocksmithVersion` is the earliest locksmith version your plugin is tested against.
Declaring it is strongly recommended - omitting it produces a `min_version_missing`
warning visible in `locksmith vault health`.

`MaxLocksmithVersion` is the latest locksmith version your plugin is tested against.
Leave it empty for open-ended compatibility (no upper bound). Standard plugins bundled
with locksmith always set this field - it is enforced by a CI test.

Both fields use `major.minor.patch` semver format (no `v` prefix, no pre-release suffix).

## Checking warnings

After starting the daemon, run:

    locksmith vault health

Any compatibility warnings appear with a `!` prefix under the affected vault:

    gopass               OK    gopass found at /usr/bin/gopass
      ! platform_mismatch: plugin supports [darwin] but running on linux

## RPCs

| RPC | Added in | Required | Behaviour for unsupported vaults |
|---|---|---|---|
| `GetSecret` | v0.1.0 | yes | n/a |
| `HealthCheck` | v0.1.0 | yes | n/a |
| `Info` | v0.1.0 | yes | n/a |
| `SetSecret` | v0.4.0 | yes | return `codes.Unimplemented` |
| `KeyExists` | v0.4.0 | yes | return `codes.Unimplemented` (host skips strict check) |

### v0.4.0 plugin compatibility

`SetSecret` and `KeyExists` are additive RPCs. Plugins built against
the v0.3.x SDK do not implement them; the host's CLI commands that
call them surface `Unimplemented` to the user. Plugins rebuilt
against the v0.4.0+ SDK must add both methods to satisfy the
`vault.Provider` Go interface.
