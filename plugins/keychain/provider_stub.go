//go:build !darwin

// Package main implements the locksmith-plugin-keychain binary.
// On non-macOS platforms, all operations return an unavailable error.
package main

import (
	"context"
	"fmt"
	"runtime"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// KeychainProvider is a no-op stub on non-macOS platforms.
type KeychainProvider struct{}

// GetSecret returns an error on non-macOS platforms.
func (p *KeychainProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return nil, fmt.Errorf("keychain is only available on macOS (current OS: %s)", runtime.GOOS)
}

// HealthCheck reports the keychain as unavailable on non-macOS platforms.
func (p *KeychainProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{
		Available: false,
		Message:   fmt.Sprintf("keychain is only available on macOS (current OS: %s)", runtime.GOOS),
	}, nil
}

// Info returns plugin metadata.
func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "keychain", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}
