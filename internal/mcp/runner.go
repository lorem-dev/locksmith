package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/lorem-dev/locksmith/internal/log"
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
	log.Debug().
		Str("command", command[0]).
		Int("args", len(command)-1).
		Int("env_injections", len(envMappings)).
		Msg("mcp local: starting")
	environ := os.Environ()
	for _, m := range envMappings {
		log.Debug().
			Str("var", m.Var).
			Str("key_alias", m.Ref.KeyAlias).
			Str("vault", m.Ref.VaultName).
			Str("path", m.Ref.Path).
			Msg("mcp local: resolving env var")
		value, err := fetcher.Fetch(ctx, m.Ref)
		if err != nil {
			log.Debug().Err(err).Str("var", m.Var).Msg("mcp local: env fetch failed")
			return fmt.Errorf("resolving env %s: %w", m.Var, err)
		}
		log.Debug().Str("var", m.Var).Int("len", len(value)).Msg("mcp local: env resolved")
		environ = append(environ, m.Var+"="+value)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) //nolint:gosec // G204: command is user-provided config
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Debug().Str("command", command[0]).Msg("mcp local: exec")
	if err := cmd.Run(); err != nil {
		log.Debug().Err(err).Msg("mcp local: subprocess exited with error")
		return fmt.Errorf("running %s: %w", command[0], err)
	}
	log.Debug().Msg("mcp local: subprocess exited cleanly")
	return nil
}

// PumpStdio drives the stdio shim for local mode. It writes firstLine
// to stdinPipe, then concurrently copies reader -> stdinPipe and
// stdoutPipe -> stdout. The function returns when stdoutPipe reaches
// EOF (the child closed its stdout, typically because the child
// exited). The stdin pump goroutine may still be blocked on reader at
// that point; the operating system reaps it when the process exits.
//
// PumpStdio does NOT call cmd.Wait. The caller invokes cmd.Wait after
// PumpStdio returns so the exec contract for StdoutPipe is satisfied
// (the pipe must be drained before Wait).
func PumpStdio(
	stdinPipe io.WriteCloser,
	stdoutPipe io.Reader,
	reader *bufio.Reader,
	firstLine []byte,
	stdout io.Writer,
) error {
	go func() {
		defer stdinPipe.Close() //nolint:errcheck // child observes EOF on stdin
		if len(firstLine) > 0 {
			if _, err := stdinPipe.Write(firstLine); err != nil {
				log.Debug().Err(err).Msg("mcp local: writing buffered first line to child failed")
				return
			}
		}
		if _, err := io.Copy(stdinPipe, reader); err != nil {
			log.Debug().Err(err).Msg("mcp local: copying parent stdin to child failed")
		}
	}()
	if _, err := io.Copy(stdout, stdoutPipe); err != nil {
		return fmt.Errorf("copying child stdout: %w", err)
	}
	return nil
}
