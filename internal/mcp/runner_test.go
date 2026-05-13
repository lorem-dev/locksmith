package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
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

	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdout := os.Stdout
	os.Stdout = w

	runErr := mcp.Run(context.Background(), fetcher, mappings, []string{"sh", "-c", "printf '%s' \"$MY_SECRET\""})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	require.NoError(t, runErr)
	assert.Equal(t, "super-secret", buf.String())
}

func TestRun_NoCommand(t *testing.T) {
	err := mcp.Run(context.Background(), staticFetcher{}, nil, nil)
	require.ErrorContains(t, err, "no command specified")
}

func TestRun_FetchError(t *testing.T) {
	fetcher := staticFetcher{}
	mappings := []mcp.EnvMapping{
		{Var: "X", Ref: mcp.SecretRef{KeyAlias: "missing-key"}},
	}
	err := mcp.Run(context.Background(), fetcher, mappings, []string{"sh", "-c", "true"})
	require.ErrorContains(t, err, "missing-key")
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
