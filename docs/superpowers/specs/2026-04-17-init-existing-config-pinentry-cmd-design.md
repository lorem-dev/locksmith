# Design: init existing-config handling + `config pinentry` command

Date: 2026-04-17

## Overview

Two related improvements to the Locksmith CLI:

1. `locksmith init` detects an existing config at the chosen path, validates it, and
   presents three options before proceeding.
2. New `locksmith config pinentry` subcommand lets users configure `locksmith-pinentry`
   in `~/.gnupg/gpg-agent.conf` independently of `locksmith init`.

---

## Feature 1: init with existing config

### Context

Currently `applyInit` always overwrites `config.yaml`. If a user re-runs `locksmith init`
on a machine that already has a config, their vaults and keys are silently replaced.

### Flow

After `configDir` is resolved (step 1 of `RunInit`), before any other prompts:

```
configPath := filepath.Join(configDir, "config.yaml")

if _, err := os.Stat(configPath); err == nil {      // file exists
    _, validErr := config.Load(configPath)          // nil = valid

    var action ExistingConfigAction
    if opts.Auto {
        // auto mode: never prompt, decide by validity
        if validErr == nil {
            action = ActionContinue   // valid -> keep as-is
        } else {
            action = ActionOverwrite  // invalid -> regenerate silently
        }
    } else {
        action, err = prompter.ExistingConfig(configPath, validErr)
        if err != nil {
            return nil, err
        }
    }

    switch action {
    case ActionExit:
        return nil, fmt.Errorf("cancelled by user")
    case ActionContinue:
        result.ConfigPreexisted = true  // applyInit skips config.yaml write
    case ActionOverwrite:
        // fall through: wizard continues normally
    }
}
```

### Prompter interface addition

```go
type ExistingConfigAction int

const (
    ActionContinue  ExistingConfigAction = iota // keep existing file
    ActionOverwrite                             // proceed with wizard, replace file
    ActionExit                                  // cancel init
)

// ExistingConfig is called when a config file already exists at path.
// validErr is nil if the file passes validation, or the validation error otherwise.
ExistingConfig(path string, validErr error) (ExistingConfigAction, error)
```

`huhPrompter.ExistingConfig` renders a `huh.Select` with three options:
- Valid config: "Config found at <path> (valid). Continue with existing / Overwrite / Exit"
- Invalid config: "Config found at <path> - invalid: <err>. Continue anyway / Overwrite / Exit"

### InitResult addition

```go
ConfigPreexisted bool // true when an existing config was found at startup
```

### applyInit change

`applyInit` skips writing `config.yaml` (and printing "config written to ...") when
`result.ConfigPreexisted == true`. All other steps (GPG pinentry, agent installation,
sandbox) continue unchanged regardless of `ConfigPreexisted`.

### mockPrompter addition (tests)

```go
existingConfigAction   ExistingConfigAction
existingConfigErr      error
```
```go
func (m *mockPrompter) ExistingConfig(_ string, _ error) (ExistingConfigAction, error) {
    return m.existingConfigAction, m.existingConfigErr
}
```

New tests:
- `TestRunInit_ExistingConfig_Valid_Continue` - valid config + ActionContinue -> no overwrite
- `TestRunInit_ExistingConfig_Valid_Overwrite` - valid config + ActionOverwrite -> file rewritten
- `TestRunInit_ExistingConfig_Invalid_AutoOverwrites` - invalid config + auto -> overwrite
- `TestRunInit_ExistingConfig_Valid_AutoContinues` - valid config + auto -> no overwrite
- `TestRunInit_ExistingConfig_Exit` - ActionExit -> returns error

---

## Feature 2: `locksmith config pinentry`

### Context

During `locksmith init`, users can opt in to configure `locksmith-pinentry` as the
gpg-agent pinentry program. There is currently no way to do this after init, or on
machines where init was run with `--auto` (which skips the step).

### New flow function

```go
// ConfigPinentryOptions controls RunConfigPinentry behaviour.
type ConfigPinentryOptions struct {
    Auto     bool
    Prompter ConfigPinentryPrompter // nil = huh TUI
}

// ConfigPinentryResult holds the outcome of RunConfigPinentry.
type ConfigPinentryResult struct {
    Configured bool   // false if user declined
    Replaced   string // previous pinentry-program value, if any
}

// ConfigPinentryPrompter is the prompt subset needed by RunConfigPinentry.
// huhPrompter satisfies this interface without changes.
type ConfigPinentryPrompter interface {
    GPGPinentry(existingPinentry string) (bool, error)
}

func RunConfigPinentry(opts ConfigPinentryOptions) (*ConfigPinentryResult, error)
```

