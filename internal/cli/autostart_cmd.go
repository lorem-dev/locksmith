package cli

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
)

// daemonProbeTimeout is how long _autostart waits when probing the daemon socket.
const daemonProbeTimeout = 200 * time.Millisecond

// newAutostartCmd returns the hidden locksmith _autostart command.
// It is called from shell rc files: it starts the daemon only if it is not
// already running, and always exits 0 so shell sessions never fail because
// of autostart errors.
func newAutostartCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "_autostart",
		Short:  "Start the daemon if not already running (used by shell hooks)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			socketPath := config.ExpandPath("~/.config/locksmith/locksmith.sock")
			if env := os.Getenv("LOCKSMITH_SOCKET"); env != "" {
				socketPath = env
			}

			// Probe the socket. If the dial succeeds the daemon is alive.
			conn, err := net.DialTimeout( //nolint:gosec // G704: local Unix socket, not a network SSRF
				"unix", socketPath, daemonProbeTimeout,
			)
			if err == nil {
				conn.Close() //nolint:errcheck // probe connection; error not actionable
				return nil   // daemon already running
			}

			// Daemon not running: spawn it in the background.
			binary, err := os.Executable()
			if err != nil {
				return nil // silently ignore
			}
			// Guard against recursive spawning when running inside a Go test binary.
			// os.Executable() returns the test binary (e.g. cli.test or a path under
			// /tmp/go-build…). Spawning it with "serve" would re-run the full test
			// suite instead of starting the daemon, causing exponential process growth.
			base := filepath.Base(binary)
			if strings.HasSuffix(base, ".test") || strings.Contains(binary, string(os.PathSeparator)+"go-build") {
				return nil
			}
			c := exec.Command(binary, "serve") //nolint:gosec // G204: binary is os.Executable(), not user input
			// Detach stdout/stderr/stdin: the daemon runs silently in the background.
			if err := c.Start(); err != nil {
				return nil // silently ignore
			}
			// Prevent zombie: reap the child if it exits before this process does.
			// In normal operation the daemon outlives _autostart and is adopted by
			// init, which reaps it. If it exits early (e.g. config error), this
			// goroutine collects the exit status before _autostart itself exits.
			go func() { _ = c.Wait() }() //nolint:errcheck // daemon exit status not actionable here
			// Give the daemon a moment to bind its socket before the shell continues.
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}
}
