# Init Existing-Config Handling + `config pinentry` Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `locksmith init` finds an existing config it validates it and offers three options (continue/overwrite/exit); new `locksmith config pinentry` command configures locksmith-pinentry independently of init.

**Architecture:** Feature 1 adds `ExistingConfigAction` + `Prompter.ExistingConfig` to `internal/initflow/flow.go`; `RunInit` checks for an existing file before the wizard and sets `result.ConfigPreexisted` to skip the config write in `applyInit`. Feature 2 adds `RunConfigPinentry` in a new `internal/initflow/config_pinentry.go` file that reuses existing `ReadExistingPinentry`/`ApplyGPGPinentry`; the CLI wires it as `locksmith config pinentry`.

**Tech Stack:** Go 1.25, `charmbracelet/huh` for TUI forms, `cobra` for CLI, `internal/config.Load` for validation.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/initflow/flow.go` | Modify | `ExistingConfigAction` type; `ExistingConfig` on `Prompter` + `huhPrompter`; `ConfigPreexisted` on `InitResult`; existing-config check in `RunInit`; skip-write in `applyInit` |
| `internal/initflow/flow_test.go` | Modify | Extend `mockPrompter`; 5 new tests for existing-config scenarios |
| `internal/initflow/config_pinentry.go` | Create | `ConfigPinentryOptions`, `ConfigPinentryResult`, `ConfigPinentryPrompter`, `RunConfigPinentry` |
| `internal/initflow/config_pinentry_test.go` | Create | 6 tests for `RunConfigPinentry` |
| `internal/cli/config_cmd.go` | Modify | Add `pinentry` subcommand under `config` |
| `docs/configuration.md` | Modify | Add init re-run note; add `locksmith config pinentry` reference section |

---

## Task 1: ExistingConfigAction type + Prompter.ExistingConfig + InitResult.ConfigPreexisted

**Files:**
- Modify: `internal/initflow/flow.go`

- [ ] **Step 1: Add `ExistingConfigAction` type and constants before the `Prompter` interface**

In `internal/initflow/flow.go`, insert after the package imports and before `type Prompter interface`:

```go
// ExistingConfigAction is the user's choice when a config file already exists.
type ExistingConfigAction int

const (
	// ActionContinue keeps the existing file; applyInit skips writing config.yaml.
	ActionContinue ExistingConfigAction = iota
	// ActionOverwrite proceeds through the wizard and replaces the file.
	ActionOverwrite
	// ActionExit cancels init without changes.
	ActionExit
)
```

- [ ] **Step 2: Add `ExistingConfig` method to the `Prompter` interface**

In `internal/initflow/flow.go`, add to the `Prompter` interface after the `GPGPinentry` method:

```go
	// ExistingConfig is called when a config file already exists at path.
	// validErr is nil if the file passes validation, or the validation error otherwise.
	ExistingConfig(path string, validErr error) (ExistingConfigAction, error)
```

- [ ] **Step 3: Add `ConfigPreexisted` field to `InitResult`**

In `internal/initflow/flow.go`, add to the `InitResult` struct:

```go
	ConfigPreexisted      bool // true when an existing config was found and kept
```

- [ ] **Step 4: Add stub `ExistingConfig` to `huhPrompter` so it compiles**

```go
// ExistingConfig prompts the user when a config file already exists.
func (p *huhPrompter) ExistingConfig(path string, validErr error) (ExistingConfigAction, error) {
	return ActionExit, nil // placeholder - replaced in Task 3
}
```

- [ ] **Step 5: Verify it compiles**

```bash
go build ./internal/initflow/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/initflow/flow.go
git commit -m "feat(init): add ExistingConfigAction type and Prompter.ExistingConfig interface"
```

---

## Task 2: Tests for existing-config scenarios in RunInit

**Files:**
- Modify: `internal/initflow/flow_test.go`

- [ ] **Step 1: Add `existingConfigAction` and `existingConfigErr` fields to `mockPrompter`**

In `internal/initflow/flow_test.go`, add to the `mockPrompter` struct:

```go
	existingConfigAction   initflow.ExistingConfigAction
	existingConfigErr      error
