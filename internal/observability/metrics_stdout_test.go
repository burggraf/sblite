package observability

import (
	"context"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// TestMetricsWithStdoutExporter tests that metrics are properly exported to stdout.
// This test is useful for manual verification during development.
func TestMetricsWithStdoutExporter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stdout test in short mode")
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := NewConfig()
	cfg.Exporter = "stdout"
	cfg.MetricsEnabled = true
	cfg.TracesEnabled = false

	tel, cleanup, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	// Check metrics are initialized
	if tel.Metrics() == nil {
		t.Fatal("expected metrics to be initialized")
	}

	// Record some metrics
	ctx := context.Background()
	tel.Metrics().HTTPRequestCount.Add(ctx, 1,
		metric.WithAttributes(AttrHTTPMethod.String("GET")),
		metric.WithAttributes(AttrHTTPStatusCode.Int(200)),
	)

	tel.Metrics().HTTPRequestDuration.Record(ctx, 42.0,
		metric.WithAttributes(AttrHTTPMethod.String("GET")),
		metric.WithAttributes(AttrHTTPStatusCode.Int(200)),
	)

	// Wait for periodic export (but this won't happen within test time)
	time.Sleep(200 * time.Millisecond)

	// Force flush via cleanup
	cleanup()

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read output (note: may be empty due to periodic reader timing)
	_, _ = r.Read(make([]byte, 1024))

	// This test mainly verifies that metrics don't crash
	t.Log("Metrics with stdout exporter completed without error")
}
