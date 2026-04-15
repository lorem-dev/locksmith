package initflow_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestInstall_ClaudeCode_CreatesSkillAndUpdatesClaudeMd(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(claudeDir, "skills", "locksmith.md")); err != nil {
		t.Error("skill file not created")
	}
	content, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if !strings.Contains(string(content), "Locksmith Integration") {
		t.Error("CLAUDE.md missing Locksmith section")
	}
}

func TestInstall_ClaudeCode_Idempotent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	writer := initflow.NewAgentWriter(home)
	writer.Install(agent)
	writer.Install(agent) // second call must not duplicate

	content, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if strings.Count(string(content), "## Locksmith Integration") != 1 {
		t.Errorf("section duplicated; count = %d", strings.Count(string(content), "## Locksmith Integration"))
	}
}

func TestInstall_ClaudeCode_AppendsToClaude_ExistingContent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	// Pre-populate CLAUDE.md with existing content.
	existingContent := "# My Project\n\nSome existing rules."
	os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existingContent), 0o644)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	// Both original and locksmith sections must be present.
	if !strings.Contains(string(content), "My Project") {
		t.Error("existing content was overwritten")
	}
	if !strings.Contains(string(content), "Locksmith Integration") {
		t.Error("Locksmith section missing")
	}
}

func TestInstall_Codex(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	agent := initflow.DetectedAgent{Name: "Codex", Detected: true, ConfigDir: codexDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(codexDir, "AGENTS.md"))
	if !strings.Contains(string(content), "Locksmith") {
		t.Error("AGENTS.md missing Locksmith section")
	}
}

func TestInstall_OpenCode(t *testing.T) {
	home := t.TempDir()
	openCodeDir := filepath.Join(home, ".config", "opencode")
	agent := initflow.DetectedAgent{Name: "OpenCode", Detected: true, ConfigDir: openCodeDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(openCodeDir, "instructions.md"))
	if !strings.Contains(string(content), "Locksmith") {
		t.Error("instructions.md missing Locksmith section")
	}
}

func TestInstall_Generic(t *testing.T) {
	home := t.TempDir()
	agent := initflow.DetectedAgent{Name: "SomeOtherAgent", Detected: true, ConfigDir: filepath.Join(home, ".someagent")}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(home, ".config", "locksmith", "agent-instructions.md"))
	if !strings.Contains(string(content), "Locksmith") {
		t.Error("agent-instructions.md missing Locksmith section")
	}
}

// --- Sandbox tests ---

func TestInstallSandboxPermissions_ClaudeCode_CreatesFreshSettings(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	if err := initflow.InstallSandboxPermissions(agent); err != nil {
		t.Fatalf("InstallSandboxPermissions() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json not valid JSON: %v", err)
	}
	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		t.Fatal("permissions key missing")
	}
	allow, _ := perms["allow"].([]interface{})
	if len(allow) == 0 {
		t.Fatal("permissions.allow is empty")
	}
	// Verify at least one locksmith entry is present.
	found := false
	for _, item := range allow {
		if s, ok := item.(string); ok && strings.Contains(s, "locksmith") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no locksmith permission found in allow list")
	}
}

func TestInstallSandboxPermissions_ClaudeCode_Idempotent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	initflow.InstallSandboxPermissions(agent)
	initflow.InstallSandboxPermissions(agent) // second call must not duplicate

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	perms, _ := settings["permissions"].(map[string]interface{})
	allow, _ := perms["allow"].([]interface{})

	seen := make(map[string]int)
	for _, item := range allow {
		if s, ok := item.(string); ok {
			seen[s]++
		}
	}
	for perm, count := range seen {
		if count > 1 {
			t.Errorf("permission %q appears %d times (expected 1)", perm, count)
		}
	}
}

func TestInstallSandboxPermissions_ClaudeCode_PreservesExistingSettings(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	// Pre-populate settings.json with an existing permission.
	existing := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []interface{}{"Bash(git status)"},
		},
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	if err := initflow.InstallSandboxPermissions(agent); err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ = os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	perms, _ := settings["permissions"].(map[string]interface{})
	allow, _ := perms["allow"].([]interface{})

	// Both original and new locksmith entries must be present.
	hasGit, hasLocksmith := false, false
	for _, item := range allow {
		s, _ := item.(string)
		if s == "Bash(git status)" {
			hasGit = true
		}
		if strings.Contains(s, "locksmith") {
			hasLocksmith = true
		}
	}
	if !hasGit {
		t.Error("original git permission was removed")
	}
	if !hasLocksmith {
		t.Error("locksmith permissions not added")
	}
}

func TestInstallSandboxPermissions_Codex(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	os.MkdirAll(codexDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Codex", Detected: true, ConfigDir: codexDir}
	if err := initflow.InstallSandboxPermissions(agent); err != nil {
		t.Fatalf("InstallSandboxPermissions() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(codexDir, "policy.yaml"))
	if err != nil {
		t.Fatalf("policy.yaml not created: %v", err)
	}
	if !strings.Contains(string(content), "locksmith") {
		t.Error("policy.yaml missing locksmith entries")
	}
}

func TestInstallSandboxPermissions_Codex_Idempotent(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	os.MkdirAll(codexDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Codex", Detected: true, ConfigDir: codexDir}
	initflow.InstallSandboxPermissions(agent)
	initflow.InstallSandboxPermissions(agent)

	content, _ := os.ReadFile(filepath.Join(codexDir, "policy.yaml"))
	if strings.Count(string(content), "# Locksmith permissions") > 1 {
		t.Error("policy.yaml header duplicated on second call")
	}
}

func TestInstallSandboxPermissions_UnknownAgent_NoOp(t *testing.T) {
	agent := initflow.DetectedAgent{Name: "Unknown", Detected: true, ConfigDir: "/tmp/nonexistent-agent"}
	// Must not return an error for unknown agents.
	if err := initflow.InstallSandboxPermissions(agent); err != nil {
		t.Errorf("InstallSandboxPermissions() unexpected error: %v", err)
	}
}
