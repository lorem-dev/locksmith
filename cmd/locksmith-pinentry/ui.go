package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// readPasswordFn is the function used to read a password from a file descriptor.
// It is a variable so tests can replace it without a real TTY.
var readPasswordFn = func(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

// openTTYFn opens /dev/tty for interactive prompting.
// It is a variable so tests can inject a fake writer without a real terminal.
var openTTYFn = func() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

// defaultGetPassword selects the best available UI and prompts for a passphrase.
func defaultGetPassword(desc, prompt string) (string, error) {
	return getPassword(desc, prompt, tryGUI, openTTYFn, readPasswordFn)
}

// getPassword is the testable core of defaultGetPassword.
// guiFn is tried first; on failure it falls back to TTY.
// openTTY opens the terminal device; readFn reads the password from its fd.
func getPassword(
	desc, prompt string,
	guiFn func(desc, prompt string) (string, error),
	openTTY func() (*os.File, error),
	readFn func(fd int) ([]byte, error),
) (string, error) {
	if prompt == "" {
		prompt = "Passphrase"
	}

	// Try GUI first (platform-specific), then fall back to TTY.
	if pin, err := guiFn(desc, prompt); err == nil {
		return pin, nil
	}

	// TTY fallback: open /dev/tty directly so it works even when stdin/stdout
	// are redirected to the gpg-agent socket.
	tty, err := openTTY()
	if err != nil {
		return "", errCancelled
	}
	defer tty.Close()

	fmt.Fprintf(tty, "%s: ", prompt)
	pin, err := readFn(int(tty.Fd()))
	fmt.Fprintln(tty)
	if err != nil {
		return "", errCancelled
	}
	return string(pin), nil
}
