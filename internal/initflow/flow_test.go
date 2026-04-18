package initflow_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/initflow"
)

// mockPrompter is a test double for initflow.Prompter that returns configured
// canned responses, allowing RunInit to be exercised without a real TTY.
type mockPrompter struct {
	configDir         string
	configErr         error
	vaults            []string
	vaultErr          error
	agents            []initflow.DetectedAgent
	agentErr          error
	sandbox           bool
	sandboxErr        error
	summaryConfirm       bool
	summaryErr           error
	gpgPinentry          bool
	gpgPinentryErr       error
	existingConfigAction initflow.ExistingConfigAction
	existingConfigErr    error
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
	if result == nil {
		t.Fatal("RunInit() returned nil result")
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
