//go:build darwin

package pinentry

import (
	"fmt"
	"os/exec"
	"strings"
)

// tryGUI shows a native macOS password dialog via osascript.
func tryGUI(desc, prompt string) (string, error) {
	return tryGUIWithCmd(desc, prompt, func(script string) ([]byte, error) {
		return exec.Command("osascript", "-e", script).Output() //nolint:gosec // G204: variable script arg
	})
}

// tryGUIWithCmd is the testable core of tryGUI.
// runCmd receives the osascript source and returns its stdout output.
func tryGUIWithCmd(desc, prompt string, runCmd func(script string) ([]byte, error)) (string, error) {
	script := fmt.Sprintf(
		`display dialog %q with hidden answer default answer "" buttons {"Cancel", "OK"} default button "OK"`,
		prompt+": "+desc,
	)
	out, err := runCmd(script)
	if err != nil {
		return "", errCancelled
	}
	// Output: "button returned:OK, text returned:<pin>"
	result := strings.TrimSpace(string(out))
	pin, ok := parseOsascriptOutput(result)
	if !ok {
		return "", errCancelled
	}
	return pin, nil
}

// parseOsascriptOutput extracts the password from osascript dialog output.
// Returns the pin and true if the output matches the expected format,
// or ("", false) if the format is unrecognised.
func parseOsascriptOutput(result string) (string, bool) {
	const prefix = "button returned:OK, text returned:"
	idx := strings.Index(result, prefix)
	if idx < 0 {
		return "", false
	}
	return result[idx+len(prefix):], true
}
