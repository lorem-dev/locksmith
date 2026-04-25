package initflow_test

// Tests for the huh-based Prompter implementation. These exercise the TUI
// wrapper methods by injecting a pipe as the input reader (accessible mode).
// Accessible mode reads plain text from an io.Reader so no real TTY is needed.
//
// Input protocol (huh v1 accessible mode):
//   - Select:      Enter a 1-based integer, then newline.
//   - MultiSelect: Enter option numbers one at a time; enter 0 to confirm.
//   - Confirm:     Enter "y" (yes) or "n" (no), then newline.
//   - Input:       Enter the string, then newline.
//
// NOTE: huh's RunAccessible uses bufio.Scanner internally, which may read-ahead
// and exhaust the underlying reader. We wrap strings.NewReader in a
// singleByteReader to prevent this and make multi-form sequences work reliably.

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

// singleByteReader wraps an io.Reader to deliver at most one byte per Read call.
// This prevents bufio.Scanner from over-consuming the reader when multiple
// huh forms share the same io.Reader.
type singleByteReader struct{ r io.Reader }

func (s *singleByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return s.r.Read(p[:1])
}

// newHuhWithInput creates a HuhPrompter with accessible=true and the given
// string as stdin, discarding all output. Suitable for headless tests.
func newHuhWithInput(input string) initflow.Prompter {
	r := &singleByteReader{r: strings.NewReader(input)}
	return initflow.NewHuhPrompter(true, r, io.Discard)
}

func TestHuhPrompter_ConfigLocation_DefaultSelected(t *testing.T) {
	// Select option 1 → the default directory.
	p := newHuhWithInput("1\n")
	defaultDir := t.TempDir()
	got, err := p.ConfigLocation(defaultDir)
	if err != nil {
		t.Fatalf("ConfigLocation() error: %v", err)
	}
	if got != defaultDir {
		t.Errorf("ConfigLocation() = %q, want %q", got, defaultDir)
	}
}

func TestHuhPrompter_ConfigLocation_CustomPath(t *testing.T) {
	home := t.TempDir()
	customDir := fmt.Sprintf("%s/custom/config", home)
	// Select option 2 (custom), then type the custom path.
	input := fmt.Sprintf("2\n%s\n", customDir)
	p := newHuhWithInput(input)
	got, err := p.ConfigLocation(t.TempDir())
	if err != nil {
		t.Fatalf("ConfigLocation() error: %v", err)
	}
	if got != customDir {
		t.Errorf("ConfigLocation() = %q, want %q", got, customDir)
	}
}

func TestHuhPrompter_VaultSelection_SelectOne(t *testing.T) {
	// MultiSelect: pick option 1, then 0 to confirm.
	p := newHuhWithInput("1\n0\n")
	vaults := []initflow.DetectedVault{
		{Type: "gopass", Available: true, Detected: true},
		{Type: "keychain", Available: true, Detected: false},
	}
	got, err := p.VaultSelection(vaults)
	if err != nil {
		t.Fatalf("VaultSelection() error: %v", err)
	}
	if len(got) != 1 || got[0] != "gopass" {
		t.Errorf("VaultSelection() = %v, want [gopass]", got)
	}
}

