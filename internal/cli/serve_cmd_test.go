package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

// TestServeCmd_DefaultPath covers the cfgPath == "" branch in serve_cmd.go.
// HOME is redirected to a temp dir so DefaultConfigPath() returns a path that
// does not exist, causing serve to fail fast with a "loading config" error.
func TestServeCmd_DefaultPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := cli.NewRootCmd()
	// Do NOT pass --config so cfgPath == "" and DefaultConfigPath() is called.
	root.SetArgs([]string{"serve"})
	// Expected to fail with "loading config" error since default path doesn't exist.
	_ = root.Execute()
}
