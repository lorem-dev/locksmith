package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
)

func TestRun_InjectsEnvVar(t *testing.T) {
	fetcher := staticFetcher{"my-key": "super-secret"}
	mappings := []mcp.EnvMapping{
		{Var: "MY_SECRET", Ref: mcp.SecretRef{KeyAlias: "my-key"}},
	}
	// Stdin must contain at least one non-empty line so the lazy shim
	// resolves env and spawns the child. The child prints MY_SECRET and
	// exits; locksmith propagates exit 0.
	in := strings.NewReader("trigger\n")
	var out bytes.Buffer

	runErr := mcp.Run(
		context.Background(),
		fetcher,
		mappings,
		[]string{"sh", "-c", "printf '%s' \"$MY_SECRET\""},
		in,
		&out,
	)
	require.NoError(t, runErr)
	assert.Equal(t, "super-secret", out.String())
}

func TestRun_FetchError(t *testing.T) {
	fetcher := staticFetcher{}
	mappings := []mcp.EnvMapping{
		{Var: "X", Ref: mcp.SecretRef{KeyAlias: "missing-key"}},
	}
	in := strings.NewReader("trigger\n") // forces lazy resolve
	var out bytes.Buffer
	err := mcp.Run(
		context.Background(),
		fetcher,
		mappings,
		[]string{"sh", "-c", "true"},
		in,
		&out,
	)
	require.ErrorContains(t, err, "missing-key")
}

func TestRun_NoCommand(t *testing.T) {
	err := mcp.Run(
		context.Background(),
		staticFetcher{},
		nil,
		nil,
		strings.NewReader(""),
		io.Discard,
	)
	require.ErrorContains(t, err, "no command specified")
}

func TestPumpStdio_ForwardsFirstLineAndStdinThroughChild(t *testing.T) {
	// Use `sh -c 'cat'` as a passthrough child: whatever we write to its
	// stdin appears on its stdout verbatim. We expect pumpStdio to:
	//   1. write the buffered first line to the child's stdin,
	//   2. copy the rest of the parent's stdin reader to the child,
	//   3. copy the child's stdout to the parent's out writer.
	cmd := exec.Command("sh", "-c", "cat")
	stdinPipe, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	reader := bufio.NewReader(strings.NewReader("world\n"))
	firstLine := []byte("hello\n")
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- mcp.PumpStdio(stdinPipe, stdoutPipe, reader, firstLine, &out)
	}()

	pumpErr := <-done
	require.NoError(t, pumpErr)
	require.NoError(t, cmd.Wait())

	assert.Equal(t, "hello\nworld\n", out.String())
}

func TestForwardSignals_DeliversSIGTERMToChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM semantics differ on Windows")
	}
	// The child prints "ready\n" once the TERM trap is installed so we
	// can avoid a race where SIGTERM arrives before sh has set up the
	// handler (in which case sh dies with the default action instead of
	// running the trap). The sleep is backgrounded and waited on with
	// `wait` so the trap can interrupt `wait` immediately - if sleep ran
	// in the foreground sh would not service the trap until sleep
	// returned, making the test take 30 s.
	cmd := exec.Command("sh", "-c", `trap 'exit 42' TERM; echo ready; sleep 30 & wait`)
	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	rd := bufio.NewReader(stdoutPipe)
	line, err := rd.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", line)

	sigCh := make(chan os.Signal, 1)
	done := make(chan struct{})
	go mcp.ForwardSignals(cmd.Process, sigCh, done)

	sigCh <- syscall.SIGTERM

	waitErr := cmd.Wait()
	close(done)

	var exitErr *exec.ExitError
	require.ErrorAs(t, waitErr, &exitErr, "child should have exited with non-zero")
	assert.Equal(t, 42, exitErr.ExitCode(), "child trap should have produced exit 42")
}

func TestRun_LazyFetch_NoStdin_NoChild(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "child-ran")
	fetcher := recordingFetcher{}
	in := strings.NewReader("") // immediate EOF
	var out bytes.Buffer

	err := mcp.Run(
		context.Background(),
		&fetcher,
		nil,
		[]string{"sh", "-c", "touch " + sentinel},
		in,
		&out,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, fetcher.calls, "fetcher must not be called when stdin is empty")

	_, statErr := os.Stat(sentinel)
	assert.True(t, os.IsNotExist(statErr), "child must not have been spawned")
}

func TestRun_LazyFetch_FirstLineTriggersChild(t *testing.T) {
	fetcher := recordingFetcher{inner: staticFetcher{"my-key": "super-secret"}}
	mappings := []mcp.EnvMapping{
		{Var: "MY_SECRET", Ref: mcp.SecretRef{KeyAlias: "my-key"}},
	}
	in := strings.NewReader("hello\n")
	var out bytes.Buffer

	err := mcp.Run(
		context.Background(),
		&fetcher,
		mappings,
		// Child echoes its stdin (including the buffered "hello\n") and
		// then exits when stdin closes.
		[]string{"sh", "-c", "cat"},
		in,
		&out,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, fetcher.calls)
	assert.Equal(t, "hello\n", out.String())
}

// recordingFetcher wraps a SecretFetcher and counts Fetch calls so
// lazy-fetch tests can assert when (and whether) the vault was hit.
type recordingFetcher struct {
	inner staticFetcher
	calls int
}

func (r *recordingFetcher) Fetch(ctx context.Context, ref mcp.SecretRef) (string, error) {
	r.calls++
	if r.inner == nil {
		return "", nil
	}
	return r.inner.Fetch(ctx, ref)
}
