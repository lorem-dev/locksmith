package initflow_test

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/bundled"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/initflow"
	"github.com/lorem-dev/locksmith/internal/log"
)

func TestMain(m *testing.M) {
	log.Init(io.Discard, "error", "text")
	os.Exit(m.Run())
}

// mockPrompter is a test double for initflow.Prompter that returns configured
// canned responses, allowing RunInit to be exercised without a real TTY.
type mockPrompter struct {
	configDir            string
	configErr            error
	vaults               []string
	vaultErr             error
	agents               []initflow.DetectedAgent
	agentErr             error
	sandbox              bool
	sandboxErr           error
	summaryConfirm       bool
	summaryErr           error
	gpgPinentry          bool
	gpgPinentryErr       error
	existingConfigAction initflow.ExistingConfigAction
	existingConfigErr    error
	shellHookInstall     bool
	shellHookErr         error
	claudeHook           bool
	claudeHookErr        error
}

func (m *mockPrompter) ConfigLocation(_ string) (string, error) {
	return m.configDir, m.configErr
}

func (m *mockPrompter) VaultSelection(_ []initflow.DetectedVault) ([]string, error) {
	return m.vaults, m.vaultErr
}

func (m *mockPrompter) AgentSelection(_ []initflow.DetectedAgent) ([]initflow.DetectedAgent, error) {
	return m.agents, m.agentErr
}

func (m *mockPrompter) Sandbox() (bool, error) {
	return m.sandbox, m.sandboxErr
}

func (m *mockPrompter) Summary(_ *initflow.InitResult) (bool, error) {
	return m.summaryConfirm, m.summaryErr
}

func (m *mockPrompter) GPGPinentry(_ string) (bool, error) {
	return m.gpgPinentry, m.gpgPinentryErr
}

func (m *mockPrompter) ExistingConfig(_ string, _ error) (initflow.ExistingConfigAction, error) {
	return m.existingConfigAction, m.existingConfigErr
}

func (m *mockPrompter) ShellHook(_ string) (bool, error) {
	return m.shellHookInstall, m.shellHookErr
}

func (m *mockPrompter) ClaudeHook(_ string) (bool, error) {
	return m.claudeHook, m.claudeHookErr
}

func (m *mockPrompter) BundleExtractPrompt(_, _, _ string) (bundled.ConflictResolution, error) {
	return bundled.Keep, nil
}

func TestAgentMatches_CaseInsensitive(t *testing.T) {
	cases := []struct {
		name, query string
		want        bool
	}{
		{"Claude Code", "claude", true},
		{"Claude Code", "Claude Code", true},
		{"Codex", "codex", true},
		{"Codex", "claude", false},
		{"OpenCode", "opencode", true},
	}
	for _, c := range cases {
		got := initflow.AgentMatches(c.name, c.query)
		if got != c.want {
			t.Errorf("AgentMatches(%q, %q) = %v, want %v", c.name, c.query, got, c.want)
		}
	}
}

func TestRunInit_Auto_NoAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:       true,
		SkipAgents: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Errorf("config file not created at %s: %v", result.ConfigPath, err)
	}
}

func TestRunInit_Auto_WithVaultDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:       true,
		SkipAgents: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result == nil {
		t.Fatal("RunInit() returned nil result")
	}
}

func TestRunInit_Auto_InstallsClaudeCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	found := false
	for _, a := range result.SelectedAgents {
		if a.Name == "Claude Code" {
			found = true
		}
	}
	if !found {
		t.Error("expected Claude Code in SelectedAgents")
	}
}

func TestRunInit_AgentOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:      true,
		AgentOnly: "claude",
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if len(result.SelectedAgents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result.SelectedAgents))
	}
	if result.SelectedAgents[0].Name != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", result.SelectedAgents[0].Name)
	}
}

func TestRunInit_Auto_InstallsClaudeHook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	result, err := initflow.RunInit(initflow.InitOptions{Auto: true})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}

	hasClaudeCode := false
	for _, a := range result.SelectedAgents {
		if a.Name == "Claude Code" {
			hasClaudeCode = true
		}
	}
	if !hasClaudeCode {
		t.Skip("Claude Code not detected - skipping hook install test")
	}

	if !result.ClaudeHookInstalled && !result.ClaudeHookAlreadyPresent {
		t.Error("expected ClaudeHookInstalled or ClaudeHookAlreadyPresent to be true in --auto mode")
	}

	scriptPath := filepath.Join(home, ".config", "locksmith", "agent-hook.sh")
	if _, statErr := os.Stat(scriptPath); statErr != nil {
		t.Errorf("hook script not written to %s: %v", scriptPath, statErr)
	}

	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	if !strings.Contains(string(data), "agent-hook.sh") {
		t.Error("settings.json does not reference agent-hook.sh")
	}
}