If `opts.Prompter == nil`, `RunConfigPinentry` constructs a default prompter:
```go
accessible := opts.NoTUI || os.Getenv("TERM") == "dumb" || !isTerminal()
opts.Prompter = NewHuhPrompter(accessible, nil, nil)
```
`ConfigPinentryOptions` therefore also carries `NoTUI bool` (mirrors `InitOptions`).

Flow inside `RunConfigPinentry`:
1. `os.UserHomeDir()` to resolve `gnupgDir`
2. `exec.LookPath("locksmith-pinentry")` - if not found, return error:
   `"locksmith-pinentry not found in PATH - run 'make init' first"`
3. `ReadExistingPinentry(gnupgDir)` to get current value
4. If `opts.Auto`: `configure = true`; else call `opts.Prompter.GPGPinentry(existing)`
5. If `configure`:
   - `ApplyGPGPinentry(gnupgDir, pinentryPath)` -> `replaced, err`
   - `exec.Command("gpgconf", "--kill", "gpg-agent").Run()`
6. Return `&ConfigPinentryResult{Configured: configure, Replaced: replaced}`

### New CLI command

File: `internal/cli/config_cmd.go` (existing file, add subcommand)

```
locksmith config pinentry [--auto]
```

Output on success:
```
locksmith-pinentry found at /Users/you/go/bin/locksmith-pinentry
Configured: pinentry-program set to /Users/you/go/bin/locksmith-pinentry
```
If a previous entry was replaced:
```
Previous pinentry-program (/opt/homebrew/bin/pinentry-mac) commented out.
```
If user declines: exit 0, no output.

`--auto` flag mirrors `locksmith init --auto`: skips prompt, configures without asking.

### Tests

New file `internal/initflow/config_pinentry_test.go`:
- `TestRunConfigPinentry_NotFound` - locksmith-pinentry absent -> error
- `TestRunConfigPinentry_Auto_Configures` - auto mode -> configured without prompt
- `TestRunConfigPinentry_Interactive_Accepted` - user accepts -> configured
- `TestRunConfigPinentry_Interactive_Declined` - user declines -> Configured=false, no file change
- `TestRunConfigPinentry_ReplacesExisting` - existing pinentry -> Replaced is set
- `TestRunConfigPinentry_PromptError` - prompter returns error -> RunConfigPinentry propagates it

---

## Files changed

| File | Change |
|------|--------|
| `internal/initflow/flow.go` | Add `ExistingConfigAction`, `ExistingConfig` to `Prompter`; check in `RunInit`; skip write in `applyInit` when `ConfigPreexisted`; add `ConfigPreexisted` to `InitResult` |
| `internal/initflow/flow_test.go` | Add `existingConfigAction/Err` to `mockPrompter`; 5 new tests |
| `internal/initflow/config_pinentry.go` | New file: `ConfigPinentryOptions` (incl. `NoTUI bool`), `ConfigPinentryResult`, `ConfigPinentryPrompter`, `RunConfigPinentry` |
| `internal/initflow/config_pinentry_test.go` | New file: 5 tests |
| `internal/cli/config_cmd.go` | Add `pinentry` subcommand under `config` |
| `docs/configuration.md` | Add `locksmith config pinentry` reference; update `locksmith init` re-run behaviour section |

No changes to `internal/initflow/gpg.go`, `internal/config/`, or any plugin code.

---

## Documentation updates (`docs/configuration.md`)

### `locksmith init` section (update)

Add a note that re-running `locksmith init` on a machine with an existing config will
detect the file, validate it, and present three options: continue with the existing
config, overwrite it with a fresh wizard run, or exit without changes.

### New section: `locksmith config pinentry`

```
## locksmith config pinentry

Configures locksmith-pinentry as the pinentry program for gpg-agent, independently
of locksmith init. Use this if you skipped the step during init, or ran init with
--auto.

    locksmith config pinentry [--auto]

Flags:
  --auto   Configure without prompting (equivalent to answering "yes")

Requires locksmith-pinentry to be installed (run make init once after cloning).
The command comments out any existing pinentry-program line in
~/.gnupg/gpg-agent.conf, writes the new path, and restarts gpg-agent.
```
