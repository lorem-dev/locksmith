package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/lorem-dev/locksmith/internal/log"
)

// EnvMapping maps an environment variable name to a secret reference.
type EnvMapping struct {
	Var string
	Ref SecretRef
}

// Run resolves secrets lazily, spawns command with them injected as
// environment variables, and proxies stdio between the parent (locksmith)
// and the child until the child exits. Locksmith stays resident for the
// lifetime of the child.
//
// in is the source of MCP JSON-RPC traffic from the AI client (production
// callers pass os.Stdin). out is the destination for child stdout
// (production callers pass os.Stdout). The child inherits os.Stderr for
// diagnostics.
//
// If in closes before any non-empty line arrives, Run returns nil
// without invoking the fetcher or spawning the child.
func Run(
	ctx context.Context,
	fetcher SecretFetcher,
	envMappings []EnvMapping,
	command []string,
	in io.Reader,
	out io.Writer,
) error {
	if len(command) == 0 {
		return fmt.Errorf("no command specified")
	}
	reader := bufio.NewReader(in)

	firstLine, err := readFirstNonEmptyLine(reader)
	if err != nil {
		return err
	}
	if firstLine == nil {
		log.Debug().Msg("mcp local: stdin closed empty, child not spawned")
		return nil
	}
	// Restore the trailing newline stripped by readFirstNonEmptyLine so
	// the child receives the same byte sequence the AI client sent.
	firstLine = append(firstLine, '\n')

	log.Debug().
		Str("command", command[0]).
		Int("args", len(command)-1).
		Int("env_injections", len(envMappings)).
		Msg("mcp local: first message received, resolving secrets")

	environ := os.Environ()
	for _, m := range envMappings {
		log.Debug().
			Str("var", m.Var).
			Str("key_alias", m.Ref.KeyAlias).
			Str("vault", m.Ref.VaultName).
			Str("path", m.Ref.Path).
			Msg("mcp local: resolving env var")
		value, fetchErr := fetcher.Fetch(ctx, m.Ref)
		if fetchErr != nil {
			log.Debug().Err(fetchErr).Str("var", m.Var).Msg("mcp local: env fetch failed")
			return fmt.Errorf("resolving env %s: %w", m.Var, fetchErr)
		}
		log.Debug().Str("var", m.Var).Int("len", len(value)).Msg("mcp local: env resolved")
		environ = append(environ, m.Var+"="+value)
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...) //nolint:gosec // G204: command is user-provided config
	cmd.Env = environ
	cmd.Stderr = os.Stderr
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("opening child stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("opening child stdout pipe: %w", err)
	}
	log.Debug().Str("command", command[0]).Msg("mcp local: exec")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", command[0], err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)
	done := make(chan struct{})
	go ForwardSignals(cmd.Process, sigCh, done)

	pumpErr := PumpStdio(stdinPipe, stdoutPipe, reader, firstLine, out)
	waitErr := cmd.Wait()
	close(done)

	if pumpErr != nil {
		return pumpErr
	}
	if waitErr != nil {
		return fmt.Errorf("running %s: %w", command[0], waitErr)
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

// ForwardSignals relays SIGTERM and SIGINT (as delivered on sigCh) to
// proc and returns when done is closed. The caller is responsible for
// registering sigCh with signal.Notify in production code and closing
// done after cmd.Wait returns. Tests drive sigCh directly so the host
// signal handlers are not touched.
func ForwardSignals(proc *os.Process, sigCh <-chan os.Signal, done <-chan struct{}) {
	for {
		select {
		case sig, ok := <-sigCh:
			if !ok {
				return
			}
			if err := proc.Signal(sig); err != nil {
				log.Debug().Err(err).Stringer("signal", sig).Msg("mcp local: forwarding signal to child failed")
			}
		case <-done:
			return
		}
	}
}
