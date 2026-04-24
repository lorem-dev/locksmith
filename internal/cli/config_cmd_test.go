package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

// TestConfigCheck_DefaultPath covers the cfgPath == "" branch in config_cmd.go.
// HOME is redirected so DefaultConfigPath() returns a nonexistent path.
func TestConfigCheck_DefaultPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := cli.NewRootCmd()
	// Do NOT pass --config so that cfgPath == "" and DefaultConfigPath() is called.
	root.SetArgs([]string{"config", "check"})
	// Will fail (no config), but covers the DefaultConfigPath() branch.
	_ = root.Execute()
}
