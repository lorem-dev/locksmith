package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestMCPRun_MutualExclusivity_URLandDash(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"mcp", "run", "--url", "https://example.com", "--", "npx", "-y", "foo"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url and -- are mutually exclusive")
}

func TestMCPRun_MutualExclusivity_ServerAndFlags(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"mcp", "run", "--server", "github", "--env", "X=y"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--server cannot be combined")
}

func TestMCPRun_NoMode(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"mcp", "run"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify --url, --server, or a command after --")
}

func TestMCPRun_InvalidEnvArg(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"mcp", "run", "--env", "NOEQUAL", "--", "sh", "-c", "true"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected VAR=ref")
}

func TestMCPRun_InvalidHeaderArg(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"mcp", "run", "--url", "https://example.com", "--header", "NOEQUAL"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected Name=template")
}
