package sdk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig holds logging configuration for NewLogWriter.
type LogConfig struct {
	Level  string
	Format string
	File   string
}

var debugMode atomic.Bool

// NewLogWriter returns an io.Writer for the daemon logger.
// If cfg.File is non-empty, ~ is expanded and a lumberjack rotating file
// writer is returned (MaxAge=3 days, MaxSize=50 MB).
// If cfg.File is empty, os.Stdout is returned.
// Records the debug state read by IsDebug and MaskSessionId.
func NewLogWriter(cfg LogConfig) (io.Writer, error) {
	debugMode.Store(cfg.Level == "debug")
	if cfg.File == "" {
		return os.Stdout, nil
	}
	expanded := expandTilde(cfg.File)
	if err := os.MkdirAll(filepath.Dir(expanded), 0700); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	return &lumberjack.Logger{
		Filename:  expanded,
		MaxSize:   50,
		MaxAge:    3,
		Compress:  false,
		LocalTime: true,
	}, nil
}

// IsDebug reports whether debug-level logging is active.
func IsDebug() bool { return debugMode.Load() }

func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
