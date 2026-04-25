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
	if err := os.MkdirAll( //nolint:gosec // G301: 0755 is standard for user config dirs
		h.locksmithConfigDir,
		0o755,
	); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	scriptContent, err := templates.ReadFile("templates/claude_hook.sh.tmpl")
	if err != nil {
		return fmt.Errorf("reading hook template: %w", err)
	}
	if err := os.WriteFile( //nolint:gosec // G306: hook script must be executable
		h.hookCmd(), scriptContent, 0o755,
	); err != nil {
		return fmt.Errorf("writing hook script: %w", err)
	}
	return nil
}

func (h *ClaudeHookInstaller) mergeSettings() error {
	if err := os.MkdirAll( //nolint:gosec // G301: 0755 is standard for user home dirs
		h.claudeConfigDir,
		0o755,
	); err != nil {
		return fmt.Errorf("creating Claude config dir: %w", err)
	}

	var settings map[string]any
	if data, err := os.ReadFile(h.settingsPath()); err == nil {
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			settings = nil // treat malformed JSON as absent
		}
	}
	if settings == nil {
		settings = make(map[string]any)
	}

	if h.findHookCmd(settings, h.hookCmd()) {
		return nil // already present - idempotent
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
	}
	var ups []any
	if raw, ok := hooks["UserPromptSubmit"].([]any); ok {
		ups = raw
	}
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
	if err := os.WriteFile( //nolint:gosec // G306: settings.json is user-readable config
		h.settingsPath(), out, 0o644,
	); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}

// findHookCmd returns true if hookCmd appears as a command value anywhere in
// the UserPromptSubmit hook entries of settings.
func (h *ClaudeHookInstaller) findHookCmd(settings map[string]any, hookCmd string) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	var ups []any
	if raw, ok := hooks["UserPromptSubmit"].([]any); ok {
		ups = raw
	}
	for _, entry := range ups {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		var subhooks []any
		if raw, ok := em["hooks"].([]any); ok {
			subhooks = raw
		}
		for _, sh := range subhooks {
			shm, ok := sh.(map[string]any)
			if !ok {
				continue
			}
			if cmd, cmdOK := shm["command"].(string); cmdOK && cmd == hookCmd {
				return true
			}
		}
	}
	return false
}
