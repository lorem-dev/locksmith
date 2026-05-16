// Package main implements the locksmith gopass vault plugin.
// It shells out to the `gopass` CLI to retrieve secrets.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
)

// GopassProvider retrieves secrets from a gopass password store.
// lookPath, runCmd, and cmdFactory are injectable for testing; zero values use the real exec functions.
type GopassProvider struct {
	// lookPath is used to locate the gopass binary. Defaults to exec.LookPath.
	lookPath func(string) (string, error)
	// runCmd is used to run a command and return its combined exit status. Defaults to cmd.Run().
	runCmd func(name string, args ...string) error
	// cmdFactory builds the exec.Cmd for GetSecret calls. Nil uses exec.CommandContext.
	cmdFactory func(ctx context.Context, name string, args ...string) *exec.Cmd
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

func (p *GopassProvider) resolveCmdFactory() func(context.Context, string, ...string) *exec.Cmd {
	if p.cmdFactory != nil {
		return p.cmdFactory
	}
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, name, args...)
	}
}

// buildGopassEnv returns the explicit environment for gopass subprocesses.
// Only non-empty variables are included to avoid overriding with empty strings.
// HOME, PATH, and GNUPGHOME ensure gopass can locate gpg-agent.
// DISPLAY, WAYLAND_DISPLAY, and GPG_TTY allow gpg-agent to prompt for the passphrase.
func buildGopassEnv() []string {
	keys := []string{"HOME", "PATH", "GNUPGHOME", "DISPLAY", "WAYLAND_DISPLAY", "GPG_TTY"}
	var env []string
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// GetSecret fetches a secret from gopass by path. Optionally uses a named store
// via opts["store"]. Authorization (GPG passphrase / Touch ID) is handled by gopass.
func (p *GopassProvider) GetSecret(
	ctx context.Context,
	req *vaultv1.GetSecretRequest,
) (*vaultv1.GetSecretResponse, error) {
	secretPath := req.Path
	if store, ok := req.Opts["store"]; ok && store != "" {
		secretPath = store + "/" + req.Path
	}

	factory := p.resolveCmdFactory()
	var stdout, stderr bytes.Buffer
	cmd := factory(ctx, "gopass", "show", "-o", secretPath)
	cmd.Env = buildGopassEnv()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if strings.Contains(stderrStr, "Inappropriate ioctl for device") {
			return nil, sdkerrors.UnauthenticatedError(
				"GPG passphrase required but no UI available - see docs/configuration.md#gpg-pinentry")
		}
		return nil, fmt.Errorf("gopass show %q: %s: %w", secretPath, stderrStr, err)
	}

	return &vaultv1.GetSecretResponse{
		Secret:      bytes.TrimRight(stdout.Bytes(), "\n"),
		ContentType: "text/plain",
	}, nil
}

// SetSecret is a stub pending implementation in a follow-up task.
func (p *GopassProvider) SetSecret(
	_ context.Context,
	_ *vaultv1.SetSecretRequest,
) (*vaultv1.SetSecretResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "gopass SetSecret pending implementation")
}

// KeyExists is a stub pending implementation in a follow-up task.
func (p *GopassProvider) KeyExists(
	_ context.Context,
	_ *vaultv1.KeyExistsRequest,
) (*vaultv1.KeyExistsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "gopass KeyExists pending implementation")
}

// HealthCheck verifies that gopass is installed and the store is initialized.
func (p *GopassProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
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
func (p *GopassProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{
		Name:                "gopass",
		Version:             "0.1.0",
		Platforms:           []string{"darwin", "linux"},
		MinLocksmithVersion: "0.1.0",
		MaxLocksmithVersion: sdkversion.Current,
	}, nil
}
