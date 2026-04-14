// Package log provides a thin wrapper around zerolog for structured logging.
// Call Init() once at startup with the config from the YAML file.
// All other packages import this package and use the module-level functions
// (Info, Debug, Warn, Error) which delegate to the global logger.
package log

import (
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Config holds logging configuration loaded from the YAML config file.
type Config struct {
	// Level is the minimum log level: "debug", "info", "warn", "error".
	Level string
	// Format is the output format: "text" (human-readable) or "json".
	Format string
	// Output is the destination writer. Defaults to os.Stdout.
	Output io.Writer
}

var globalLogger atomic.Pointer[zerolog.Logger]

// Init configures the global logger. Must be called before any logging.
func Init(cfg Config) {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	var w io.Writer
	if cfg.Format == "json" {
		w = out
	} else {
		w = zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
	}

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	l := zerolog.New(w).Level(level).With().Timestamp().Logger()
	globalLogger.Store(&l)
}

// Debug returns a debug-level log event.
func Debug() *zerolog.Event { return globalLogger.Load().Debug() }

// Info returns an info-level log event.
func Info() *zerolog.Event { return globalLogger.Load().Info() }

// Warn returns a warn-level log event.
func Warn() *zerolog.Event { return globalLogger.Load().Warn() }

// Error returns an error-level log event.
func Error() *zerolog.Event { return globalLogger.Load().Error() }

// With returns the logger with additional context fields.
func With() zerolog.Context { return globalLogger.Load().With() }
