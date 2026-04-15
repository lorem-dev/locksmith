package initflow_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestDetectAgents_ClaudeCode_ByDir(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	agents := initflow.DetectAgents(home)
	for _, a := range agents {
		if a.Name == "Claude Code" {
			if !a.Detected {
				t.Error("Claude Code should be detected via directory")
			}
			return
		}
	}
	t.Error("Claude Code not in agent list")
}

func TestDetectAgents_NoneFound(t *testing.T) {
	// This test verifies that agents without a config directory are not detected
	// via the directory strategy. Agents whose CLI binary is in PATH will still
	// be detected via the binary fallback, which is correct behaviour.
	home := t.TempDir()
	for _, a := range initflow.DetectAgents(home) {
		if a.Detected {
			// Skip agents that were legitimately found via binary-in-PATH fallback.
			if _, err := exec.LookPath(a.BinaryName); err == nil {
				continue
			}
			t.Errorf("agent %q should not be detected in empty home (no dir, no binary)", a.Name)
		}
	}
}

func TestDetectVaults_ContainsGopass(t *testing.T) {
	vaults := initflow.DetectVaults()
	for _, v := range vaults {
		if v.Type == "gopass" {
			return
		}
	}
	t.Error("gopass should always be in vault list")
}

func TestDetectVaults_KeychainInList(t *testing.T) {
	vaults := initflow.DetectVaults()
	for _, v := range vaults {
		if v.Type == "keychain" {
			return
		}
	}
	t.Error("keychain should always be in vault list")
}