```

- [ ] **Step 2: Add `ExistingConfig` method to `mockPrompter`**

```go
func (m *mockPrompter) ExistingConfig(_ string, _ error) (initflow.ExistingConfigAction, error) {
	return m.existingConfigAction, m.existingConfigErr
}
```

- [ ] **Step 3: Write the five failing tests**

Append to `internal/initflow/flow_test.go`:

```go
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
```

Note: `TestRunInit_ExistingConfig_Valid_AutoContinues` calls `RunInit` with `Auto: true` but without an explicit configDir — it relies on `t.Setenv("HOME", home)` so the default path `~/.config/locksmith/config.yaml` matches `cfgPath`.

- [ ] **Step 4: Add `config` import to the test file if not already present**

Check the import block; add `"github.com/lorem-dev/locksmith/internal/config"` if needed for `TestRunInit_ExistingConfig_Invalid_AutoOverwrites`.

- [ ] **Step 5: Run tests to confirm they fail (compile errors are expected before Task 3)**

```bash
go test ./internal/initflow/... -run "TestRunInit_ExistingConfig" -v 2>&1 | head -30
```

Expected: compilation failure or test failures — `ExistingConfig` not yet implemented in `RunInit`.

- [ ] **Step 6: Commit the test additions**

```bash
git add internal/initflow/flow_test.go
git commit -m "test(init): add existing-config scenario tests (red)"
```

---

## Task 3: Implement existing-config check in RunInit + applyInit + huhPrompter.ExistingConfig

**Files:**
- Modify: `internal/initflow/flow.go`

- [ ] **Step 1: Insert existing-config check in `RunInit` after `result.ConfigPath` is set**

In `RunInit`, immediately after:
```go
result.ConfigPath = filepath.Join(configDir, "config.yaml")
```

Add:

```go
	// --- Existing config check ---
	if _, statErr := os.Stat(result.ConfigPath); statErr == nil {
		_, validErr := config.Load(result.ConfigPath)
		var action ExistingConfigAction
		if opts.Auto {
			if validErr == nil {
				action = ActionContinue
			} else {
				action = ActionOverwrite
			}
		} else {
			action, err = prompter.ExistingConfig(result.ConfigPath, validErr)
			if err != nil {
				return nil, err
			}
		}
		switch action {
		case ActionExit:
			return nil, fmt.Errorf("cancelled by user")
		case ActionContinue:
			result.ConfigPreexisted = true
		case ActionOverwrite:
			// fall through: wizard continues normally
		}
	}
```

- [ ] **Step 2: Wrap the config-write block in `applyInit` with a `ConfigPreexisted` guard**

In `applyInit`, replace:
```go
	cfg := config.Config{
		Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "~/.config/locksmith/locksmith.sock"},
		Logging:  config.Logging{Level: "info", Format: "text"},
		Vaults:   make(map[string]config.Vault),
		Keys:     make(map[string]config.Key),
	}
	for _, vt := range result.SelectedVaults {
		cfg.Vaults[vt] = config.Vault{Type: vt}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(result.ConfigPath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("  config written to %s\n", result.ConfigPath)
```

With:
```go
	if result.ConfigPreexisted {
		fmt.Printf("  config kept at %s\n", result.ConfigPath)
	} else {
		cfg := config.Config{
			Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "~/.config/locksmith/locksmith.sock"},
			Logging:  config.Logging{Level: "info", Format: "text"},
			Vaults:   make(map[string]config.Vault),
			Keys:     make(map[string]config.Key),
		}
		for _, vt := range result.SelectedVaults {
			cfg.Vaults[vt] = config.Vault{Type: vt}
		}
		data, err := yaml.Marshal(&cfg)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
		if err := os.WriteFile(result.ConfigPath, data, 0o644); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		fmt.Printf("  config written to %s\n", result.ConfigPath)
	}
```

- [ ] **Step 3: Replace the stub `huhPrompter.ExistingConfig` with the real implementation**

Replace the placeholder from Task 1 Step 4 with:

```go
// ExistingConfig prompts the user when a config file already exists at path.
// validErr is nil if the config passed validation, or the error otherwise.
func (p *huhPrompter) ExistingConfig(path string, validErr error) (ExistingConfigAction, error) {
	title := fmt.Sprintf("Config already exists at %s", path)
	var desc string
	continueLabel := "Continue with existing config"
	if validErr == nil {
		desc = "The existing config is valid."
	} else {
		desc = fmt.Sprintf("The existing config is invalid: %v", validErr)
		continueLabel = "Continue with invalid config (not recommended)"
	}
	var selected ExistingConfigAction
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewSelect[ExistingConfigAction]().
			Title(title).
			Description(desc).
			Options(
				huh.NewOption(continueLabel, ActionContinue),
				huh.NewOption("Overwrite with new config", ActionOverwrite),
				huh.NewOption("Exit setup", ActionExit),
			).Value(&selected),
	)))
	if err := form.Run(); err != nil {
		return ActionExit, err
	}
	return selected, nil
}
```

- [ ] **Step 4: Run the existing-config tests**

```bash
go test ./internal/initflow/... -run "TestRunInit_ExistingConfig" -v 2>&1
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Run the full initflow test suite to check for regressions**

