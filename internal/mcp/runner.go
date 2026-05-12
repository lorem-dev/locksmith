package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// EnvMapping maps an environment variable name to a secret reference.
type EnvMapping struct {
	Var string
	Ref SecretRef
}

// Run resolves secrets, injects them as environment variables, and runs command.
// command[0] is the executable; command[1:] are its arguments.
// The subprocess inherits the current process's stdin, stdout, and stderr.
func Run(ctx context.Context, fetcher SecretFetcher, envMappings []EnvMapping, command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("no command specified")
	}
	environ := os.Environ()
	for _, m := range envMappings {
		value, err := fetcher.Fetch(ctx, m.Ref)
		if err != nil {
			return fmt.Errorf("resolving env %s: %w", m.Var, err)
		}
		environ = append(environ, m.Var+"="+value)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) //nolint:gosec // G204: command is user-provided config
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", command[0], err)
	}
	return nil
}
