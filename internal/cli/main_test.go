package cli_test

import (
	"io"
	"os"
	"testing"

	"github.com/lorem-dev/locksmith/internal/log"
)

// TestMain initialises the package-level zerolog logger so tests that exercise
// code paths emitting log records (notably initflow.ExtractBundled via
// `locksmith init`) do not panic on a nil global logger.
func TestMain(m *testing.M) {
	log.Init(io.Discard, "error", "text")
	os.Exit(m.Run())
}