```bash
go test ./internal/initflow/... -timeout 30s 2>&1
```

Expected: `ok github.com/lorem-dev/locksmith/internal/initflow`

- [ ] **Step 6: Commit**

```bash
git add internal/initflow/flow.go
git commit -m "feat(init): detect existing config, validate, and offer continue/overwrite/exit"
```

---

## Task 4: RunConfigPinentry - new file

**Files:**
- Create: `internal/initflow/config_pinentry.go`

- [ ] **Step 1: Create the file with types and function signature**

Create `internal/initflow/config_pinentry.go`:

```go
package initflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ConfigPinentryPrompter is the prompt subset needed by RunConfigPinentry.
// huhPrompter satisfies this interface without modification.
type ConfigPinentryPrompter interface {
	GPGPinentry(existingPinentry string) (bool, error)
}

// ConfigPinentryOptions controls RunConfigPinentry behaviour.
type ConfigPinentryOptions struct {
	NoTUI    bool
	Auto     bool
	Prompter ConfigPinentryPrompter // nil = huh TUI
}

// ConfigPinentryResult holds the outcome of RunConfigPinentry.
type ConfigPinentryResult struct {
	Configured   bool   // false if user declined
	PinentryPath string // absolute path of locksmith-pinentry binary
	Replaced     string // previous pinentry-program value, if any
}

// RunConfigPinentry configures locksmith-pinentry as the gpg-agent pinentry program.
// It is the flow behind `locksmith config pinentry`.
func RunConfigPinentry(opts ConfigPinentryOptions) (*ConfigPinentryResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	pinentryPath, err := exec.LookPath("locksmith-pinentry")
	if err != nil {
		return nil, fmt.Errorf("locksmith-pinentry not found in PATH - run 'make init' first")
	}

	gnupgDir := filepath.Join(homeDir, ".gnupg")
	existing := ReadExistingPinentry(gnupgDir)

	var prompter ConfigPinentryPrompter
	if opts.Prompter != nil {
		prompter = opts.Prompter
	} else {
		accessible := opts.NoTUI || os.Getenv("TERM") == "dumb" || !isTerminal()
		prompter = NewHuhPrompter(accessible, nil, nil)
	}

	var configure bool
	if opts.Auto {
		configure = true
	} else {
		configure, err = prompter.GPGPinentry(existing)
		if err != nil {
			return nil, err
		}
	}

	result := &ConfigPinentryResult{PinentryPath: pinentryPath}
	if configure {
		replaced, applyErr := ApplyGPGPinentry(gnupgDir, pinentryPath)
		if applyErr != nil {
			return nil, fmt.Errorf("updating gpg-agent.conf: %w", applyErr)
		}
		exec.Command("gpgconf", "--kill", "gpg-agent").Run() //nolint:errcheck
		result.Configured = true
		result.Replaced = replaced
	}
	return result, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/initflow/...
```

Expected: no errors.

- [ ] **Step 3: Commit the new file**

```bash
git add internal/initflow/config_pinentry.go
git commit -m "feat(initflow): add RunConfigPinentry for standalone GPG pinentry setup"
```

---

## Task 5: Tests for RunConfigPinentry

**Files:**
- Create: `internal/initflow/config_pinentry_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/initflow/config_pinentry_test.go`:

