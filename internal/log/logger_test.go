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
	if cfg.BufferLines != 500 {
		t.Errorf("expected BufferLines 500, got %d", cfg.BufferLines)
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

func TestInit_CreatesBuffer(t *testing.T) {
	cfg := &Config{
		Mode:        "console",
		Level:       "info",
		BufferLines: 100,
	}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Log something
	Info("test buffer message")

	// Check buffer has content
	lines := GetBufferedLogs(10)
	if len(lines) == 0 {
		t.Error("expected buffered logs, got none")
	}
}

func TestInit_BufferDisabled(t *testing.T) {
	cfg := &Config{
		Mode:        "console",
		Level:       "info",
		BufferLines: 0, // disabled
	}
	if err := Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	lines := GetBufferedLogs(10)
	if lines != nil {
		t.Error("expected nil when buffer disabled")
	}
}
