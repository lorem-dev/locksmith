package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestNewRootCmd_HasSubcommands(t *testing.T) {
	root := cli.NewRootCmd()
	if root == nil {
		t.Fatal("NewRootCmd() returned nil")
	}
	want := []string{"serve", "get", "session", "vault", "config", "init"}
	for _, name := range want {
		found := false
		for _, cmd := range root.Commands() {
			if cmd.Use == name || cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("command %q not found in root", name)
		}
	}
}

func TestNewRootCmd_Help(t *testing.T) {
	root := cli.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	// Execute returns an error for --help (exits with 0 but cobra returns it)
	root.Execute() //nolint:errcheck
	if buf.Len() == 0 {
		t.Error("expected help output")
	}
}

func TestGetCmd_NoDaemon(t *testing.T) {
	// When no daemon is running, locksmith get should fail regardless of session state.
	root := cli.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"get", "--key", "test"})
	t.Setenv("LOCKSMITH_SESSION", "")
	t.Setenv("LOCKSMITH_SOCKET", "/tmp/locksmith-nonexistent-test.sock")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when daemon is not running")
	}
}

func TestSessionEndCmd_NoSession(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"session", "end"})
	t.Setenv("LOCKSMITH_SESSION", "")
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no session ID provided")
	}
}

func TestConfigCheck_NoConfig(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", "/nonexistent/config.yaml", "config", "check"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestSessionCmd_HasSubcommands(t *testing.T) {
	root := cli.NewRootCmd()
	var sessionCmd interface{ Commands() interface{} }
	_ = sessionCmd
	for _, cmd := range root.Commands() {
		if cmd.Name() == "session" {
			subs := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subs[sub.Name()] = true
			}
			for _, want := range []string{"start", "end", "list"} {
				if !subs[want] {
					t.Errorf("session subcommand %q not found", want)
				}
			}
			return
		}
	}
	t.Fatal("session command not found")
}

func TestVaultCmd_HasSubcommands(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "vault" {
			subs := map[string]bool{}
			for _, sub := range cmd.Commands() {
				subs[sub.Name()] = true
			}
			for _, want := range []string{"list", "health"} {
				if !subs[want] {
					t.Errorf("vault subcommand %q not found", want)
				}
			}
			return
		}
	}
	t.Fatal("vault command not found")
}

func TestSessionStartCmd_Flags(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "session" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "start" {
					if sub.Flags().Lookup("ttl") == nil {
						t.Error("session start: missing --ttl flag")
					}
					if sub.Flags().Lookup("keys") == nil {
						t.Error("session start: missing --keys flag")
					}
					return
				}
			}
		}
	}
	t.Fatal("session start command not found")
}

func TestSessionEndCmd_HasSessionFlag(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "session" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "end" {
					if sub.Flags().Lookup("session") == nil {
						t.Error("session end: missing --session flag")
					}
					return
				}
			}
		}
	}
	t.Fatal("session end command not found")
}

func TestGetCmd_Flags(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "get" {
			for _, flag := range []string{"key", "vault", "path"} {
				if cmd.Flags().Lookup(flag) == nil {
					t.Errorf("get: missing --%s flag", flag)
				}
			}
			return
		}
	}
	t.Fatal("get command not found")
}

func TestRootCmd_PersistentConfigFlag(t *testing.T) {
	root := cli.NewRootCmd()
	if root.PersistentFlags().Lookup("config") == nil {
		t.Error("root: missing --config persistent flag")
	}
}

func TestRootCmd_Use(t *testing.T) {
	root := cli.NewRootCmd()
	if root.Use != "locksmith" {
		t.Errorf("root Use = %q, want %q", root.Use, "locksmith")
	}
}

func TestServeCmd_Present(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "serve" {
			return
		}
	}
	t.Fatal("serve command not found")
}

func TestConfigCmd_CheckSubcommand(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "config" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == "check" {
					return
				}
			}
			t.Fatal("config check subcommand not found")
		}
	}
	t.Fatal("config command not found")
}

func TestRootCmd_ShortDescription(t *testing.T) {
	root := cli.NewRootCmd()
	if root.Short == "" {
		t.Error("root command has no Short description")
	}
	if !strings.Contains(root.Long, "vault") {
		t.Error("root Long description should mention vault")
	}
}

func TestInitCmd_Present(t *testing.T) {
	root := cli.NewRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Name() == "init" {
			return
		}
	}
	t.Fatal("init command not found")
}
