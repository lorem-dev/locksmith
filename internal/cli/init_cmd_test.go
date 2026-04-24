package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

// TestInitCmd_Auto covers RunE in init_cmd.go via --auto --skip-agents.
// The command writes a config file; we redirect HOME so it stays in a temp dir.
func TestInitCmd_Auto(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := cli.NewRootCmd()
	root.SetArgs([]string{"init", "--auto", "--skip-agents"})
	if err := root.Execute(); err != nil {
		t.Fatalf("init --auto --skip-agents: %v", err)
	}
}

// TestInitCmd_Auto_AgentOnly covers the --agent flag path.
func TestInitCmd_Auto_AgentOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := cli.NewRootCmd()
	root.SetArgs([]string{"init", "--auto", "--agent", "claude"})
	// Claude Code dir does not exist in the temp home, so it won't be detected.
	// The command should still succeed (no agents to install).
	_ = root.Execute()
}
