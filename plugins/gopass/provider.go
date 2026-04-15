// Package main implements the locksmith gopass vault plugin.
// It shells out to the `gopass` CLI to retrieve secrets.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// GopassProvider retrieves secrets from a gopass password store.
// lookPath and runCmd are injectable for testing; zero values use the real exec functions.
type GopassProvider struct {
	// lookPath is used to locate the gopass binary. Defaults to exec.LookPath.
	lookPath func(string) (string, error)
	// runCmd is used to run a command and return its combined exit status. Defaults to cmd.Run().
	runCmd func(name string, args ...string) error
}

func (p *GopassProvider) resolveLookPath() func(string) (string, error) {
	if p.lookPath != nil {
		return p.lookPath
	}
	return exec.LookPath
}

func (p *GopassProvider) resolveRunCmd() func(string, ...string) error {
	if p.runCmd != nil {
		return p.runCmd
	}
	return func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
}

// GetSecret fetches a secret from gopass by path. Optionally uses a named store
// via opts["store"]. Authorization (GPG passphrase / Touch ID) is handled by gopass.
func (p *GopassProvider) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	secretPath := req.Path
	if store, ok := req.Opts["store"]; ok && store != "" {
		secretPath = store + "/" + req.Path
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gopass", "show", "-o", secretPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gopass show %q: %s: %w", secretPath, strings.TrimSpace(stderr.String()), err)
	}

	return &vaultv1.GetSecretResponse{
		Secret:      bytes.TrimRight(stdout.Bytes(), "\n"),
		ContentType: "text/plain",
	}, nil
}

// HealthCheck verifies that gopass is installed and the store is initialized.
func (p *GopassProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	lookPath := p.resolveLookPath()
	runCmd := p.resolveRunCmd()

	path, err := lookPath("gopass")
	if err != nil {
		return &vaultv1.HealthCheckResponse{Available: false, Message: "gopass not found in PATH"}, nil
	}
	if err := runCmd("gopass", "ls", "--flat"); err != nil {
		return &vaultv1.HealthCheckResponse{
			Available: false,
			Message:   fmt.Sprintf("gopass at %s is not initialized: %v", path, err),
		}, nil
	}
	return &vaultv1.HealthCheckResponse{Available: true, Message: fmt.Sprintf("gopass available at %s", path)}, nil
}

// Info returns plugin metadata.
func (p *GopassProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{
		Name:      "gopass",
		Version:   "0.1.0",
		Platforms: []string{"darwin", "linux"},
	}, nil
}
