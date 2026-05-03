package initflow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lorem-dev/locksmith/internal/bundled"
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

	pinentryPath, err := bundled.PinentryPath()
	if err != nil {
		return nil, fmt.Errorf("resolving pinentry path: %w", err)
	}
	_, statErr := os.Stat(pinentryPath)
	if errors.Is(statErr, fs.ErrNotExist) {
		// Pinentry not yet extracted (e.g. user is running `locksmith config
		// pinentry` standalone without `init`). Extract it now.
		bundle, openErr := bundled.OpenBundle()
		if openErr != nil {
			if errors.Is(openErr, bundled.ErrEmptyBundle) {
				return nil, fmt.Errorf(
					"locksmith-pinentry not extracted and bundle is empty (dev build):" +
						" run `make build-all` then re-run init",
				)
			}
			return nil, fmt.Errorf("opening bundle: %w", openErr)
		}
		extractErr := bundled.Extract(bundle, bundled.ExtractOptions{
			Names:        []string{"locksmith-pinentry"},
			PinentryPath: pinentryPath,
		})
		if extractErr != nil {
			return nil, fmt.Errorf("extracting pinentry: %w", extractErr)
		}
	} else if statErr != nil {
		return nil, fmt.Errorf("stat %s: %w", pinentryPath, statErr)
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
			return nil, fmt.Errorf("prompting for GPG pinentry: %w", err)
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
