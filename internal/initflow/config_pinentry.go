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
