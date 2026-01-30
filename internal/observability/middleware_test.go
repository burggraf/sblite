package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestMiddlewareWithDisabledTelemetry(t *testing.T) {
	cfg := NewConfig()
	cfg.Exporter = "none"

	tel, cleanup, _ := Init(nil, cfg)
	defer cleanup()

	middleware := HTTPMiddleware(tel, "test")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that tracer is noop
		span := trace.SpanFromContext(r.Context())
		if span.IsRecording() {
			t.Error("expected noop span when telemetry disabled")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMiddlewareWithEnabledTelemetry(t *testing.T) {
	cfg := NewConfig()
	cfg.Exporter = "stdout"
	cfg.TracesEnabled = true

	tel, cleanup, _ := Init(nil, cfg)
	defer cleanup()

	middleware := HTTPMiddleware(tel, "test")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that span is recording
		span := trace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			t.Error("expected recording span when telemetry enabled")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMiddlewareCapturesStatusCode(t *testing.T) {
	cfg := NewConfig()
	cfg.Exporter = "stdout"
	cfg.TracesEnabled = true

	tel, cleanup, _ := Init(nil, cfg)
	defer cleanup()

	middleware := HTTPMiddleware(tel, "test")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
