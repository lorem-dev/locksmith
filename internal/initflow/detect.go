// Package initflow implements the interactive locksmith setup wizard.
package initflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// DetectedAgent describes an AI agent installation found on the system.
type DetectedAgent struct {
	Name       string
	Detected   bool
	ConfigDir  string
	HomePath   string // path relative to home to check for existence
	BinaryName string // CLI binary name to check in PATH as fallback
}

// DetectedVault describes a vault backend found on the system.
type DetectedVault struct {
	Type      string
	Detected  bool
	Available bool // false on unsupported platforms
}

// DetectAgents scans homeDir for known AI agent installations.
// Detection uses two strategies: directory existence and CLI binary in PATH.
func DetectAgents(homeDir string) []DetectedAgent {
	agents := []DetectedAgent{
		{Name: "Claude Code", HomePath: ".claude", BinaryName: "claude"},
		{Name: "Codex", HomePath: ".codex", BinaryName: "codex"},
		{Name: "OpenCode", HomePath: filepath.Join(".config", "opencode"), BinaryName: "opencode"},
	}

	for i := range agents {
		a := &agents[i]
		dirPath := filepath.Join(homeDir, a.HomePath)

		// Strategy 1: config directory exists
		if _, err := os.Stat(dirPath); err == nil {
			a.Detected = true
			a.ConfigDir = dirPath
		}

		// Strategy 2: CLI binary in PATH (fallback — agent may not have created dir yet)
		if !a.Detected {
			if _, err := exec.LookPath(a.BinaryName); err == nil {
				a.Detected = true
				a.ConfigDir = dirPath // use expected dir even if not yet created
			}
		}
	}

	return agents
}

// DetectVaults returns all known vault types with platform availability and
// installation status for the current system.
func DetectVaults() []DetectedVault {
	vaults := []DetectedVault{
		{Type: "keychain", Available: runtime.GOOS == "darwin"},
		{Type: "gopass", Available: true},
		{Type: "1password", Available: true},
		{Type: "gnome-keyring", Available: runtime.GOOS == "linux"},
	}

	for i := range vaults {
		v := &vaults[i]
		switch v.Type {
		case "keychain":
			v.Detected = runtime.GOOS == "darwin"
		case "gopass":
			v.Detected = binaryExists("gopass")
		case "1password":
			v.Detected = binaryExists("op")
		case "gnome-keyring":
			v.Detected = binaryExists("secret-tool")
		}
	}
	return vaults
}

func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
