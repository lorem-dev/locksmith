//go:build !darwin

package pinentry

import (
	"os"
	"os/exec"
	"strings"
)

// tryGUI tries zenity then kdialog for a password dialog on Linux.
func tryGUI(desc, prompt string) (string, error) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", errCancelled
	}

	// Try zenity first.
	if _, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command("zenity", "--password", "--title="+prompt).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\n"), nil
		}
	}

	// Fall back to kdialog.
	if _, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command("kdialog", "--password", prompt+": "+desc).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\n"), nil
		}
	}

	return "", errCancelled
}
