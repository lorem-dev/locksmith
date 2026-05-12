package mcp_test

import (
	"bytes"
	"context"
	"io"
	"os"
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
