package main

import (
	"context"
	"fmt"
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

func TestKeychainProvider_GetSecret_Success(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain only available on darwin")
	}
	orig := keychainGetPasswordFunc
	keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
		if service != "locksmith" {
			t.Errorf("service = %q, want %q", service, "locksmith")
		}
		if account != "my-key" {
			t.Errorf("account = %q, want %q", account, "my-key")
		}
		return []byte("test-secret"), nil
	}
	defer func() { keychainGetPasswordFunc = orig }()

	p := &KeychainProvider{}
	resp, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "my-key"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("Secret = %q, want %q", string(resp.Secret), "test-secret")
	}
	if resp.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want %q", resp.ContentType, "text/plain")
	}
}

func TestKeychainProvider_GetSecret_ServiceOverride(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain only available on darwin")
	}
	orig := keychainGetPasswordFunc
	keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
		if service != "my-service" {
			t.Errorf("service = %q, want %q", service, "my-service")
		}
		return []byte("secret"), nil
	}
	defer func() { keychainGetPasswordFunc = orig }()

	p := &KeychainProvider{}
	resp, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "key",
		Opts: map[string]string{"service": "my-service"},
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "secret" {
		t.Errorf("Secret = %q, want %q", string(resp.Secret), "secret")
	}
}

func TestKeychainProvider_GetSecret_Error(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain only available on darwin")
	}
	orig := keychainGetPasswordFunc
	keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
		return nil, fmt.Errorf("keychain: SecItemCopyMatching failed: -25300")
	}
	defer func() { keychainGetPasswordFunc = orig }()

	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "key"})
	if err == nil {
		t.Fatal("GetSecret() expected error")
	}
}
