//go:build !darwin

package pinentry

import (
	"os"
	"os/exec"
	"strings"
)

// tryGUI tries zenity then kdialog for a password dialog on Linux.
//
// The desc and prompt strings originate from the local pinentry caller (gpg)
// over the Assuan protocol, not from network input. They are passed as
// separate argv entries to a fixed binary name, so there is no shell
// interpretation and no command-injection vector.
func tryGUI(desc, prompt string) (string, error) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", errCancelled
	}

	// Try zenity first. The --title=<value> form is unambiguous regardless
	// of prompt content because the first '=' terminates the flag name.
	if _, err := exec.LookPath("zenity"); err == nil {
		//nolint:gosec // G204: fixed binary; --title=<prompt> form prevents flag-injection
		out, err := exec.Command("zenity", "--password", "--title="+prompt).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\n"), nil
		}
	}

	// Fall back to kdialog. prompt and desc come from the trusted local
	// pinentry caller (gpg), not from attacker-controlled input.
	if _, err := exec.LookPath("kdialog"); err == nil {
		//nolint:gosec // G204: fixed binary; prompt/desc are from trusted local gpg caller
		out, err := exec.Command("kdialog", "--password", prompt+": "+desc).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\n"), nil
		}
	}

	return "", errCancelled
}
