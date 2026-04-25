package initflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeHookInstaller installs the Locksmith UserPromptSubmit hook into
// the global Claude Code settings file (~/.claude/settings.json).
type ClaudeHookInstaller struct {
	locksmithConfigDir string // ~/.config/locksmith
	claudeConfigDir    string // ~/.claude
}

// NewClaudeHookInstaller creates a ClaudeHookInstaller for the given directories.
func NewClaudeHookInstaller(locksmithConfigDir, claudeConfigDir string) *ClaudeHookInstaller {
	return &ClaudeHookInstaller{
		locksmithConfigDir: locksmithConfigDir,
		claudeConfigDir:    claudeConfigDir,
	}
}

func (h *ClaudeHookInstaller) hookCmd() string {
	return filepath.Join(h.locksmithConfigDir, "agent-hook.sh")
}

func (h *ClaudeHookInstaller) settingsPath() string {
	return filepath.Join(h.claudeConfigDir, "settings.json")
}

// IsInstalled reports whether the hook command is already registered in
// ~/.claude/settings.json.
func (h *ClaudeHookInstaller) IsInstalled() bool {
	data, err := os.ReadFile(h.settingsPath())
	if err != nil {
		return false
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	return h.findHookCmd(settings, h.hookCmd())
}

// Install writes the hook script to ~/.config/locksmith/agent-hook.sh and
// merges the UserPromptSubmit entry into ~/.claude/settings.json.
// It is idempotent: calling Install twice produces the same result as calling it once.
func (h *ClaudeHookInstaller) Install() error {
	if err := h.writeScript(); err != nil {
		return fmt.Errorf("writing hook script: %w", err)
	}
	return h.mergeSettings()
}

func (h *ClaudeHookInstaller) writeScript() error {
	if err := os.MkdirAll(h.locksmithConfigDir, 0o755); err != nil {
		return err
	}
	scriptContent, err := templates.ReadFile("templates/claude_hook.sh.tmpl")
	if err != nil {
		return err
	}
	return os.WriteFile(h.hookCmd(), scriptContent, 0o755)
}

func (h *ClaudeHookInstaller) mergeSettings() error {
	if err := os.MkdirAll(h.claudeConfigDir, 0o755); err != nil {
		return err
	}

	var settings map[string]any
	if data, err := os.ReadFile(h.settingsPath()); err == nil {
		json.Unmarshal(data, &settings) //nolint:errcheck
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	if h.findHookCmd(settings, h.hookCmd()) {
		return nil // already present - idempotent
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}
	ups, _ := hooks["UserPromptSubmit"].([]any)
	ups = append(ups, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": h.hookCmd(),
			},
		},
	})
	hooks["UserPromptSubmit"] = ups
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	return os.WriteFile(h.settingsPath(), out, 0o644)
}

// findHookCmd returns true if hookCmd appears as a command value anywhere in
// the UserPromptSubmit hook entries of settings.
func (h *ClaudeHookInstaller) findHookCmd(settings map[string]any, hookCmd string) bool {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return false
	}
	ups, _ := hooks["UserPromptSubmit"].([]any)
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
				return true
			}
		}
	}
	return false
}
