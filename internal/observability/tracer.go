package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// initTracerProvider initializes the trace provider based on config.
func initTracerProvider(ctx context.Context, cfg *Config) (trace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	switch cfg.Exporter {
	case "stdout":
		exporter, err = stdouttrace.New(
			stdouttrace.WithWriter(os.Stderr),
			stdouttrace.WithPrettyPrint(),
		)
	case "otlp":
		// Create gRPC connection
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, cfg.Endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to OTLP collector: %w", err)
		}

		// Create OTLP exporter
		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "none":
		return trace.NewNoopTracerProvider(), nil
	default:
		return nil, fmt.Errorf("unknown exporter: %s", cfg.Exporter)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource with service info
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("0.4.2"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler based on config
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(cfg.SampleRate),
	)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	return tp, nil
}

// Common span attributes
var (
	AttrHTTPMethod     = attribute.Key("http.method")
	AttrHTTPRoute      = attribute.Key("http.route")
	AttrHTTPStatusCode = attribute.Key("http.status_code")
	AttrHTTPTarget     = attribute.Key("http.target")
	AttrHTTPScheme     = attribute.Key("http.scheme")
	AttrHTTPHost       = attribute.Key("http.host")
	AttrHTTPRemoteAddr = attribute.Key("http.remote_addr")
	AttrUserID         = attribute.Key("user.id")
	AttrUserRole       = attribute.Key("user.role")
	AttrDBName         = attribute.Key("db.name")
	AttrDBTable        = attribute.Key("db.table")
	AttrDBOperation    = attribute.Key("db.operation")
)