func TestHuhPrompter_VaultSelection_SelectNone(t *testing.T) {
	// Immediately enter 0 → no vaults selected.
	p := newHuhWithInput("0\n")
	vaults := []initflow.DetectedVault{
		{Type: "gopass", Available: true},
	}
	got, err := p.VaultSelection(vaults)
	if err != nil {
		t.Fatalf("VaultSelection() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("VaultSelection() = %v, want []", got)
	}
}

func TestHuhPrompter_AgentSelection_AllDetected(t *testing.T) {
	// Option 1 = "All detected".
	p := newHuhWithInput("1\n")
	agents := []initflow.DetectedAgent{
		{Name: "Claude Code", Detected: true, ConfigDir: t.TempDir()},
		{Name: "Codex", Detected: true, ConfigDir: t.TempDir()},
	}
	got, err := p.AgentSelection(agents)
	if err != nil {
		t.Fatalf("AgentSelection() error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("AgentSelection() = %d agents, want 2", len(got))
	}
}

func TestHuhPrompter_AgentSelection_Skip(t *testing.T) {
	// Option 3 = "Skip".
	p := newHuhWithInput("3\n")
	agents := []initflow.DetectedAgent{
		{Name: "Claude Code", Detected: true},
	}
	got, err := p.AgentSelection(agents)
	if err != nil {
		t.Fatalf("AgentSelection() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("AgentSelection() = %v, want []", got)
	}
}

func TestHuhPrompter_AgentSelection_Manual(t *testing.T) {
	// Option 2 = "Select manually", then MultiSelect: pick item 1, confirm with 0.
	p := newHuhWithInput("2\n1\n0\n")
	agents := []initflow.DetectedAgent{
		{Name: "Claude Code", Detected: true},
		{Name: "Codex", Detected: true},
	}
	got, err := p.AgentSelection(agents)
	if err != nil {
		t.Fatalf("AgentSelection() error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("AgentSelection() = %d agents, want 1", len(got))
	}
	if len(got) > 0 && got[0].Name != "Claude Code" {
		t.Errorf("AgentSelection() got %q, want Claude Code", got[0].Name)
	}
}

func TestHuhPrompter_AgentSelection_NoDetectedAgents(t *testing.T) {
	// When no agents are detected, prompt is skipped entirely.
	p := newHuhWithInput("")
	agents := []initflow.DetectedAgent{
		{Name: "Claude Code", Detected: false},
	}
	got, err := p.AgentSelection(agents)
	if err != nil {
		t.Fatalf("AgentSelection() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("AgentSelection() = %v, want []", got)
	}
}

func TestHuhPrompter_Sandbox_Yes(t *testing.T) {
	p := newHuhWithInput("y\n")
	got, err := p.Sandbox()
	if err != nil {
		t.Fatalf("Sandbox() error: %v", err)
	}
	if !got {
		t.Error("Sandbox() = false, want true")
	}
}

func TestHuhPrompter_Sandbox_No(t *testing.T) {
	p := newHuhWithInput("n\n")
	got, err := p.Sandbox()
	if err != nil {
		t.Fatalf("Sandbox() error: %v", err)
	}
	if got {
		t.Error("Sandbox() = true, want false")
	}
}

func TestHuhPrompter_Summary_Confirmed(t *testing.T) {
	p := newHuhWithInput("y\n")
	result := &initflow.InitResult{
		ConfigPath:     "/tmp/test/config.yaml",
		SelectedVaults: []string{"gopass"},
	}
	ok, err := p.Summary(result)
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if !ok {
		t.Error("Summary() = false, want true")
	}
}

func TestHuhPrompter_Summary_Cancelled(t *testing.T) {
	p := newHuhWithInput("n\n")
	result := &initflow.InitResult{ConfigPath: "/tmp/test/config.yaml"}
	ok, err := p.Summary(result)
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if ok {
		t.Error("Summary() = true, want false")
	}
}

func TestHuhPrompter_Summary_WithAgents(t *testing.T) {
	// Exercises the agentNames loop in Summary.
	p := newHuhWithInput("y\n")
	result := &initflow.InitResult{
		ConfigPath:     "/tmp/test/config.yaml",
		SelectedVaults: []string{"gopass"},
		SelectedAgents: []initflow.DetectedAgent{
			{Name: "Claude Code", Detected: true},
		},
		SandboxEnabled: true,
	}
	ok, err := p.Summary(result)
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if !ok {
		t.Error("Summary() = false, want true")
	}
}

func TestHuhPrompter_VaultSelection_Unavailable(t *testing.T) {
	// Exercises the "not available on this platform" label branch.
	p := newHuhWithInput("0\n")
	vaults := []initflow.DetectedVault{
		{Type: "keychain", Available: false, Detected: false},
	}
	got, err := p.VaultSelection(vaults)
	if err != nil {
		t.Fatalf("VaultSelection() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("VaultSelection() = %v, want []", got)
	}
}

func TestHuhPrompter_GPGPinentry_Yes(t *testing.T) {
	p := newHuhWithInput("y\n")
	got, err := p.GPGPinentry("")
	if err != nil {
		t.Fatalf("GPGPinentry() error: %v", err)
	}
	if !got {
		t.Error("GPGPinentry() = false, want true")
	}
}

func TestHuhPrompter_GPGPinentry_No(t *testing.T) {
	p := newHuhWithInput("n\n")
	got, err := p.GPGPinentry("")
	if err != nil {
		t.Fatalf("GPGPinentry() error: %v", err)
	}
	if got {
		t.Error("GPGPinentry() = true, want false")
	}
}

func TestHuhPrompter_GPGPinentry_WithExisting(t *testing.T) {
	// When an existing pinentry is set, the description changes but behaviour is the same.
	p := newHuhWithInput("y\n")
	got, err := p.GPGPinentry("/usr/bin/pinentry-mac")
	if err != nil {
		t.Fatalf("GPGPinentry() error: %v", err)
	}
	if !got {
		t.Error("GPGPinentry() = false, want true")
	}
}

func TestHuhPrompter_ExistingConfig_ValidContinue(t *testing.T) {
	// Option 1 = ActionContinue; valid config so description says "The existing config is valid."
	p := newHuhWithInput("1\n")
	got, err := p.ExistingConfig("/home/user/.config/locksmith/config.yaml", nil)
	if err != nil {
		t.Fatalf("ExistingConfig() error: %v", err)
	}
	if got != initflow.ActionContinue {
		t.Errorf("ExistingConfig() = %v, want ActionContinue", got)
	}
}

func TestHuhPrompter_ExistingConfig_ValidOverwrite(t *testing.T) {
	// Option 2 = ActionOverwrite.
	p := newHuhWithInput("2\n")
	got, err := p.ExistingConfig("/home/user/.config/locksmith/config.yaml", nil)
	if err != nil {
		t.Fatalf("ExistingConfig() error: %v", err)
	}
	if got != initflow.ActionOverwrite {
		t.Errorf("ExistingConfig() = %v, want ActionOverwrite", got)
	}
}

func TestHuhPrompter_ExistingConfig_ValidExit(t *testing.T) {
	// Option 3 = ActionExit.
	p := newHuhWithInput("3\n")
	got, err := p.ExistingConfig("/home/user/.config/locksmith/config.yaml", nil)
	if err != nil {
		t.Fatalf("ExistingConfig() error: %v", err)
	}
	if got != initflow.ActionExit {
		t.Errorf("ExistingConfig() = %v, want ActionExit", got)
	}
}

func TestHuhPrompter_ExistingConfig_InvalidContinue(t *testing.T) {
	// Invalid config - option 1 = "Continue with invalid config (not recommended)".
	p := newHuhWithInput("1\n")
	got, err := p.ExistingConfig("/home/user/.config/locksmith/config.yaml", fmt.Errorf("missing required field"))
	if err != nil {
		t.Fatalf("ExistingConfig() error: %v", err)
	}
	if got != initflow.ActionContinue {
		t.Errorf("ExistingConfig() = %v, want ActionContinue", got)
	}
}

func TestHuhPrompter_ExistingConfig_InvalidOverwrite(t *testing.T) {
	// Invalid config - option 2 = ActionOverwrite.
	p := newHuhWithInput("2\n")
	got, err := p.ExistingConfig("/home/user/.config/locksmith/config.yaml", fmt.Errorf("parse error"))
	if err != nil {
		t.Fatalf("ExistingConfig() error: %v", err)
	}
	if got != initflow.ActionOverwrite {
		t.Errorf("ExistingConfig() = %v, want ActionOverwrite", got)
	}
}

func TestHuhPrompter_ShellHook_Yes(t *testing.T) {
	p := newHuhWithInput("y\n")
	got, err := p.ShellHook("/home/user/.zshrc")
	if err != nil {
		t.Fatalf("ShellHook() error: %v", err)
	}
	if !got {
		t.Error("ShellHook() = false, want true")
	}
}

func TestHuhPrompter_ShellHook_No(t *testing.T) {
	p := newHuhWithInput("n\n")
	got, err := p.ShellHook("/home/user/.zshrc")
	if err != nil {
		t.Fatalf("ShellHook() error: %v", err)
	}
	if got {
		t.Error("ShellHook() = true, want false")
	}
}

func TestHuhPrompter_ClaudeHook_Yes(t *testing.T) {
	p := newHuhWithInput("y\n")
	got, err := p.ClaudeHook("/home/user/.claude/settings.json")
	if err != nil {
		t.Fatalf("ClaudeHook() error: %v", err)
	}
	if !got {
		t.Error("ClaudeHook() = false, want true")
	}
}

func TestHuhPrompter_ClaudeHook_No(t *testing.T) {
	p := newHuhWithInput("n\n")
	got, err := p.ClaudeHook("/home/user/.claude/settings.json")
	if err != nil {
		t.Fatalf("ClaudeHook() error: %v", err)
	}
	if got {
		t.Error("ClaudeHook() = true, want false")
	}
}
