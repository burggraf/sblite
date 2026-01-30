package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Test that InitMetrics creates valid metric instruments
func TestInitMetrics(t *testing.T) {
	// Create a simple meter provider for testing
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	metrics, err := InitMetrics(mp)
	if err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}

	if metrics.HTTPRequestCount == nil {
		t.Error("expected HTTPRequestCount to be initialized")
	}

	if metrics.HTTPRequestDuration == nil {
		t.Error("expected HTTPRequestDuration to be initialized")
	}

	if metrics.HTTPResponseSize == nil {
		t.Error("expected HTTPResponseSize to be initialized")
	}
}

// Test that metrics are recorded
func TestMetricsRecording(t *testing.T) {
	// Create a manual reader to collect metrics
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	metrics, err := InitMetrics(mp)
	if err != nil {
		t.Fatalf("failed to init metrics: %v", err)
	}

	ctx := context.Background()

	// Record some metrics
	metrics.HTTPRequestCount.Add(ctx, 1,
		metric.WithAttributes(AttrHTTPMethod.String("GET")),
		metric.WithAttributes(AttrHTTPStatusCode.Int(200)),
	)

	metrics.HTTPRequestDuration.Record(ctx, 42.0,
		metric.WithAttributes(AttrHTTPMethod.String("GET")),
		metric.WithAttributes(AttrHTTPStatusCode.Int(200)),
	)

	metrics.HTTPResponseSize.Record(ctx, 1024,
		metric.WithAttributes(AttrHTTPMethod.String("GET")),
		metric.WithAttributes(AttrHTTPStatusCode.Int(200)),
	)

	// Collect and verify metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	if err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("expected scope metrics to be recorded")
	}

	sm := rm.ScopeMetrics[0]
	if len(sm.Metrics) == 0 {
		t.Fatal("expected metrics to be recorded")
	}

	// Check that we have the expected metrics
	metricNames := make(map[string]bool)
	for _, m := range sm.Metrics {
		metricNames[m.Name] = true
	}

	if !metricNames["http.server.request_count"] {
		t.Error("expected request_count metric to be recorded")
	}

	if !metricNames["http.server.request_duration"] {
		t.Error("expected request_duration metric to be recorded")
	}

	if !metricNames["http.server.response_size"] {
		t.Error("expected response_size metric to be recorded")
	}
}
