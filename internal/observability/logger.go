package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog with a structured API.
type Logger struct {
	inner *slog.Logger
}

// New creates a new structured logger.
func New(level slog.Level, format string, output io.Writer) *Logger {
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return &Logger{inner: slog.New(handler)}
}

// NewFromConfig creates a logger from string configuration.
func NewFromConfig(level, format string) (*Logger, error) {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	return New(l, format, os.Stderr), nil
}

// Info logs an info message with key-value pairs.
func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Error logs an error message with key-value pairs.
func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

// Debug logs a debug message with key-value pairs.
func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

// With returns a new logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{inner: l.inner.With(args...)}
}

// Context returns a logger stored in context, or the default.
func Context(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		return l
	}
	return Default()
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

type loggerKey struct{}

var defaultLogger = New(slog.LevelInfo, "text", os.Stderr)

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}
