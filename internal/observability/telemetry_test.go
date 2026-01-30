package observability

import (
	"context"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.Exporter != "none" {
		t.Errorf("expected default exporter 'none', got %q", cfg.Exporter)
	}
	if cfg.ServiceName != "sblite" {
		t.Errorf("expected default service name 'sblite', got %q", cfg.ServiceName)
	}
	if cfg.SampleRate != 0.1 {
		t.Errorf("expected default sample rate 0.1, got %f", cfg.SampleRate)
	}
}

func TestConfigWithExporter(t *testing.T) {
	cfg := NewConfig()
	cfg.Exporter = "stdout"

	if !cfg.ShouldEnable() {
		t.Error("expected ShouldEnable to return true with exporter")
	}
}

func TestTelemetryInitDisabled(t *testing.T) {
	ctx := context.Background()
	cfg := NewConfig()
	cfg.Exporter = "none"

	_, cleanup, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleanup == nil {
		t.Error("expected cleanup function to be returned")
	}
	cleanup()
}

func TestTelemetryInitStdout(t *testing.T) {
	ctx := context.Background()
	cfg := NewConfig()
	cfg.Exporter = "stdout"
	cfg.MetricsEnabled = true
	cfg.TracesEnabled = true

	tel, cleanup, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tel == nil {
		t.Fatal("expected telemetry to be returned")
	}
	if tel.TracerProvider() == nil {
		t.Error("expected tracer provider")
	}
	if tel.MeterProvider() == nil {
		t.Error("expected meter provider")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tel.Shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
	cleanup()
}
