// internal/log/logger_test.go
package log

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Mode != "console" {
		t.Errorf("expected mode 'console', got %q", cfg.Mode)
	}
	if cfg.Level != "info" {
		t.Errorf("expected level 'info', got %q", cfg.Level)
	}
	if cfg.Format != "text" {
		t.Errorf("expected format 'text', got %q", cfg.Format)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  int // slog.Level value
	}{
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"invalid", 0}, // defaults to info
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if int(got) != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
