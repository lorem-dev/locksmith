package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/log"
)

func TestInit_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "text", Output: &buf})

	log.Info().Str("key", "val").Msg("hello")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Errorf("output %q missing message", out)
	}
}

func TestInit_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "json", Output: &buf})

	log.Info().Str("key", "val").Msg("hello")

	out := buf.String()
	if !strings.Contains(out, `"message":"hello"`) {
		t.Errorf("output %q is not JSON with message field", out)
	}
}

func TestInit_DebugLevelSuppressed(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "text", Output: &buf})

	log.Debug().Msg("should not appear")

	if buf.Len() > 0 {
		t.Errorf("debug message leaked at info level: %q", buf.String())
	}
}

func TestInit_DebugLevelVisible(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "debug", Format: "text", Output: &buf})

	log.Debug().Msg("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Error("debug message not visible at debug level")
	}
}

func TestInit_InvalidLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	// Should not panic
	log.Init(log.Config{Level: "bogus", Format: "text", Output: &buf})
	log.Info().Msg("ok")
	if buf.Len() == 0 {
		t.Error("expected output after init with invalid level")
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "warn", Format: "text", Output: &buf})

	log.Warn().Msg("warn message")

	if !strings.Contains(buf.String(), "warn message") {
		t.Error("warn message not visible at warn level")
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "error", Format: "text", Output: &buf})

	log.Error().Msg("error message")

	if !strings.Contains(buf.String(), "error message") {
		t.Error("error message not visible at error level")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "json", Output: &buf})

	// With() adds fields to the logger context; verify it returns a valid Context
	derived := log.With().Str("component", "test").Logger()
	derived.Info().Msg("with context")

	out := buf.String()
	if !strings.Contains(out, "component") {
		t.Errorf("output %q missing component field from With()", out)
	}
}

func TestInit_DefaultOutput(t *testing.T) {
	// Verify Init with nil Output does not panic (defaults to os.Stdout).
	log.Init(log.Config{Level: "info", Format: "text"})
	// If we reach here without panic, the test passes.
}
