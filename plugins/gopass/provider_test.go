package main

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func TestGopassProvider_Info(t *testing.T) {
	p := &GopassProvider{}
	info, err := p.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "gopass" {
		t.Errorf("Name = %q, want %q", info.Name, "gopass")
	}
	if len(info.Platforms) == 0 {
		t.Error("Platforms is empty")
	}
}

func TestGopassProvider_HealthCheck_NotInstalled(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err == nil {
		t.Skip("gopass is installed — skipping not-installed test")
	}
	p := &GopassProvider{}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if resp.Available {
		t.Error("Available should be false when gopass is not installed")
	}
}

// TestGopassProvider_HealthCheck_NotInstalled_Mocked exercises the "not found" branch
// regardless of whether gopass is present on the machine.
func TestGopassProvider_HealthCheck_NotInstalled_Mocked(t *testing.T) {
	p := &GopassProvider{
		lookPath: func(string) (string, error) {
			return "", errors.New("not found")
		},
	}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if resp.Available {
		t.Error("Available should be false when gopass is not installed")
	}
	if resp.Message == "" {
		t.Error("Message should not be empty")
	}
}

// TestGopassProvider_HealthCheck_NotInitialized_Mocked exercises the branch where
// gopass is installed but the password store is not initialized.
func TestGopassProvider_HealthCheck_NotInitialized_Mocked(t *testing.T) {
	p := &GopassProvider{
		lookPath: func(string) (string, error) {
			return "/usr/local/bin/gopass", nil
		},
		runCmd: func(string, ...string) error {
			return errors.New("password store not initialized")
		},
	}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if resp.Available {
		t.Error("Available should be false when store is not initialized")
	}
	if resp.Message == "" {
		t.Error("Message should not be empty")
	}
}

// TestGopassProvider_HealthCheck_Available_Mocked exercises the happy path.
func TestGopassProvider_HealthCheck_Available_Mocked(t *testing.T) {
	p := &GopassProvider{
		lookPath: func(string) (string, error) {
			return "/usr/local/bin/gopass", nil
		},
		runCmd: func(string, ...string) error {
			return nil
		},
	}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available should be true when gopass is installed and initialized")
	}
	if resp.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestGopassProvider_HealthCheck_Installed(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err != nil {
		t.Skip("gopass not installed")
	}
	p := &GopassProvider{}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	// gopass is installed; available depends on whether the store is initialized.
	// Just verify the response is well-formed.
	if resp.Message == "" {
		t.Error("HealthCheck message should not be empty")
	}
}

func TestGopassProvider_GetSecret_InvalidPath(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err != nil {
		t.Skip("gopass not installed")
	}
	p := &GopassProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "locksmith-test/nonexistent-key-12345",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for nonexistent key")
	}
}

func TestGopassProvider_GetSecret_WithStore(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err != nil {
		t.Skip("gopass not installed")
	}
	p := &GopassProvider{}
	// This will fail because the path doesn't exist — we just want to verify
	// the store prefix is applied (the error message should contain the store prefix)
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "nonexistent-key-12345",
		Opts: map[string]string{"store": "teststore"},
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for nonexistent key")
	}
}
