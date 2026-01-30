package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestMetricsIntegrationFull tests the full integration of metrics with stdout exporter.
func TestMetricsIntegrationFull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create manual reader to collect metrics
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Initialize metrics
	metrics, err := InitMetrics(mp)
	if err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	// Create telemetry with metrics
	cfg := NewConfig()
	cfg.Exporter = "none"
	cfg.MetricsEnabled = true
	cfg.TracesEnabled = false

	tel := &Telemetry{
		config:        cfg,
		meterProvider: mp,
		metrics:       metrics,
		meterReader:   reader,
	}

	// Create middleware
	middleware := HTTPMiddleware(tel, "test")

	// Create a handler
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, rec.Code)
		}
	}

	// Collect metrics manually
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	// Verify metrics were recorded
	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("expected scope metrics to be recorded")
	}

	sm := rm.ScopeMetrics[0]
	if len(sm.Metrics) == 0 {
		t.Fatal("expected metrics to be recorded")
	}

	// Check for expected metric names
	metricNames := make(map[string]bool)
	for _, m := range sm.Metrics {
		metricNames[m.Name] = true
		t.Logf("Recorded metric: %s", m.Name)
	}

	expectedMetrics := []string{
		"http.server.request_count",
		"http.server.request_duration",
		"http.server.response_size",
	}

	for _, name := range expectedMetrics {
		if !metricNames[name] {
			t.Errorf("expected metric %s to be recorded", name)
		}
	}
}
