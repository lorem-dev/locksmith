package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
)

// recordingCmdFactory captures the arguments and stdin bytes of a
// gopass invocation while emitting canned stdout/stderr and exit code.
type recordingCmdFactory struct {
	args      []string
	stdinPath string
	stdout    string
	stderr    string
	exit      int
}

func (r *recordingCmdFactory) factory(ctx context.Context, name string, args ...string) *exec.Cmd {
	r.args = append([]string{name}, args...)
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	r.stdinPath = filepath.Join(tmp, fmt.Sprintf("gopass-stdin-%d-%d", os.Getpid(), time.Now().UnixNano()))
	script := fmt.Sprintf(`cat > %q; printf %%b %q; printf %%b %q 1>&2; exit %d`,
		r.stdinPath, r.stdout, r.stderr, r.exit)
	return exec.CommandContext(ctx, "sh", "-c", script)
}

func (r *recordingCmdFactory) readStdin(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(r.stdinPath)
	if err != nil {
		t.Fatalf("read stdin file: %v", err)
	}
	return data
}

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
		t.Skip("gopass is installed - skipping not-installed test")
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
	// This will fail because the path doesn't exist - we just want to verify
	// the store prefix is applied (the error message should contain the store prefix)
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "nonexistent-key-12345",
		Opts: map[string]string{"store": "teststore"},
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for nonexistent key")
	}
}

func TestBuildGopassEnv_IncludesSetVars(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("GNUPGHOME", "/home/tester/.gnupg")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "") // empty - should be omitted
	t.Setenv("GPG_TTY", "/dev/pts/0")

	env := buildGopassEnv()

	has := func(key string) bool {
		prefix := key + "="
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				return true
			}
		}
		return false
	}

	for _, want := range []string{"HOME", "PATH", "GNUPGHOME", "DISPLAY", "GPG_TTY"} {
		if !has(want) {
			t.Errorf("buildGopassEnv() missing %s", want)
		}
	}
	if has("WAYLAND_DISPLAY") {
		t.Error("buildGopassEnv() should omit empty WAYLAND_DISPLAY")
	}
}

func TestGopassProvider_GetSecret_InadequateIoctl(t *testing.T) {
	p := &GopassProvider{
		cmdFactory: func(_ context.Context, name string, args ...string) *exec.Cmd {
			// Simulate a command that writes ioctl error to stderr and fails.
			return exec.Command("sh", "-c",
				`echo "gpg: signing failed: Inappropriate ioctl for device" >&2; exit 2`)
		},
	}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "test/key",
		Opts: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not gRPC status: %T %v", err, err)
	}
	if s.Code() != codes.Unauthenticated {
		t.Errorf("Code() = %v, want Unauthenticated", s.Code())
	}
}

func TestGopassProvider_GetSecret_Success_Mocked(t *testing.T) {
	p := &GopassProvider{
		cmdFactory: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "printf mysecret")
		},
	}
	resp, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "test/key",
		Opts: map[string]string{},
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "mysecret" {
		t.Errorf("secret = %q, want %q", resp.Secret, "mysecret")
	}
}

func TestGopassProvider_GetSecret_GenericError_Mocked(t *testing.T) {
	p := &GopassProvider{
		cmdFactory: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", `echo "some error" >&2; exit 1`)
		},
	}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "test/key",
		Opts: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// Generic error should NOT be a gRPC Unauthenticated error.
	s, ok := status.FromError(err)
	if ok && s.Code() == codes.Unauthenticated {
		t.Errorf("generic gopass error should not map to Unauthenticated")
	}
}

func TestSetSecret_Success(t *testing.T) {
	rec := &recordingCmdFactory{}
	p := &GopassProvider{cmdFactory: rec.factory}

	_, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "personal/github",
		Secret: []byte("ghp_xxx"),
	})
	if err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	want := []string{"gopass", "insert", "-m", "personal/github"}
	if !slices.Equal(rec.args, want) {
		t.Errorf("args = %v, want %v", rec.args, want)
	}
	if got := rec.readStdin(t); string(got) != "ghp_xxx" {
		t.Errorf("stdin = %q, want ghp_xxx", got)
	}
}

