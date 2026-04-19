// Package log provides a thin wrapper around zerolog for structured logging.
// Call Init() once at startup; pass the io.Writer from sdk.NewLogWriter.
// All other packages use the module-level functions (Info, Debug, Warn, Error).
package log

import (
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

var globalLogger atomic.Pointer[zerolog.Logger]

// Init configures the global logger. Must be called before any logging.
// w is the destination writer (from sdk.NewLogWriter); nil defaults to os.Stdout.
func Init(w io.Writer, level, format string) {
	if w == nil {
		w = os.Stdout
	}
	var wr io.Writer
	if format == "json" {
		wr = w
	} else {
		wr = zerolog.ConsoleWriter{Out: w, TimeFormat: time.RFC3339}
	}
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	l := zerolog.New(wr).Level(lvl).With().Timestamp().Logger()
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