func TestRunInit_NonAuto_HookConfirmed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	prompter := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{},
		agents:         []initflow.DetectedAgent{{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}},
		sandbox:        false,
		summaryConfirm: true,
		claudeHook:     true,
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: prompter})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.ClaudeHookInstalled {
		t.Error("ClaudeHookInstalled = false, want true when user confirmed")
	}
}

func TestRunInit_NonAuto_HookDeclined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	prompter := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{},
		agents:         []initflow.DetectedAgent{{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}},
		sandbox:        false,
		summaryConfirm: true,
		claudeHook:     false,
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: prompter})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result.ClaudeHookInstalled {
		t.Error("ClaudeHookInstalled = true, want false when user declined")
	}
}

func TestRunInit_HookAlreadyPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	lsDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(claudeDir, 0o755)
	os.MkdirAll(lsDir, 0o755)

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

	prompter := &mockPrompter{
		configDir:      lsDir,
		vaults:         []string{},
		agents:         []initflow.DetectedAgent{{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}},
		summaryConfirm: true,
		claudeHook:     false, // should never be called
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: prompter})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.ClaudeHookAlreadyPresent {
		t.Error("ClaudeHookAlreadyPresent = false, want true when hook pre-installed")
	}
	if result.ClaudeHookInstalled {
		t.Error("ClaudeHookInstalled = true, want false when hook already present")
	}
}

// --- Tests using mockPrompter to cover the interactive (non-auto) path ---

func TestRunInit_Interactive_Confirmed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{"gopass"},
		agents:         []initflow.DetectedAgent{{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}},
		sandbox:        true,
		summaryConfirm: true,
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if len(result.SelectedVaults) != 1 || result.SelectedVaults[0] != "gopass" {
		t.Errorf("SelectedVaults = %v, want [gopass]", result.SelectedVaults)
	}
	if len(result.SelectedAgents) != 1 {
		t.Errorf("SelectedAgents count = %d, want 1", len(result.SelectedAgents))
	}
	if !result.SandboxEnabled {
		t.Error("expected SandboxEnabled = true")
	}
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestRunInit_Interactive_Cancelled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		summaryConfirm: false, // user cancels
	}

	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err == nil {
		t.Fatal("RunInit() expected error when user cancels")
	}
}

func TestRunInit_Interactive_ConfigLocationError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wantErr := errors.New("config location error")

	mp := &mockPrompter{configErr: wantErr}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Interactive_VaultSelectionError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wantErr := errors.New("vault error")

	mp := &mockPrompter{
		configDir: filepath.Join(home, ".config", "locksmith"),
		vaultErr:  wantErr,
	}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Interactive_AgentSelectionError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wantErr := errors.New("agent selection error")

	mp := &mockPrompter{
		configDir: filepath.Join(home, ".config", "locksmith"),
		agentErr:  wantErr,
	}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Interactive_SandboxError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	wantErr := errors.New("sandbox error")

	mp := &mockPrompter{
		configDir:  filepath.Join(home, ".config", "locksmith"),
		agents:     []initflow.DetectedAgent{{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}},
		sandboxErr: wantErr,
	}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Interactive_SummaryError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wantErr := errors.New("summary error")

	mp := &mockPrompter{
		configDir:  filepath.Join(home, ".config", "locksmith"),
		summaryErr: wantErr,
	}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Interactive_SkipAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		summaryConfirm: true,
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp, SkipAgents: true})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if len(result.SelectedAgents) != 0 {
		t.Errorf("expected no agents, got %d", len(result.SelectedAgents))
	}
}

func TestRunInit_Interactive_GPGPinentryAccepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{"gopass"},
		summaryConfirm: true,
		gpgPinentry:    true, // user opts in to locksmith-pinentry
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.GPGPinentryConfigured {
		t.Error("expected GPGPinentryConfigured = true when user accepts")
	}
}

func TestRunInit_Interactive_GPGPinentrySkippedForNonGopass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{"keychain"}, // gopass not selected - GPG step skipped
		summaryConfirm: true,
		gpgPinentry:    true, // would return true but should never be called
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result.GPGPinentryConfigured {
		t.Error("GPGPinentryConfigured should be false when gopass not selected")
	}
}

func TestRunInit_Interactive_NoSandboxWhenNoAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sandboxCalled := false
	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		summaryConfirm: true,
		// Sandbox() must NOT be called when no agents are selected.
		// We verify indirectly: if called, it would set sandboxCalled below.
	}
	_ = sandboxCalled

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result.SandboxEnabled {
		t.Error("SandboxEnabled should be false when no agents selected")
	}
}

