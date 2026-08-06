// Package svcutil provides shared stdlib-only service helpers for environment
// parsing (Or, OrInt, ParseLogLevel) and graceful HTTP server shutdown (RunHTTP).
// Used across operator, api, agent, audit-syslog-bridge, and telemetry-receiver
// to reduce code duplication and enforce consistent startup and shutdown behavior.
package svcutil

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Or returns the value of the environment variable key if it is set (even to
// an empty string), otherwise returns the fallback. This matches the semantics
// of os.LookupEnv: an empty-but-set var is distinguishable from an unset one.
func Or(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// OrInt returns the value of the environment variable key parsed as an integer,
// or the default value def if the key is unset or the value is not a valid
// integer. Unparseable values degrade to the safe default rather than crashing
// startup — a typo in a manifest shouldn't take the service down.
func OrInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ParseLogLevel maps a log level string to a slog.Level. The following values
// are recognized (case-insensitive): debug, warn, error. Unknown values
// degrade to info — a typo in a manifest shouldn't crash the service.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
