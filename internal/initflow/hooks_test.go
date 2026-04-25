package initflow_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func makeHookInstaller(t *testing.T) (*initflow.ClaudeHookInstaller, string) {
	t.Helper()
	home := t.TempDir()
	lsDir := filepath.Join(home, ".config", "locksmith")
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(lsDir, 0o755)
	os.MkdirAll(claudeDir, 0o755)
	return initflow.NewClaudeHookInstaller(lsDir, claudeDir), home
}

func TestClaudeHookInstaller_IsInstalled_FalseWhenNoSettings(t *testing.T) {
	installer, _ := makeHookInstaller(t)
	if installer.IsInstalled() {
		t.Error("IsInstalled() = true, want false when settings.json absent")
	}
}

func TestClaudeHookInstaller_IsInstalled_FalseWhenHooksAbsent(t *testing.T) {
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")
	existing := map[string]any{
		"theme": "dark",
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	if installer.IsInstalled() {
		t.Error("IsInstalled() = true, want false when no hooks key")
	}
}

func TestClaudeHookInstaller_IsInstalled_TrueWhenHookPresent(t *testing.T) {
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")
	lsDir := filepath.Join(home, ".config", "locksmith")
	hookCmd := filepath.Join(lsDir, "agent-hook.sh")
	settings := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": hookCmd},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(settings)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	if !installer.IsInstalled() {
		t.Error("IsInstalled() = false, want true when hook command present")
	}
}

func TestClaudeHookInstaller_Install_WritesHookScript(t *testing.T) {
	installer, home := makeHookInstaller(t)
	lsDir := filepath.Join(home, ".config", "locksmith")

	if err := installer.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	scriptPath := filepath.Join(lsDir, "agent-hook.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("hook script not created at %s: %v", scriptPath, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("hook script is not executable")
	}
	content, _ := os.ReadFile(scriptPath)
	if !strings.Contains(string(content), "locksmith session ensure") {
		t.Error("hook script missing expected locksmith command")
	}
}

func TestClaudeHookInstaller_Install_CreatesSettingsJson(t *testing.T) {
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")
	lsDir := filepath.Join(home, ".config", "locksmith")

	if err := installer.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json is invalid JSON: %v", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks key missing from settings.json")
	}
	ups, _ := hooks["UserPromptSubmit"].([]any)
	if len(ups) == 0 {
		t.Fatal("UserPromptSubmit array is empty")
	}
	hookCmd := filepath.Join(lsDir, "agent-hook.sh")
	found := false
	for _, entry := range ups {
		em, _ := entry.(map[string]any)
		if em == nil {
			continue
		}
		subhooks, _ := em["hooks"].([]any)
		for _, sh := range subhooks {
			shm, _ := sh.(map[string]any)
			if shm == nil {
				continue
			}
			if cmd, _ := shm["command"].(string); cmd == hookCmd {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("hook command %q not found in settings.json", hookCmd)
	}
}

func TestClaudeHookInstaller_Install_MergesExistingSettings(t *testing.T) {
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")

	existing := map[string]any{
		"theme": "dark",
		"permissions": map[string]any{
			"allow": []any{"Bash(git status)"},
		},
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	if err := installer.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, _ = os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]any
	json.Unmarshal(data, &settings) //nolint:errcheck

	if settings["theme"] != "dark" {
		t.Error("existing theme setting was lost")
	}
	perms, _ := settings["permissions"].(map[string]any)
	allow, _ := perms["allow"].([]any)
	hasGit := false
	for _, item := range allow {
		if s, _ := item.(string); s == "Bash(git status)" {
			hasGit = true
		}
	}
	if !hasGit {
		t.Error("existing git permission was lost")
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Error("hooks key missing - Install did not merge")
	}
}

func TestClaudeHookInstaller_Install_Idempotent(t *testing.T) {
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")
	lsDir := filepath.Join(home, ".config", "locksmith")
	hookCmd := filepath.Join(lsDir, "agent-hook.sh")

	installer.Install() //nolint:errcheck
	installer.Install() //nolint:errcheck

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]any
	json.Unmarshal(data, &settings) //nolint:errcheck
	hooks, _ := settings["hooks"].(map[string]any)
	ups, _ := hooks["UserPromptSubmit"].([]any)

	count := 0
	for _, entry := range ups {
		em, _ := entry.(map[string]any)
		if em == nil {
			continue
		}
		subhooks, _ := em["hooks"].([]any)
		for _, sh := range subhooks {
			shm, _ := sh.(map[string]any)
			if shm == nil {
				continue
			}
			if cmd, _ := shm["command"].(string); cmd == hookCmd {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("hook command appears %d times, want 1", count)
	}
}

func TestClaudeHookInstaller_Install_IsInstalledAfterInstall(t *testing.T) {
	installer, _ := makeHookInstaller(t)

	if err := installer.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if !installer.IsInstalled() {
		t.Error("IsInstalled() = false after Install()")
	}
}

func TestClaudeHookInstaller_IsInstalled_MalformedEntries(t *testing.T) {
	// Non-map entries in UserPromptSubmit and hooks arrays must not panic
	// and must return false when the locksmith command is absent.
	installer, home := makeHookInstaller(t)
	claudeDir := filepath.Join(home, ".claude")
	settings := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				"not-a-map", // non-map entry at the top level
				map[string]any{
					"matcher": "",
					"hooks": []any{
						"also-not-a-map", // non-map entry in inner hooks
					},
				},
			},
		},
	}
	data, _ := json.Marshal(settings)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	if installer.IsInstalled() {
		t.Error("IsInstalled() = true for malformed entries without locksmith command")
	}
}

func TestClaudeHookInstaller_Install_WriteScriptError(t *testing.T) {
	// If the locksmith config path exists as a regular file, MkdirAll fails
	// and Install should return a wrapped error.
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.MkdirAll(filepath.Join(home, ".config"), 0o755)

	// Create a file where MkdirAll would expect to create a directory.
	lsConfigPath := filepath.Join(home, ".config", "locksmith")
	os.WriteFile(lsConfigPath, []byte("not a directory"), 0o644)

	installer := initflow.NewClaudeHookInstaller(lsConfigPath, claudeDir)
	if err := installer.Install(); err == nil {
		t.Error("Install() should fail when locksmith config dir path is a file")
	}
}
