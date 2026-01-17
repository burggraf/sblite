// internal/log/console_test.go
package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestConsoleHandler_Text(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelInfo)

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected output to contain 'key=value', got %q", output)
	}
}

func TestConsoleHandler_JSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "json"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelInfo)

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("expected JSON output with msg field, got %q", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("expected JSON output with key field, got %q", output)
	}
}

func TestConsoleHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	h := NewConsoleHandler(&buf, cfg, slog.LevelWarn)

	logger := slog.New(h)
	logger.Info("should not appear")
	logger.Warn("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("info message should be filtered out at warn level")
	}
	if !strings.Contains(output, "should appear") {
		t.Errorf("warn message should appear")
	}
}
