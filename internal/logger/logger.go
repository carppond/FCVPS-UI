// Package logger wraps log/slog with project-wide conventions:
//   - JSON encoder on stdout by default (text encoder available for dev).
//   - Optional file output with size/age based rotation (see rotate.go).
//   - A process-wide Default() logger plus per-component sub-loggers.
package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"shiguang-vps/internal/config"
)

// Options controls how Init builds the underlying slog.Logger.
//
// All fields are optional; zero values fall back to config defaults.
type Options struct {
	Level     string
	Format    string
	File      string
	MaxSizeMB int
	MaxAgeDay int
	Backups   int
}

// ErrInitFailed wraps the underlying error when Init cannot construct the
// configured logger (e.g. the rotated file cannot be opened).
var ErrInitFailed = errors.New("logger init failed")

var (
	defaultLogger *slog.Logger
	logFileCloser io.Closer
	initOnce      sync.Once
	initialised   bool
	initMu        sync.Mutex
)

// Init builds the global logger from Options. It is safe to call concurrently;
// only the first call wins. Subsequent calls return ErrAlreadyInitialised
// without mutating state.
func Init(opts Options) error {
	var initErr error
	initOnce.Do(func() {
		initMu.Lock()
		defer initMu.Unlock()
		logger, closer, err := build(opts)
		if err != nil {
			initErr = err
			return
		}
		defaultLogger = logger
		logFileCloser = closer
		initialised = true
	})
	if initErr != nil {
		return initErr
	}
	initMu.Lock()
	defer initMu.Unlock()
	if !initialised {
		// Should be unreachable; surfaced for safety.
		return fmt.Errorf("logger init: unknown failure")
	}
	return nil
}

// Default returns the process-wide logger. If Init has not been called yet,
// a no-op fallback writing to stderr at info level is returned so libraries
// loaded before main can still emit messages.
func Default() *slog.Logger {
	initMu.Lock()
	defer initMu.Unlock()
	if defaultLogger == nil {
		return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return defaultLogger
}

// WithComponent returns a sub-logger carrying a `component` attribute. Use it
// at the top of business packages, e.g. `log := logger.WithComponent("auth")`.
func WithComponent(name string) *slog.Logger {
	return Default().With(slog.String("component", name))
}

// Sync flushes any buffered file output and closes the rotated file. It is a
// no-op for the stdout-only configuration.
func Sync() error {
	initMu.Lock()
	defer initMu.Unlock()
	if logFileCloser == nil {
		return nil
	}
	err := logFileCloser.Close()
	logFileCloser = nil
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

// build creates the underlying slog.Logger and (optional) file rotator.
func build(opts Options) (*slog.Logger, io.Closer, error) {
	level := parseLevel(opts.Level)
	format := strings.ToLower(opts.Format)
	if format == "" {
		format = config.DefaultLogFormat
	}

	writers := []io.Writer{os.Stdout}
	var closer io.Closer
	if opts.File != "" {
		fileWriter, err := newRotator(opts.File, rotateOptions{
			MaxSizeMB:  fallbackInt(opts.MaxSizeMB, config.DefaultLogMaxSizeMB),
			MaxAgeDays: fallbackInt(opts.MaxAgeDay, config.DefaultLogMaxAgeDays),
			MaxBackups: fallbackInt(opts.Backups, config.DefaultLogMaxBackups),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %q: %w", opts.File, err)
		}
		writers = append(writers, fileWriter)
		closer = fileWriter
	}

	writer := io.MultiWriter(writers...)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(writer, handlerOpts)
	default:
		handler = slog.NewJSONHandler(writer, handlerOpts)
	}

	return slog.New(handler), closer, nil
}

// parseLevel maps the string config to slog.Level, defaulting to Info.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fallbackInt returns v if it is positive, otherwise fallback.
func fallbackInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// LogAttrError returns an slog attribute representing err. Returns nil-valued
// attribute when err is nil so callers can unconditionally append it.
func LogAttrError(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("err", err.Error())
}

// Reset is exposed for tests; it clears the singleton and closes any file.
// Production code MUST NOT call Reset.
func Reset() {
	initMu.Lock()
	defer initMu.Unlock()
	if logFileCloser != nil {
		_ = logFileCloser.Close()
	}
	defaultLogger = nil
	logFileCloser = nil
	initialised = false
	initOnce = sync.Once{}
}
