package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/mcp"
)

func TestLoadServerConfig(t *testing.T) {
	yml := `
vaults:
  work:
    type: gopass
mcp:
  servers:
    github:
      command: ["npx", "-y", "@github/mcp"]
      env:
        GITHUB_TOKEN: github-token
    my-api:
      url: https://api.example.com
      transport: sse
      headers:
        Authorization: "Bearer {key:openai-key}"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yml), 0o600))

	got, err := mcp.LoadServerConfig(path, "github")
	require.NoError(t, err)
	assert.Equal(t, []string{"npx", "-y", "@github/mcp"}, got.Command)

	got, err = mcp.LoadServerConfig(path, "my-api")
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com", got.URL)
	assert.Equal(t, "sse", got.Transport)

	_, err = mcp.LoadServerConfig(path, "nonexistent")
	require.ErrorContains(t, err, "server \"nonexistent\" not found")
}

// Ensure LoadServerConfig accepts a config.MCPServerConfig directly (used in tests).
var _ = config.MCPServerConfig{}
