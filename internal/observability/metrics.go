package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metrics holds common metric instruments.
type Metrics struct {
	// HTTP server metrics
	HTTPRequestCount    metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram
	HTTPResponseSize    metric.Int64Histogram
}

// InitMetrics initializes and returns metric instruments.
func InitMetrics(mp metric.MeterProvider) (*Metrics, error) {
	meter := mp.Meter("sblite")

	m := &Metrics{}

	var err error
	m.HTTPRequestCount, err = meter.Int64Counter(
		"http.server.request_count",
		metric.WithDescription("Number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request count counter: %w", err)
	}

	m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http.server.request_duration",
		metric.WithDescription("HTTP request latency"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	m.HTTPResponseSize, err = meter.Int64Histogram(
		"http.server.response_size",
		metric.WithDescription("HTTP response size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create response size histogram: %w", err)
	}

	return m, nil
}

// initMeterProvider initializes the meter provider based on config.
func initMeterProvider(ctx context.Context, cfg *Config) (metric.MeterProvider, any, error) {
	var reader sdkmetric.Reader
	var err error

	switch cfg.Exporter {
	case "stdout":
		exporter, err := stdoutmetric.New(
			stdoutmetric.WithWriter(os.Stderr),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}
		reader = sdkmetric.NewPeriodicReader(exporter)
	case "otlp":
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, cfg.Endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to OTLP collector: %w", err)
		}

		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
		}
		reader = sdkmetric.NewPeriodicReader(exporter)
	case "none":
		return sdkmetric.NewMeterProvider(), nil, nil
	default:
		return nil, nil, fmt.Errorf("unknown exporter: %s", cfg.Exporter)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create metrics reader: %w", err)
	}

	// Create resource with service info
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("0.4.2"),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	return mp, reader, nil
}
