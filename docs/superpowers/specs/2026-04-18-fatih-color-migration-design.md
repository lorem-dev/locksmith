# Design: Migrate CLI color helpers to fatih/color

Date: 2026-04-18

## Overview

Replace the hand-rolled ANSI color functions in `internal/cli/color.go` with direct
use of `github.com/fatih/color`. Callers no longer pass a `bool` to color functions;
color detection is encapsulated in a single `IsNoColor()` helper and applied once at
program startup via `color.NoColor`.

---

## Motivation

The current approach has two problems:

1. Every call site must independently resolve color state:
   `color := IsColorEnabled(false)` then pass `color` to every function call.
2. The custom ANSI escape sequences duplicate functionality already in `fatih/color`,
   which is already a dependency (currently `indirect`).

---

## Design

### `internal/cli/color.go` (replace contents)

The file is kept but reduced to one function:

```go
package cli

import (
    "os"

    "github.com/mattn/go-isatty"
)

// IsNoColor reports whether ANSI color output should be disabled.
// Color is disabled when NO_COLOR is set or stderr is not a TTY.
func IsNoColor() bool {
    return os.Getenv("NO_COLOR") != "" || !isatty.IsTerminal(os.Stderr.Fd())
}
```

`fatih/color` auto-detects based on stdout by default. Overriding via `color.NoColor`
with a stderr-aware check is correct here because all colored output in this codebase
goes to stderr (`PrintError`, session start message in `get.go`).

### `internal/cli/root.go` (add PersistentPreRunE)

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    color.NoColor = IsNoColor()
    return nil
},
```

This runs before every subcommand, setting the global once per invocation.
`fatih/color`'s `Sprint`-style functions read `color.NoColor` on every call, so
setting it here is sufficient.

### `internal/cli/errors.go` (update color calls)

Remove `color := IsColorEnabled(false)`. Replace color function calls:

| Before | After |
|--------|-------|
| `ColorRed("Error:", color)` | `color.New(color.FgRed, color.Bold).Sprint("Error:")` |
| `ColorYellow("Hint:", color)` | `color.New(color.FgYellow, color.Bold).Sprint("Hint:")` |
| `ColorGray(hint, color)` | `color.New(color.FgHiBlack).Sprint(hint)` |

### `internal/cli/get.go` (update color call)

Remove `color := IsColorEnabled(false)`. Replace:

| Before | After |
|--------|-------|
| `ColorCyan(sdk.HideSession(sessionID), color)` | `color.New(color.FgCyan, color.Bold).Sprint(sdk.HideSession(sessionID))` |

### `go.mod`

`fatih/color` is already present as `// indirect`. After the explicit import is added
to `root.go`, `go mod tidy` promotes it to a direct dependency automatically.

---

## Tests

### `internal/cli/color_test.go` (replace contents)

Old tests for the deleted functions (`Bold`, `ColorRed`, etc.) are removed.
New tests cover `IsNoColor`:

- `TestIsNoColor_NoColorEnv` - `NO_COLOR` set -> `true`
- `TestIsNoColor_NotTTY` - no `NO_COLOR`, non-TTY stderr (test environment) -> `true`

The `color.NoColor` global is not tested directly in unit tests; it is set by
`PersistentPreRunE` at runtime. Existing `TestPrintError` tests in `errors_test.go`
continue to work unchanged - they test output format, not color mechanism.

---

## Files changed

| File | Change |
|------|--------|
| `internal/cli/color.go` | Replace: remove 5 color functions + `IsColorEnabled`; keep only `IsNoColor()` |
| `internal/cli/color_test.go` | Replace: remove old tests; add `TestIsNoColor_*` |
| `internal/cli/root.go` | Add `PersistentPreRunE` that sets `color.NoColor = IsNoColor()` |
| `internal/cli/errors.go` | Remove `color` local var; use `fatih/color` Sprint functions |
| `internal/cli/get.go` | Remove `color` local var; use `fatih/color` Sprint function |
| `go.mod` | `fatih/color` promoted from indirect to direct after `go mod tidy` |

No changes to daemon, SDK, plugin, or initflow code.
