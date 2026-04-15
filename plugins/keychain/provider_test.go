package main

import (
	"context"
	"runtime"
	"testing"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func TestKeychainProvider_Info(t *testing.T) {
	p := &KeychainProvider{}
	info, err := p.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "keychain" {
		t.Errorf("Name = %q, want %q", info.Name, "keychain")
	}
	if len(info.Platforms) == 0 {
		t.Error("Platforms is empty")
	}
}

func TestKeychainProvider_HealthCheck(t *testing.T) {
	p := &KeychainProvider{}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if runtime.GOOS == "darwin" && !resp.Available {
		t.Error("keychain should be available on macOS")
	}
	if runtime.GOOS != "darwin" && resp.Available {
		t.Error("keychain should not be available on non-macOS")
	}
}
