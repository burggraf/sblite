// internal/log/middleware_test.go
package log

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	handler := NewConsoleHandler(&buf, cfg, slog.LevelInfo)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Wrap with our middleware
	wrapped := RequestLogger(testHandler)

	// Make a request
	req := httptest.NewRequest("GET", "/test/path", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check log output
	output := buf.String()
	if !strings.Contains(output, "http request") {
		t.Errorf("expected log to contain 'http request', got %q", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("expected log to contain 'GET', got %q", output)
	}
	if !strings.Contains(output, "/test/path") {
		t.Errorf("expected log to contain '/test/path', got %q", output)
	}
	if !strings.Contains(output, "status=200") {
		t.Errorf("expected log to contain 'status=200', got %q", output)
	}
}

func TestRequestLogger_ErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{Format: "text"}
	handler := NewConsoleHandler(&buf, cfg, slog.LevelInfo)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	wrapped := RequestLogger(testHandler)

	req := httptest.NewRequest("GET", "/error", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	output := buf.String()
	if !strings.Contains(output, "level=ERROR") {
		t.Errorf("expected ERROR level for 500 status, got %q", output)
	}
}

func TestGetRequestID(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := GetRequestID(r.Context())
		if reqID == "" {
			t.Error("expected request ID in context")
		}
		if len(reqID) != 8 {
			t.Errorf("expected 8-char request ID, got %d chars", len(reqID))
		}
	})

	wrapped := RequestLogger(testHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)
}
