# fatih/color Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace custom ANSI color helpers in `internal/cli/color.go` with direct `fatih/color` usage, encapsulating color detection in a single `IsNoColor()` function applied once at startup.

**Architecture:** Add `IsNoColor()` to `color.go` and a `PersistentPreRunE` to the root command that sets `color.NoColor = IsNoColor()` before any subcommand runs. Update `errors.go` and `get.go` to call `fatih/color` Sprint functions directly (no `bool` parameter). Then remove the now-unused old functions and their tests in a second commit.

**Tech Stack:** Go 1.25, `github.com/fatih/color v1.13.0` (already in `go.mod` as indirect), `github.com/mattn/go-isatty` (already direct).

---

## File Map

| File | Change |
|------|--------|
| `internal/cli/color.go` | Task 1: add `IsNoColor()`; Task 2: remove old functions |
| `internal/cli/root.go` | Task 1: add `PersistentPreRunE` |
| `internal/cli/errors.go` | Task 1: remove `color` local var, use `fatih/color` |
| `internal/cli/get.go` | Task 1: remove `color` local var, use `fatih/color` |
| `internal/cli/color_test.go` | Task 2: replace old tests with `TestIsNoColor_*` |
| `go.mod` | Task 2: `go mod tidy` promotes `fatih/color` to direct |

---

## Task 1: Add IsNoColor + PersistentPreRunE + migrate callers

**Files:**
- Modify: `internal/cli/color.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/errors.go`
- Modify: `internal/cli/get.go`

- [ ] **Step 1: Add `IsNoColor()` to `color.go`**

Open `internal/cli/color.go`. The file currently contains `IsColorEnabled`, `Bold`, `ColorRed`, `ColorYellow`, `ColorGray`, `ColorCyan`. Add `IsNoColor` at the top, before `IsColorEnabled` (keep old functions for now - they will be removed in Task 2):

```go
// IsNoColor reports whether ANSI color output should be disabled.
// Color is disabled when NO_COLOR is set or stderr is not a TTY.
func IsNoColor() bool {
	return os.Getenv("NO_COLOR") != "" || !isatty.IsTerminal(os.Stderr.Fd())
}
```

The existing imports (`"os"` and `"github.com/mattn/go-isatty"`) already cover this function - no new imports needed.

- [ ] **Step 2: Add `PersistentPreRunE` to `newRootCmd` in `root.go`**

Open `internal/cli/root.go`. The `root` command currently has no `PersistentPreRunE`. Add it, and import `fatih/color`:

Replace the entire file with:

```go
package cli

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the cobra root command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	var cfgFile string

	root := &cobra.Command{
		Use:           "locksmith",
		Short:         "Secure secret middleware for AI agents",
		Long:          "Locksmith gives AI agents secure access to secrets from vault providers (macOS Keychain, gopass, etc.) with per-session caching and Touch ID support.",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			color.NoColor = IsNoColor()
			return nil
		},
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/locksmith/config.yaml)")
	root.AddCommand(
		newServeCmd(&cfgFile),
		newGetCmd(),
		newSessionCmd(),
		newVaultCmd(),
		newConfigCmd(&cfgFile),
		newInitCmd(),
	)
	return root
}
```

- [ ] **Step 3: Update `errors.go` to use `fatih/color` directly**

Open `internal/cli/errors.go`. Replace the `PrintError` function:

```go
// PrintError prints a formatted error (and optional hint) to stderr.
// Uses ANSI color when stderr is a TTY and NO_COLOR is not set.
func PrintError(err error) {
	msg, hint := FormatErrorParts(err)

	fmt.Fprintf(os.Stderr, "%s %s\n", color.New(color.FgRed, color.Bold).Sprint("Error:"), msg)
	if hint != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", color.New(color.FgYellow, color.Bold).Sprint("Hint:"), color.New(color.FgHiBlack).Sprint(hint))
	}
}
```