```go
package initflow_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

// mockPinentryPrompter implements ConfigPinentryPrompter for tests.
type mockPinentryPrompter struct {
	accept bool
	err    error
}

func (m *mockPinentryPrompter) GPGPinentry(_ string) (bool, error) {
	return m.accept, m.err
}

// fakePinentryBin creates a fake locksmith-pinentry executable in a temp dir,
// prepends that dir to PATH, and returns the binary path.
func fakePinentryBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "locksmith-pinentry")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("creating fake locksmith-pinentry: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func TestRunConfigPinentry_NotFound(t *testing.T) {
	// Empty PATH - locksmith-pinentry cannot be found.
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err == nil {
		t.Fatal("expected error when locksmith-pinentry not found")
	}
	if !strings.Contains(err.Error(), "locksmith-pinentry not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunConfigPinentry_Auto_Configures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := fakePinentryBin(t)

	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if !result.Configured {
		t.Error("expected Configured = true in auto mode")
	}
	if result.PinentryPath != binPath {
		t.Errorf("PinentryPath = %q, want %q", result.PinentryPath, binPath)
	}
	// Verify gpg-agent.conf was written.
	confPath := filepath.Join(home, ".gnupg", "gpg-agent.conf")
	data, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatalf("gpg-agent.conf not created: %v", readErr)
	}
	if !strings.Contains(string(data), "pinentry-program "+binPath) {
		t.Errorf("gpg-agent.conf does not contain expected line:\n%s", data)
	}
}

func TestRunConfigPinentry_Interactive_Accepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	mp := &mockPinentryPrompter{accept: true}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if !result.Configured {
		t.Error("expected Configured = true when user accepts")
	}
}

func TestRunConfigPinentry_Interactive_Declined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	mp := &mockPinentryPrompter{accept: false}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if result.Configured {
		t.Error("expected Configured = false when user declines")
	}
	// gpg-agent.conf must not be created.
	confPath := filepath.Join(home, ".gnupg", "gpg-agent.conf")
	if _, statErr := os.Stat(confPath); statErr == nil {
		t.Error("gpg-agent.conf should not be created when user declines")
	}
}

func TestRunConfigPinentry_ReplacesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := fakePinentryBin(t)

	// Pre-populate gpg-agent.conf with an existing pinentry-program.
	gnupgDir := filepath.Join(home, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("pinentry-program /opt/homebrew/bin/pinentry-mac\n"), 0o600)

	mp := &mockPinentryPrompter{accept: true}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if result.Replaced != "/opt/homebrew/bin/pinentry-mac" {
		t.Errorf("Replaced = %q, want %q", result.Replaced, "/opt/homebrew/bin/pinentry-mac")
	}
	data, _ := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	content := string(data)
	if !strings.Contains(content, "#pinentry-program /opt/homebrew/bin/pinentry-mac") {
		t.Errorf("old line should be commented out:\n%s", content)
	}
	if !strings.Contains(content, "pinentry-program "+binPath) {
		t.Errorf("new line missing:\n%s", content)
	}
}

func TestRunConfigPinentry_PromptError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	wantErr := errors.New("prompt failed")
	mp := &mockPinentryPrompter{err: wantErr}
	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/initflow/... -run "TestRunConfigPinentry" -v 2>&1
```

Expected: all 6 tests PASS.

- [ ] **Step 3: Run full initflow suite**

```bash
go test ./internal/initflow/... -timeout 30s 2>&1
```

Expected: `ok github.com/lorem-dev/locksmith/internal/initflow`

- [ ] **Step 4: Commit**

```bash
git add internal/initflow/config_pinentry_test.go
git commit -m "test(initflow): add RunConfigPinentry tests"
```

---

## Task 6: `locksmith config pinentry` CLI command

**Files:**
- Modify: `internal/cli/config_cmd.go`

- [ ] **Step 1: Refactor `newConfigCmd` to extract check subcommand and add pinentry subcommand**

Replace the entire contents of `internal/cli/config_cmd.go` with:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/initflow"
)

// newConfigCmd returns the `locksmith config` command group.
func newConfigCmd(cfgFile *string) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration management"}
	cmd.AddCommand(newConfigCheckCmd(cfgFile))
	cmd.AddCommand(newConfigPinentryCmd())
	return cmd
}

func newConfigCheckCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := *cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			fmt.Printf("config OK: %s\n  vaults: %d\n  keys:   %d\n  ttl:    %s\n",
				cfgPath, len(cfg.Vaults), len(cfg.Keys), cfg.Defaults.SessionTTL)
			return nil
		},
	}
}

