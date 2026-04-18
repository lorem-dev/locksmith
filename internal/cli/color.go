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
