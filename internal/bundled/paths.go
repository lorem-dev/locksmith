// Package bundled exposes default vault plugins and locksmith-pinentry that
// ship embedded inside the locksmith binary, plus the extraction logic that
// places them at canonical paths under ~/.config/locksmith/.
package bundled

import (
	"os"
	"path/filepath"
)

// PluginsDir returns ~/.config/locksmith/plugins/, the canonical destination
// for extracted vault plugins.
func PluginsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "locksmith", "plugins"), nil
}

// BinDir returns ~/.config/locksmith/bin/, where extracted helper binaries
// (currently only locksmith-pinentry) live.
func BinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "locksmith", "bin"), nil
}

// PinentryPath returns the absolute path of the extracted locksmith-pinentry
// binary. The path is stable across reinstalls and is what gpg-agent.conf
// records.
func PinentryPath() (string, error) {
	bin, err := BinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bin, "locksmith-pinentry"), nil
}
