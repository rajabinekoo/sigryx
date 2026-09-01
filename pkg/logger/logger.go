// Package logger wraps log/slog with a single constructor so every service
// produces structured logs in the same shape (JSON, levelled, with source).
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config is the minimal set of knobs every service needs.
// Wire it from the service's own config struct.
type Config struct {
	ServiceName string // attached to every record as service=<name>; empty skips
	Level       string // debug | info | warn | error
	Format      string // json | text
	Source      bool   // include file:line in records
}

// New returns a slog.Logger configured per cfg, writing to stdout.
// Pass an explicit io.Writer via NewWithWriter when stdout is wrong (tests).
func New(cfg Config) *slog.Logger {
	return NewWithWriter(cfg, os.Stdout)
}

// NewWithWriter is the testable form of New.
func NewWithWriter(cfg Config, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     parseLevel(cfg.Level),
		AddSource: cfg.Source,
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	log := slog.New(handler)
	if cfg.ServiceName != "" {
		log = log.With(slog.String("service", cfg.ServiceName))
	}
	return log
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