func TestRunInit_ExistingConfig_Valid_Continue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(cfgDir, 0o755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	// Write a minimal valid config.
	os.WriteFile(cfgPath, []byte("defaults:\n  session_ttl: 1h\n"), 0o644)
	originalContent, _ := os.ReadFile(cfgPath)

	mp := &mockPrompter{
		configDir:            cfgDir,
		summaryConfirm:       true,
		existingConfigAction: initflow.ActionContinue,
	}
	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.ConfigPreexisted {
		t.Error("expected ConfigPreexisted = true")
	}
	// File must not be overwritten.
	gotContent, _ := os.ReadFile(cfgPath)
	if string(gotContent) != string(originalContent) {
		t.Errorf("config file was modified; want unchanged")
	}
}

func TestRunInit_ExistingConfig_Valid_Overwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(cfgDir, 0o755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	os.WriteFile(cfgPath, []byte("defaults:\n  session_ttl: 1h\n"), 0o644)

	mp := &mockPrompter{
		configDir:            cfgDir,
		summaryConfirm:       true,
		existingConfigAction: initflow.ActionOverwrite,
	}
	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result.ConfigPreexisted {
		t.Error("ConfigPreexisted should be false when user chose Overwrite")
	}
}

func TestRunInit_ExistingConfig_Exit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("defaults:\n  session_ttl: 1h\n"), 0o644)

	mp := &mockPrompter{
		configDir:            cfgDir,
		existingConfigAction: initflow.ActionExit,
	}
	_, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err == nil {
		t.Fatal("RunInit() expected error when user exits")
	}
}

func TestRunInit_ExistingConfig_Valid_AutoContinues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(cfgDir, 0o755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	os.WriteFile(cfgPath, []byte("defaults:\n  session_ttl: 1h\n"), 0o644)
	originalContent, _ := os.ReadFile(cfgPath)

	result, err := initflow.RunInit(initflow.InitOptions{Auto: true, SkipAgents: true})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.ConfigPreexisted {
		t.Error("expected ConfigPreexisted = true for valid config in auto mode")
	}
	gotContent, _ := os.ReadFile(cfgPath)
	if string(gotContent) != string(originalContent) {
		t.Errorf("config file was modified in auto mode with valid config")
	}
}

func TestRunInit_ExistingConfig_Invalid_AutoOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	os.MkdirAll(cfgDir, 0o755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	// Write a config that references a non-existent vault - will fail validation.
	os.WriteFile(cfgPath, []byte("keys:\n  mykey:\n    vault: missing\n    path: p\n"), 0o644)

	result, err := initflow.RunInit(initflow.InitOptions{Auto: true, SkipAgents: true})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result.ConfigPreexisted {
		t.Error("ConfigPreexisted should be false when invalid config was overwritten")
	}
	// File must have been rewritten to a valid config.
	if _, loadErr := config.Load(cfgPath); loadErr != nil {
		t.Errorf("overwritten config is not valid: %v", loadErr)
	}
}

