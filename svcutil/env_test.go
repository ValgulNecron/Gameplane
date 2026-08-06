package svcutil

import (
	"log/slog"
	"os"
	"testing"
)

func TestOr(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback string
		setup    func()
		want     string
	}{
		{
			name:     "unset key returns fallback",
			key:      "TEST_OR_UNSET",
			fallback: "default_value",
			setup:    func() { os.Unsetenv("TEST_OR_UNSET") },
			want:     "default_value",
		},
		{
			name:     "set key returns value",
			key:      "TEST_OR_SET",
			fallback: "default_value",
			setup: func() {
				os.Setenv("TEST_OR_SET", "actual_value")
			},
			want: "actual_value",
		},
		{
			name:     "empty but set key returns empty string",
			key:      "TEST_OR_EMPTY",
			fallback: "default_value",
			setup: func() {
				os.Setenv("TEST_OR_EMPTY", "")
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer os.Unsetenv(tt.key)

			got := Or(tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("Or(%q, %q) = %q; want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestOrInt(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		def   int
		setup func()
		want  int
	}{
		{
			name:  "unset key returns default",
			key:   "TEST_OR_INT_UNSET",
			def:   42,
			setup: func() { os.Unsetenv("TEST_OR_INT_UNSET") },
			want:  42,
		},
		{
			name: "valid integer returns parsed value",
			key:  "TEST_OR_INT_VALID",
			def:  42,
			setup: func() {
				os.Setenv("TEST_OR_INT_VALID", "123")
			},
			want: 123,
		},
		{
			name: "negative integer returns parsed value",
			key:  "TEST_OR_INT_NEGATIVE",
			def:  42,
			setup: func() {
				os.Setenv("TEST_OR_INT_NEGATIVE", "-999")
			},
			want: -999,
		},
		{
			name: "zero returns parsed value",
			key:  "TEST_OR_INT_ZERO",
			def:  42,
			setup: func() {
				os.Setenv("TEST_OR_INT_ZERO", "0")
			},
			want: 0,
		},
		{
			name: "invalid integer returns default",
			key:  "TEST_OR_INT_INVALID",
			def:  42,
			setup: func() {
				os.Setenv("TEST_OR_INT_INVALID", "not_a_number")
			},
			want: 42,
		},
		{
			name: "empty string returns default",
			key:  "TEST_OR_INT_EMPTY",
			def:  42,
			setup: func() {
				os.Setenv("TEST_OR_INT_EMPTY", "")
			},
			want: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer os.Unsetenv(tt.key)

			got := OrInt(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("OrInt(%q, %d) = %d; want %d", tt.key, tt.def, got, tt.want)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want slog.Level
	}{
		{
			name: "debug returns LevelDebug",
			s:    "debug",
			want: slog.LevelDebug,
		},
		{
			name: "DEBUG (uppercase) returns LevelDebug",
			s:    "DEBUG",
			want: slog.LevelDebug,
		},
		{
			name: "Debug (mixed case) returns LevelDebug",
			s:    "Debug",
			want: slog.LevelDebug,
		},
		{
			name: "warn returns LevelWarn",
			s:    "warn",
			want: slog.LevelWarn,
		},
		{
			name: "WARN (uppercase) returns LevelWarn",
			s:    "WARN",
			want: slog.LevelWarn,
		},
		{
			name: "error returns LevelError",
			s:    "error",
			want: slog.LevelError,
		},
		{
			name: "ERROR (uppercase) returns LevelError",
			s:    "ERROR",
			want: slog.LevelError,
		},
		{
			name: "info returns LevelInfo",
			s:    "info",
			want: slog.LevelInfo,
		},
		{
			name: "INFO (uppercase) returns LevelInfo",
			s:    "INFO",
			want: slog.LevelInfo,
		},
		{
			name: "empty string returns LevelInfo (default)",
			s:    "",
			want: slog.LevelInfo,
		},
		{
			name: "unknown value returns LevelInfo (default)",
			s:    "unknown",
			want: slog.LevelInfo,
		},
		{
			name: "typo value returns LevelInfo (default)",
			s:    "debugg",
			want: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLogLevel(tt.s)
			if got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v; want %v", tt.s, got, tt.want)
			}
		})
	}
}
