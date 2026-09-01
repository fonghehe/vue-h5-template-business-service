// Package logging configures the structured logger used across the service.
//
// Production emits newline-delimited JSON so that log shippers can index
// fields without brittle regex rules. Local development emits human readable
// text to keep the feedback loop comfortable.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New builds a logger for the requested level and format.
// level is one of debug, info, warn, error; format is json or text.
// Unrecognised values fall back to info/json rather than failing startup.
func New(level, format string) *slog.Logger {
	handler := buildHandler(level, format)
	return slog.New(handler)
}

func buildHandler(level, format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	if strings.EqualFold(format, "text") {
		return slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.NewJSONHandler(os.Stdout, opts)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
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
