// Package shellhook detects the user's shell and manages the locksmith daemon
// autostart snippet in shell rc files.
package shellhook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Shell identifies the user's interactive shell.
type Shell int

const (
	ShellUnknown Shell = iota
	ShellBash
	ShellZsh
	ShellAsh
	ShellFish
)

// marker is the unique comment line that identifies an existing installation.
const marker = "# locksmith daemon autostart"

// DetectShell identifies the current shell by inspecting $SHELL then $0.
func DetectShell() Shell {
	for _, key := range []string{"SHELL", "0"} {
		if s := parseShellPath(os.Getenv(key)); s != ShellUnknown {
			return s
		}
	}
	return ShellUnknown
}

// parseShellPath maps a shell binary path to a Shell constant.
func parseShellPath(path string) Shell {
	switch strings.ToLower(filepath.Base(path)) {
	case "zsh":
		return ShellZsh
	case "bash":
		return ShellBash
	case "ash":
		return ShellAsh
	case "fish":
		return ShellFish
	}
	return ShellUnknown
}

// RCFile returns the rc file path for s and true if the shell is known.
func RCFile(s Shell) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	switch s {
	case ShellZsh:
		return filepath.Join(home, ".zshrc"), true
	case ShellBash:
		return filepath.Join(home, ".bashrc"), true
	case ShellAsh:
		if env := os.Getenv("ENV"); env != "" {
			return env, true
		}
		return filepath.Join(home, ".profile"), true
	case ShellFish:
		return filepath.Join(home, ".config", "fish", "config.fish"), true
	}
	return "", false
}

// Snippet returns the rc-file snippet for s.
// For ShellUnknown it returns the bare command without a marker or guard.
func Snippet(s Shell) string {
	// ShellUnknown: return the bare command without a guard or marker, stderr
	// redirected to /dev/null so missing-locksmith errors stay silent.
	if s == ShellUnknown {
		return "locksmith _autostart 2>/dev/null"
	}
	if s == ShellFish {
		return marker + "\nif command -v locksmith >/dev/null 2>&1; locksmith _autostart 2>/dev/null; end"
	}
	return marker + "\nif command -v locksmith >/dev/null 2>&1; then locksmith _autostart 2>/dev/null; fi"
}

// IsInstalled reports whether the marker is already present in rcFile.
// Returns false (not an error) if the file does not exist.
func IsInstalled(rcFile string) (bool, error) {
	data, err := os.ReadFile(rcFile) //nolint:gosec // G304: rcFile is derived from user home dir
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", rcFile, err)
	}
	return strings.Contains(string(data), marker), nil
}

// Install appends the shell-appropriate snippet to rcFile, preceded by a blank
// line. The caller must check IsInstalled first; Install does not de-duplicate.
func Install(rcFile string, s Shell) error {
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G304
	if err != nil {
		return fmt.Errorf("opening %s: %w", rcFile, err)
	}
	defer f.Close() //nolint:errcheck // close of append-only file; write error already captured
	if _, err = fmt.Fprintf(f, "\n%s\n", Snippet(s)); err != nil {
		return fmt.Errorf("writing to %s: %w", rcFile, err)
	}
	return nil
}