func newConfigPinentryCmd() *cobra.Command {
	var auto, noTUI bool
	cmd := &cobra.Command{
		Use:   "pinentry",
		Short: "Configure locksmith-pinentry as the gpg-agent pinentry program",
		Long: `Configure locksmith-pinentry as the pinentry program for gpg-agent.

Use this after running 'locksmith init --auto', or any time you want to
(re-)configure GPG passphrase prompts independently of init.

Requires locksmith-pinentry to be installed (run 'make init' once after cloning).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{
				Auto:  auto,
				NoTUI: noTUI,
			})
			if err != nil {
				return err
			}
			if !result.Configured {
				return nil
			}
			fmt.Printf("  locksmith-pinentry found at %s\n", result.PinentryPath)
			if result.Replaced != "" {
				fmt.Printf("  Previous pinentry-program (%s) commented out.\n", result.Replaced)
			}
			fmt.Printf("  Configured: pinentry-program set to %s\n", result.PinentryPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&auto, "auto", false, "configure without prompting")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "use plain-text prompts (also auto-enabled when TERM=dumb or non-TTY stdin)")
	return cmd
}
```

- [ ] **Step 2: Verify it compiles and the subcommand is visible**

```bash
go build ./... && go run ./cmd/locksmith config --help 2>&1
```

Expected output includes:
```
Available Commands:
  check       Validate config file
  pinentry    Configure locksmith-pinentry as the gpg-agent pinentry program
```

- [ ] **Step 3: Run the existing CLI tests to check for regressions**

```bash
go test -timeout 30s -run "TestNewRootCmd|TestGetCmd|TestSession|TestConfig|TestVault|TestIsColor|TestBold|TestColor|TestServeCmd_Present|TestInitCmd|TestRoot|TestFormat|TestPrintError" ./internal/cli/... 2>&1
```

Expected: `ok github.com/lorem-dev/locksmith/internal/cli`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/config_cmd.go
git commit -m "feat(cli): add 'locksmith config pinentry' subcommand"
```

---

## Task 7: Documentation updates

**Files:**
- Modify: `docs/configuration.md`

- [ ] **Step 1: Add re-run note to the init section**

`docs/configuration.md` has no dedicated `locksmith init` section today — the GPG pinentry setup is described under "GPG passphrase and background daemons". Add a new section **before** the "GPG passphrase and background daemons" heading:

```markdown
## Re-running locksmith init

Running `locksmith init` on a machine that already has a config file at the chosen
path will detect the file and validate it. You will be offered three options:

- **Continue with existing config** - skip rewriting the config file; proceed with
  agent and sandbox setup only.
- **Overwrite with new config** - run the full wizard and replace the file.
- **Exit setup** - cancel without any changes.

In `--auto` mode the choice is made automatically: a valid config is kept as-is;
an invalid config is silently replaced.

---
```

- [ ] **Step 2: Add `locksmith config pinentry` section**

Append at the end of `docs/configuration.md` (after the GPG limitations section).
The content to append (verbatim, including the leading `---`):

~~~markdown
---

## locksmith config pinentry

Configures `locksmith-pinentry` as the pinentry program for `gpg-agent`, independently
of `locksmith init`. Use this if you:

- Ran `locksmith init --auto` (which skips the GPG pinentry step), or
- Want to reconfigure after changing your GPG setup.

```bash
locksmith config pinentry [--auto] [--no-tui]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--auto` | Configure without prompting (equivalent to answering "yes") |
| `--no-tui` | Use plain-text prompts instead of the TUI |

**Requires** `locksmith-pinentry` to be installed - run `make init` once after cloning.

The command comments out any existing `pinentry-program` line in
`~/.gnupg/gpg-agent.conf`, writes the new path, and restarts `gpg-agent`.
See "GPG passphrase and background daemons" above for full context.
~~~

- [ ] **Step 3: Run a quick sanity build to make sure nothing is broken**

```bash
go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add docs/configuration.md
git commit -m "docs: add init re-run behaviour and locksmith config pinentry reference"
```

---

## Task 8: Final verification

- [ ] **Step 1: Run the full test suite (excluding the pre-existing hanging test)**

```bash
go test -timeout 30s ./sdk/... ./internal/config/... ./internal/daemon/... \
  ./internal/initflow/... ./plugins/gopass/... ./plugins/keychain/... \
  ./cmd/locksmith-pinentry/... 2>&1
go test -timeout 30s -run "TestNewRootCmd|TestGetCmd|TestSession|TestConfig|TestVault|TestIsColor|TestBold|TestColor|TestServeCmd_Present|TestInitCmd|TestRoot|TestFormat|TestPrintError" \
  ./internal/cli/... 2>&1
```

Expected: all `ok`.

- [ ] **Step 2: Build all binaries**

```bash
make build 2>&1
```

Expected: `bin/locksmith` and `bin/locksmith-pinentry` produced with no errors.

- [ ] **Step 3: Smoke-test the new command help**

```bash
./bin/locksmith config pinentry --help 2>&1
```

Expected output contains `"Configure locksmith-pinentry as the gpg-agent pinentry program"` and lists `--auto` and `--no-tui` flags.

- [ ] **Step 4: Commit if any last-minute fixes were needed; otherwise done**
