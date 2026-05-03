# Writing a Vault Plugin

Locksmith vault plugins are standalone Go binaries that implement the
`VaultProviderService` gRPC service via the SDK.

## Quickstart

```bash
go mod init github.com/yourorg/locksmith-plugin-myvault
go get github.com/lorem-dev/locksmith/sdk
```

```go
package main

import (
    "context"
    sdk "github.com/lorem-dev/locksmith/sdk"
    vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

type MyVaultProvider struct{}

func (p *MyVaultProvider) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
    // fetch secret from your vault, trigger auth if needed
    return &vaultv1.GetSecretResponse{Secret: []byte("secret"), ContentType: "text/plain"}, nil
}

func (p *MyVaultProvider) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
    return &vaultv1.HealthCheckResponse{Available: true, Message: "ok"}, nil
}

func (p *MyVaultProvider) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
    return &vaultv1.InfoResponse{
        Name:                "myvault",
        Version:             "0.1.0",
        Platforms:           []string{"linux", "darwin"},
        MinLocksmithVersion: "0.1.0",
        // MaxLocksmithVersion: omit for open-ended compatibility
    }, nil
}

func main() { sdk.Serve(&MyVaultProvider{}) }
```

## Plugin Discovery

Name your binary `locksmith-plugin-<type>` and place it in one of:

1. Same directory as the `locksmith` binary
2. `~/.config/locksmith/plugins/`
3. Anywhere in `$PATH`

The daemon loads plugins for vault types referenced in `config.yaml`.

## Compatibility

When locksmith launches your plugin it calls `Info()` and checks compatibility. Issues
are reported as soft warnings - the plugin continues to run - and can be inspected
with `locksmith vault health`.

### Platforms

Set `Platforms` to the list of `runtime.GOOS` values your plugin supports. Use the
constants from `sdk/platform` to avoid typos:

    import "github.com/lorem-dev/locksmith/sdk/platform"
    Platforms: []string{platform.Darwin, platform.Linux}

If `Platforms` is empty the check is skipped.

### Locksmith version range

`MinLocksmithVersion` is the earliest locksmith version your plugin is tested against.
Declaring it is strongly recommended - omitting it produces a `min_version_missing`
warning visible in `locksmith vault health`.

`MaxLocksmithVersion` is the latest locksmith version your plugin is tested against.
Leave it empty for open-ended compatibility (no upper bound). Standard plugins bundled
with locksmith always set this field - it is enforced by a CI test.

Both fields use `major.minor.patch` semver format (no `v` prefix, no pre-release suffix).

### Checking warnings

After starting the daemon, run:

    locksmith vault health

Any compatibility warnings appear with a `!` prefix under the affected vault:

    gopass               OK    gopass found at /usr/bin/gopass
      ! platform_mismatch: plugin supports [darwin] but running on linux

## Built-in plugins

Locksmith ships with two reference plugins. Each has its own README that walks
through installation, configuration, and troubleshooting - useful when writing
your own plugin and looking for worked examples:

- [`plugins/gopass/README.md`](../plugins/gopass/README.md) - shells out to the `gopass` CLI; Linux + macOS.
- [`plugins/keychain/README.md`](../plugins/keychain/README.md) - macOS Security framework via CGo.
