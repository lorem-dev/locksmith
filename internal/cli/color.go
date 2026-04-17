package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
)

// IsColorEnabled reports whether ANSI color should be used.
// Pass isTTY=true to override the automatic stderr TTY detection (for testing).
func IsColorEnabled(isTTY bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if isTTY {
		return true
	}
	return isatty.IsTerminal(os.Stderr.Fd())
}

// Bold returns text wrapped in ANSI bold codes when color is enabled.
func Bold(text string, color bool) string {
	if !color {
		return text
	}
	return fmt.Sprintf("\033[1m%s\033[0m", text)
}

// ColorRed returns text in bold red when color is enabled.
func ColorRed(text string, color bool) string {
	if !color {
		return text
	}
	return fmt.Sprintf("\033[1;31m%s\033[0m", text)
}

// ColorYellow returns text in bold yellow when color is enabled.
func ColorYellow(text string, color bool) string {
	if !color {
		return text
	}
	return fmt.Sprintf("\033[1;33m%s\033[0m", text)
}

// ColorGray returns text in gray when color is enabled.
func ColorGray(text string, color bool) string {
	if !color {
		return text
	}
	return fmt.Sprintf("\033[90m%s\033[0m", text)
}
