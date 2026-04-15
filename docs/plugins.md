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
    return &vaultv1.InfoResponse{Name: "myvault", Version: "0.1.0", Platforms: []string{"linux"}}, nil
}

func main() { sdk.Serve(&MyVaultProvider{}) }
```

## Plugin Discovery

Name your binary `locksmith-plugin-<type>` and place it in one of:

1. Same directory as the `locksmith` binary
2. `~/.config/locksmith/plugins/`
3. Anywhere in `$PATH`

The daemon loads plugins for vault types referenced in `config.yaml`.
