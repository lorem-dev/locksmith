// Package pinentry implements the Assuan pinentry protocol and the
// platform-specific UI for the locksmith-pinentry binary.
package pinentry

import "os"

// Run starts the pinentry Assuan protocol loop on stdin/stdout.
// Call this from the main() of cmd/locksmith-pinentry.
func Run() {
	run(os.Stdin, os.Stdout, defaultGetPassword)
}
