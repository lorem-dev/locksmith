//go:build darwin

package main

import (
	"context"
	"runtime"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
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
		return nil, keychainError(-25300, "The specified item could not be found in the keychain.")
	}
	defer func() { keychainGetPasswordFunc = orig }()

	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "key"})
	if err == nil {
		t.Fatal("GetSecret() expected error")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", err)
	}
	if s.Code() != codes.NotFound {
		t.Errorf("Code() = %v, want NotFound", s.Code())
	}
}

func TestParseKeychainPath_AccountOnly(t *testing.T) {
	svc, acc := parseKeychainPath("notion", "com.example.app")
	if svc != "com.example.app" || acc != "notion" {
		t.Errorf("got service=%q account=%q, want service=%q account=%q",
			svc, acc, "com.example.app", "notion")
	}
}

func TestParseKeychainPath_ServiceSlash(t *testing.T) {
	svc, acc := parseKeychainPath("github/token", "com.example.app")
	if svc != "github" || acc != "token" {
		t.Errorf("got service=%q account=%q, want service=%q account=%q",
			svc, acc, "github", "token")
	}
}

func TestParseKeychainPath_NoVaultService(t *testing.T) {
	svc, acc := parseKeychainPath("notion", "")
	if svc != "locksmith" || acc != "notion" {
		t.Errorf("got service=%q account=%q, want service=%q account=%q",
			svc, acc, "locksmith", "notion")
	}
}

func TestGetSecret_ServiceFromPath(t *testing.T) {
	var capturedService, capturedAccount string
	keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
		capturedService = service
		capturedAccount = account
		return []byte("secret"), nil
	}
	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "github/token",
		Opts: map[string]string{},
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if capturedService != "github" {
		t.Errorf("service = %q, want %q", capturedService, "github")
	}
	if capturedAccount != "token" {
		t.Errorf("account = %q, want %q", capturedAccount, "token")
	}
}

func TestGetSecret_ServiceFromOpts(t *testing.T) {
	var capturedService, capturedAccount string
	keychainGetPasswordFunc = func(service, account string) ([]byte, error) {
		capturedService = service
		capturedAccount = account
		return []byte("secret"), nil
	}
	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "notion",
		Opts: map[string]string{"service": "com.example.app"},
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if capturedService != "com.example.app" {
		t.Errorf("service = %q, want %q", capturedService, "com.example.app")
	}
	if capturedAccount != "notion" {
		t.Errorf("account = %q, want %q", capturedAccount, "notion")
	}
}

func TestGetSecret_NotFoundError(t *testing.T) {
	keychainGetPasswordFunc = func(_, _ string) ([]byte, error) {
		return nil, keychainError(-25300, "The specified item could not be found in the keychain.")
	}
	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "notion",
		Opts: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not gRPC status: %v", err)
	}
	if s.Code() != codes.NotFound {
		t.Errorf("Code() = %v, want NotFound", s.Code())
	}
}

func TestGetSecret_PermissionDeniedError(t *testing.T) {
	keychainGetPasswordFunc = func(_, _ string) ([]byte, error) {
		return nil, keychainError(-25293, "Auth failed")
	}
	p := &KeychainProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "secret",
		Opts: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not gRPC status: %v", err)
	}
	if s.Code() != codes.PermissionDenied {
		t.Errorf("Code() = %v, want PermissionDenied", s.Code())
	}
}

func TestKeychainError_Internal(t *testing.T) {
	err := keychainError(-99999, "Unknown error")
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not gRPC status: %v", err)
	}
	if s.Code() != codes.Internal {
		t.Errorf("Code() = %v, want Internal", s.Code())
	}
}

func TestSetSecret_Success(t *testing.T) {
	orig := keychainSetPasswordFunc
	defer func() { keychainSetPasswordFunc = orig }()

	var gotService, gotAccount string
	var gotSecret []byte
	keychainSetPasswordFunc = func(service, account string, secret []byte) error {
		gotService, gotAccount = service, account
		gotSecret = append([]byte(nil), secret...)
		return nil
	}

	p := &KeychainProvider{}
	_, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "github/token",
		Secret: []byte("ghp_xxx"),
		Opts:   map[string]string{"service": "com.acme.work"},
	})
	if err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if gotService != "github" || gotAccount != "token" {
		t.Errorf("service/account = %q/%q, want github/token (path overrides opts.service)", gotService, gotAccount)
	}
	if string(gotSecret) != "ghp_xxx" {
		t.Errorf("secret = %q, want ghp_xxx", gotSecret)
	}
}

func TestSetSecret_ServiceFromOpts(t *testing.T) {
	orig := keychainSetPasswordFunc
	defer func() { keychainSetPasswordFunc = orig }()

	var gotService string
	keychainSetPasswordFunc = func(service, account string, secret []byte) error {
		gotService = service
		return nil
	}

	p := &KeychainProvider{}
	if _, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "plain-account",
		Secret: []byte("v"),
		Opts:   map[string]string{"service": "com.acme.work"},
	}); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if gotService != "com.acme.work" {
		t.Errorf("service = %q, want com.acme.work", gotService)
	}
}

func TestSetSecret_EmptySecret(t *testing.T) {
	p := &KeychainProvider{}
	_, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "a",
		Secret: nil,
	})
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestSetSecret_PropagatesError(t *testing.T) {
	orig := keychainSetPasswordFunc
	defer func() { keychainSetPasswordFunc = orig }()
	keychainSetPasswordFunc = func(string, string, []byte) error {
		return keychainError(-25293, "errSecAuthFailed")
	}

	p := &KeychainProvider{}
	if _, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path: "a/b", Secret: []byte("v"),
	}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestKeyExists_True(t *testing.T) {
	orig := keychainExistsFunc
	defer func() { keychainExistsFunc = orig }()
	keychainExistsFunc = func(string, string) (bool, error) { return true, nil }

	p := &KeychainProvider{}
	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "github/token"})
	if err != nil {
		t.Fatalf("KeyExists: %v", err)
	}
	if !resp.Exists {
		t.Error("expected exists=true")
	}
}

func TestKeyExists_False(t *testing.T) {
	orig := keychainExistsFunc
	defer func() { keychainExistsFunc = orig }()
	keychainExistsFunc = func(string, string) (bool, error) { return false, nil }

	p := &KeychainProvider{}
	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "missing/token"})
	if err != nil {
		t.Fatalf("KeyExists: %v", err)
	}
	if resp.Exists {
		t.Error("expected exists=false")
	}
}

func TestInfoCompatibility(t *testing.T) {
	p := &KeychainProvider{}
	resp, err := p.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.MaxLocksmithVersion != sdkversion.Current {
		t.Errorf("MaxLocksmithVersion = %q, want %q",
			resp.MaxLocksmithVersion, sdkversion.Current)
	}
	if resp.MinLocksmithVersion == "" {
		t.Error("MinLocksmithVersion must not be empty")
	}
}
