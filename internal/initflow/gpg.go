package initflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadExistingPinentry returns the path from the current `pinentry-program` line
// in gnupgDir/gpg-agent.conf, or "" if none is set or the file does not exist.
func ReadExistingPinentry(gnupgDir string) string {
	data, err := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "pinentry-program ") {
			return strings.TrimPrefix(trimmed, "pinentry-program ")
		}
	}
	return ""
}

// ApplyGPGPinentry writes pinentryPath as the new pinentry-program in
// gnupgDir/gpg-agent.conf. If an existing uncommented pinentry-program line
// is found, it is commented out and its path is returned so the caller can
// warn the user. Returns "" if no existing line was found.
func ApplyGPGPinentry(gnupgDir, pinentryPath string) (replaced string, err error) {
	if err := os.MkdirAll(gnupgDir, 0o700); err != nil {
		return "", fmt.Errorf("creating .gnupg dir: %w", err)
	}
	confPath := filepath.Join(gnupgDir, "gpg-agent.conf")

	var lines []string
	if data, readErr := os.ReadFile(confPath); readErr == nil {
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "pinentry-program ") {
				// Comment out the existing line and record what it was.
				replaced = strings.TrimPrefix(trimmed, "pinentry-program ")
				lines = append(lines, "#"+line)
			} else {
				lines = append(lines, line)
			}
		}
		// Remove trailing blank lines introduced by splitting.
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	lines = append(lines, "pinentry-program "+pinentryPath)
	content := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(confPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing gpg-agent.conf: %w", err)
	}
	return replaced, nil
}
