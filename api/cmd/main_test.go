package main

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
		// Unknown values degrade to info rather than refusing to start.
		"":        slog.LevelInfo,
		"verbose": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLogLevel(in); got != want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseRoleMapping(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		wantErr bool
		want    []string
	}{
		{
			name:    "valid single group",
			csv:     "admin",
			wantErr: false,
			want:    []string{"admin"},
		},
		{
			name:    "valid multi-group",
			csv:     "group1,group2,group3",
			wantErr: false,
			want:    []string{"group1", "group2", "group3"},
		},
		{
			name:    "valid multi-group with surrounding whitespace",
			csv:     "  group1  ,  group2  , group3  ",
			wantErr: false,
			want:    []string{"group1", "group2", "group3"},
		},
		{
			name:    "empty/unset value accepted",
			csv:     "",
			wantErr: false,
			want:    nil,
		},
		{
			name:    "leading comma",
			csv:     ",group1",
			wantErr: true,
		},
		{
			name:    "trailing comma",
			csv:     "group1,",
			wantErr: true,
		},
		{
			name:    "doubled comma",
			csv:     "group1,,group2",
			wantErr: true,
		},
		{
			name:    "whitespace-only entry",
			csv:     "group1,  ,group2",
			wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRoleMapping(tt.csv)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRoleMapping(%q) error = %v, wantErr %v", tt.csv, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseRoleMapping(%q) = %v, want %v", tt.csv, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseRoleMapping(%q)[%d] = %q, want %q", tt.csv, i, got[i], tt.want[i])
					return
				}
			}
		})
	}
}