Add `"github.com/fatih/color"` to the import block. Remove the `color := IsColorEnabled(false)` line (it's now gone). The final import block for `errors.go`:

```go
import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)
```

- [ ] **Step 4: Update `get.go` to use `fatih/color` directly**

Open `internal/cli/get.go`. In the `RunE` closure, find the session-start block:

```go
sessionID = startResp.SessionId
color := IsColorEnabled(false)
fmt.Fprintf(os.Stderr, "locksmith: session started (expires %s)\n  export LOCKSMITH_SESSION=%s\n",
    startResp.ExpiresAt, ColorCyan(sdk.HideSession(sessionID), color),
)
```

Replace with:

```go
sessionID = startResp.SessionId
fmt.Fprintf(os.Stderr, "locksmith: session started (expires %s)\n  export LOCKSMITH_SESSION=%s\n",
    startResp.ExpiresAt, color.New(color.FgCyan, color.Bold).Sprint(sdk.HideSession(sessionID)),
)
```

Add `"github.com/fatih/color"` to the import block. The final import block for `get.go`:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/sdk"
)
```

- [ ] **Step 5: Verify the build compiles**

```bash
go build ./internal/cli/...
```

Expected: no errors. (The old functions in `color.go` are still there and unused — that's fine, compiler won't complain about unused functions.)

- [ ] **Step 6: Run the CLI test suite**

```bash
go test -timeout 30s -run "TestNewRootCmd|TestGetCmd|TestSession|TestConfig|TestVault|TestIsColor|TestBold|TestColor|TestServeCmd_Present|TestInitCmd|TestRoot|TestFormat|TestPrintError" ./internal/cli/... 2>&1
```

Expected: `ok github.com/lorem-dev/locksmith/internal/cli`

- [ ] **Step 7: Commit**

```bash
git add internal/cli/color.go internal/cli/root.go internal/cli/errors.go internal/cli/get.go
git -c commit.gpgsign=false commit -m "refactor(cli): migrate color output to fatih/color, add IsNoColor helper"
```

---

## Task 2: Remove old color functions + update tests + go mod tidy

**Files:**
- Modify: `internal/cli/color.go`
- Modify: `internal/cli/color_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Write the new `TestIsNoColor_*` tests**

Open `internal/cli/color_test.go`. Replace the **entire file** with:

```go
package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestIsNoColor_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !cli.IsNoColor() {
		t.Error("expected IsNoColor() = true when NO_COLOR is set")
	}
}

func TestIsNoColor_NotTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	// In the test environment stderr is not a real TTY, so IsNoColor returns true.
	if !cli.IsNoColor() {
		t.Error("expected IsNoColor() = true when stderr is not a TTY")
	}
}
```

- [ ] **Step 2: Run the new tests to confirm they compile and pass**

```bash
go test ./internal/cli/... -run "TestIsNoColor" -v 2>&1
```

Expected:
```
--- PASS: TestIsNoColor_NoColorEnv
--- PASS: TestIsNoColor_NotTTY
ok  github.com/lorem-dev/locksmith/internal/cli
```

- [ ] **Step 3: Remove the old functions from `color.go`**

Replace the **entire contents** of `internal/cli/color.go` with:

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

- [ ] **Step 4: Verify the build still compiles**

```bash
go build ./internal/cli/...
```

Expected: no errors.

- [ ] **Step 5: Run go mod tidy to promote fatih/color to direct**

```bash
go mod tidy
```

Expected: `go.mod` updated — `github.com/fatih/color` loses the `// indirect` comment.

Verify:
```bash
grep "fatih/color" go.mod
```

Expected output: `github.com/fatih/color v1.13.0` (no `// indirect`).

- [ ] **Step 6: Run the full CLI test suite**

```bash
go test -timeout 30s -run "TestNewRootCmd|TestGetCmd|TestSession|TestConfig|TestVault|TestIsNoColor|TestServeCmd_Present|TestInitCmd|TestRoot|TestFormat|TestPrintError" ./internal/cli/... 2>&1
```

Expected: `ok github.com/lorem-dev/locksmith/internal/cli`

- [ ] **Step 7: Commit**

```bash
git add internal/cli/color.go internal/cli/color_test.go go.mod go.sum
git -c commit.gpgsign=false commit -m "refactor(cli): remove custom color functions, keep only IsNoColor"
```