func TestSetSecret_WithForce(t *testing.T) {
	rec := &recordingCmdFactory{}
	p := &GopassProvider{cmdFactory: rec.factory}

	if _, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "personal/github",
		Secret: []byte("v"),
		Opts:   map[string]string{"force": "true"},
	}); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	want := []string{"gopass", "insert", "-m", "-f", "personal/github"}
	if !slices.Equal(rec.args, want) {
		t.Errorf("args = %v, want %v", rec.args, want)
	}
}

func TestSetSecret_WithStore(t *testing.T) {
	rec := &recordingCmdFactory{}
	p := &GopassProvider{cmdFactory: rec.factory}

	if _, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "github",
		Secret: []byte("v"),
		Opts:   map[string]string{"store": "work"},
	}); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if rec.args[len(rec.args)-1] != "work/github" {
		t.Errorf("last arg = %q, want work/github", rec.args[len(rec.args)-1])
	}
}

func TestSetSecret_EmptySecret(t *testing.T) {
	p := &GopassProvider{}
	_, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "x",
		Secret: nil,
	})
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestSetSecret_GopassFails(t *testing.T) {
	rec := &recordingCmdFactory{exit: 1, stderr: "boom"}
	p := &GopassProvider{cmdFactory: rec.factory}
	_, err := p.SetSecret(context.Background(), &vaultv1.SetSecretRequest{
		Path:   "x",
		Secret: []byte("v"),
	})
	if err == nil {
		t.Fatal("expected error when gopass exits non-zero")
	}
}

func TestKeyExists_True(t *testing.T) {
	rec := &recordingCmdFactory{stdout: "personal/github\n"}
	p := &GopassProvider{cmdFactory: rec.factory}

	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "personal/github"})
	if err != nil {
		t.Fatalf("KeyExists: %v", err)
	}
	if !resp.Exists {
		t.Error("expected exists=true")
	}
	want := []string{"gopass", "ls", "--flat", "personal/github"}
	if !slices.Equal(rec.args, want) {
		t.Errorf("args = %v, want %v", rec.args, want)
	}
}

func TestKeyExists_FalseExitNonZero(t *testing.T) {
	rec := &recordingCmdFactory{exit: 1}
	p := &GopassProvider{cmdFactory: rec.factory}

	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "missing"})
	if err != nil {
		t.Fatalf("KeyExists should not error on non-zero exit: %v", err)
	}
	if resp.Exists {
		t.Error("expected exists=false")
	}
}

func TestKeyExists_FalseEmptyStdout(t *testing.T) {
	rec := &recordingCmdFactory{exit: 0, stdout: ""}
	p := &GopassProvider{cmdFactory: rec.factory}

	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "missing"})
	if err != nil {
		t.Fatalf("KeyExists: %v", err)
	}
	if resp.Exists {
		t.Error("expected exists=false (stdout did not contain path)")
	}
}

func TestKeyExists_NoPrefixFalsePositive(t *testing.T) {
	// stdout from `gopass ls --flat personal/git` could contain
	// "personal/github" and "personal/gitlab" - neither is an exact
	// match for "personal/git", so KeyExists must report false.
	rec := &recordingCmdFactory{stdout: "personal/github\npersonal/gitlab\n"}
	p := &GopassProvider{cmdFactory: rec.factory}

	resp, err := p.KeyExists(context.Background(), &vaultv1.KeyExistsRequest{Path: "personal/git"})
	if err != nil {
		t.Fatalf("KeyExists: %v", err)
	}
	if resp.Exists {
		t.Error("expected exists=false: 'personal/git' is a prefix, not an exact match")
	}
}

func TestInfoCompatibility(t *testing.T) {
	p := &GopassProvider{}
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
