package log_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/lorem-dev/locksmith/internal/log"
)

func TestInit_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "info", "text")
	log.Info().Str("key", "val").Msg("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("output %q missing message", buf.String())
	}
}

func TestInit_JSONFormat(t *testing.T) {
	origFieldName := zerolog.MessageFieldName
	zerolog.MessageFieldName = "message"
	defer func() { zerolog.MessageFieldName = origFieldName }()

	var buf bytes.Buffer
	log.Init(&buf, "info", "json")
	log.Info().Str("key", "val").Msg("hello")
	if !strings.Contains(buf.String(), `"message":"hello"`) {
		t.Errorf("output %q is not JSON with message field", buf.String())
	}
}

func TestInit_DebugLevelSuppressed(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "info", "text")
	log.Debug().Msg("should not appear")
	if buf.Len() > 0 {
		t.Errorf("debug message leaked at info level: %q", buf.String())
	}
}

func TestInit_DebugLevelVisible(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "debug", "text")
	log.Debug().Msg("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Error("debug message not visible at debug level")
	}
}

func TestInit_InvalidLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "bogus", "text")
	log.Info().Msg("ok")
	if buf.Len() == 0 {
		t.Error("expected output after init with invalid level")
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "warn", "text")
	log.Warn().Msg("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("warn message not visible at warn level")
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "error", "text")
	log.Error().Msg("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("error message not visible at error level")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	log.Init(&buf, "info", "json")
	derived := log.With().Str("component", "test").Logger()
	derived.Info().Msg("with context")
	if !strings.Contains(buf.String(), "component") {
		t.Errorf("output %q missing component field from With()", buf.String())
	}
}

func TestInit_DefaultOutput(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	log.Init(nil, "info", "text") // nil -> os.Stdout
	log.Info().Msg("default output test")
	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "default output test") {
		t.Errorf("expected output on stdout, got %q", buf.String())
	}
}
