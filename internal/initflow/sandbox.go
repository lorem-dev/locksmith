package initflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// locksmithAllowList is the set of locksmith commands to permit in agent sandboxes.
var locksmithAllowList = []string{
	"Bash(locksmith get *)",
	"Bash(locksmith session start *)",
	"Bash(locksmith session end)",
	"Bash(locksmith vault list)",
	"Bash(locksmith vault health)",
}

// InstallSandboxPermissions adds locksmith commands to the agent's permission allowlist.
// Note: "Bash(...)" refers to the Claude Code tool name, not the user's shell —
// it works regardless of whether the user runs bash, zsh, or fish.
func InstallSandboxPermissions(agent DetectedAgent) error {
	switch agent.Name {
	case "Claude Code":
		return installClaudeSandbox(agent)
	case "Codex":
		return installCodexSandbox(agent)
	}
	return nil
}

func installClaudeSandbox(agent DetectedAgent) error {
	settingsPath := filepath.Join(agent.ConfigDir, "settings.json")
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &settings) //nolint:errcheck
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
	}
	allowList, _ := perms["allow"].([]interface{})

	existing := make(map[string]bool)
	for _, item := range allowList {
		if s, ok := item.(string); ok {
			existing[s] = true
		}
	}
	for _, perm := range locksmithAllowList {
		if !existing[perm] {
			allowList = append(allowList, perm)
		}
	}

	perms["allow"] = allowList
	settings["permissions"] = perms
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

func installCodexSandbox(agent DetectedAgent) error {
	policyPath := filepath.Join(agent.ConfigDir, "policy.yaml")
	content := "# Locksmith permissions\nallow:\n"
	for _, perm := range locksmithAllowList {
		cmd := perm
		if len(cmd) > 5 && cmd[:5] == "Bash(" {
			cmd = cmd[5 : len(cmd)-1]
		}
		content += fmt.Sprintf("  - %q\n", cmd)
	}
	return appendIfAbsent(policyPath, content, "# Locksmith permissions")
}