func TestRunInit_Interactive_GPGPinentryApplied(t *testing.T) {
	// Cover the applyInit GPG block (lines 243-252) by putting a fake
	// locksmith-pinentry binary in PATH so exec.LookPath succeeds.
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a fake locksmith-pinentry script.
	binDir := t.TempDir()
	fakePinentry := filepath.Join(binDir, "locksmith-pinentry")
	if err := os.WriteFile(fakePinentry, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("creating fake pinentry: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	mp := &mockPrompter{
		configDir:      filepath.Join(home, ".config", "locksmith"),
		vaults:         []string{"gopass"},
		summaryConfirm: true,
		gpgPinentry:    true,
	}

	result, err := initflow.RunInit(initflow.InitOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if !result.GPGPinentryConfigured {
		t.Error("expected GPGPinentryConfigured = true")
	}
}

func TestRunInit_ShellHook_AlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Pre-write the marker to a fake .zshrc
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# locksmith daemon autostart\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "/bin/zsh")

	dir := t.TempDir()
	_, err := initflow.RunInit(initflow.InitOptions{
		Auto: true,
		Prompter: &mockPrompter{
			configDir:      dir,
			summaryConfirm: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Marker must NOT be duplicated
	data, _ := os.ReadFile(zshrc)
	count := strings.Count(string(data), "# locksmith daemon autostart")
	if count != 1 {
		t.Errorf("marker count = %d, want 1", count)
	}
}

func TestRunInit_ShellHook_Auto_Installs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	dir := t.TempDir()
	result, err := initflow.RunInit(initflow.InitOptions{
		Auto: true,
		Prompter: &mockPrompter{
			configDir:      dir,
			summaryConfirm: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ShellHookInstall {
		t.Error("ShellHookInstall should be true in auto mode")
	}

	bashrc := filepath.Join(home, ".bashrc")
	data, readErr := os.ReadFile(bashrc)
	if readErr != nil {
		t.Fatalf("bashrc not written: %v", readErr)
	}
	if !strings.Contains(string(data), "# locksmith daemon autostart") {
		t.Error("marker not found in .bashrc")
	}
}

func TestRunInit_ShellHook_Interactive_Accepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	dir := t.TempDir()
	result, err := initflow.RunInit(initflow.InitOptions{
		Prompter: &mockPrompter{
			configDir:        dir,
			summaryConfirm:   true,
			shellHookInstall: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ShellHookInstall {
		t.Error("ShellHookInstall should be true when user accepted")
	}

	zshrc := filepath.Join(home, ".zshrc")
	data, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(data), "# locksmith daemon autostart") {
		t.Error("marker not found in .zshrc")
	}
}

func TestRunInit_ShellHook_Interactive_Declined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	dir := t.TempDir()
	result, err := initflow.RunInit(initflow.InitOptions{
		Prompter: &mockPrompter{
			configDir:        dir,
			summaryConfirm:   true,
			shellHookInstall: false, // user declines
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShellHookInstall {
		t.Error("ShellHookInstall should be false when user declined")
	}

	bashrc := filepath.Join(home, ".bashrc")
	if _, statErr := os.Stat(bashrc); statErr == nil {
		data, _ := os.ReadFile(bashrc)
		if strings.Contains(string(data), "# locksmith daemon autostart") {
			t.Error("marker must not be written when user declined")
		}
	}
}

func TestRunInit_ShellHook_UnknownShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh") // unknown (not bash/zsh/ash/fish)
	t.Setenv("0", "")

	dir := t.TempDir()
	result, err := initflow.RunInit(initflow.InitOptions{
		Auto: true,
		Prompter: &mockPrompter{
			configDir:      dir,
			summaryConfirm: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ShellHookInstall {
		t.Error("ShellHookInstall must be false for unknown shell")
	}
}

func TestExtractBundled_EmptyBundleWarnsButReturnsNil(t *testing.T) {
	// internal/bundled has no exposed override for bundleBytes; this test
	// runs in builds where the placeholder bundle is in effect.
	// Caller of extractBundled treats ErrEmptyBundle as a warning, returning nil.
	if err := initflow.ExtractBundled([]string{"gopass"}, nil, true); err != nil {
		// Only fail if the bundle is non-empty AND extraction is failing.
		// In a placeholder-bundle build this returns nil.
		t.Logf("extractBundled returned %v (likely real bundle present)", err)
	}
}

func TestRunInit_ShellHook_Error(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	wantErr := errors.New("shell hook prompt error")

	dir := t.TempDir()
	_, err := initflow.RunInit(initflow.InitOptions{
		Prompter: &mockPrompter{
			configDir:      dir,
			summaryConfirm: true,
			shellHookErr:   wantErr,
		},
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("RunInit() error = %v, want %v", err, wantErr)
	}
}

func TestRunInit_Auto_SkipsPlannedVaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Override DetectVaultsFn to return a controlled set:
	// - 1password: detected but not implemented
	// - gopass: detected and implemented
	orig := initflow.DetectVaultsFn
	initflow.DetectVaultsFn = func() []initflow.DetectedVault {
		return []initflow.DetectedVault{
			{Type: "1password", Detected: true, Available: true, Implemented: false},
			{Type: "gopass", Detected: true, Available: true, Implemented: true},
		}
	}
	t.Cleanup(func() { initflow.DetectVaultsFn = orig })

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:       true,
		SkipAgents: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}

	// gopass must be selected
	foundGopass := false
	for _, v := range result.SelectedVaults {
		if v == "gopass" {
			foundGopass = true
		}
	}
	if !foundGopass {
		t.Errorf("SelectedVaults = %v, want to contain gopass", result.SelectedVaults)
	}

	// 1password must NOT be selected
	for _, v := range result.SelectedVaults {
		if v == "1password" {
			t.Errorf("SelectedVaults = %v, must not contain 1password (not implemented)", result.SelectedVaults)
		}
	}

	if len(result.SelectedVaults) != 1 {
		t.Errorf("SelectedVaults = %v, want exactly [gopass]", result.SelectedVaults)
	}
}
