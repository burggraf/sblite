# Observability with OpenTelemetry

sblite includes production-grade observability using OpenTelemetry for metrics, distributed tracing, and log correlation.

## Quick Start

```bash
# Development: see traces and metrics in stdout
./sblite serve --otel-exporter stdout --otel-sample-rate 1.0

# Production: send to Grafana/Tempo
./sblite serve --otel-exporter otlp --otel-endpoint grafana:4317
```

## Features

| Feature | Description |
|---------|-------------|
| **Metrics** | HTTP request rate, latency histograms (p50/p95/p99), response sizes |
| **Traces** | Distributed tracing with automatic HTTP instrumentation |
| **Zero Overhead** | Completely disabled by default |
| **Standard Protocol** | OTLP compatible with Grafana, Jaeger, Prometheus |

## Configuration

### CLI Flags

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--otel-exporter` | `SBLITE_OTEL_EXPORTER` | `none` | Exporter type: `none`, `stdout`, `otlp` |
| `--otel-endpoint` | `SBLITE_OTEL_ENDPOINT` | `localhost:4317` | OTLP collector endpoint |
| `--otel-service-name` | `SBLITE_OTEL_SERVICE_NAME` | `sblite` | Service name |
| `--otel-sample-rate` | `SBLITE_OTEL_SAMPLE_RATE` | `0.1` | Trace sampling (0.0-1.0) |
| `--otel-metrics-enabled` | - | `true` | Enable metrics |
| `--otel-traces-enabled` | - | `true` | Enable traces |

## Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `http.server.request_count` | Counter | `{request}` | HTTP requests |
| `http.server.request_duration` | Histogram | `ms` | Request latency |
| `http.server.response_size` | Histogram | `By` | Response size |

All metrics include: `http.method`, `http.status_code`

## Performance Impact

| Configuration | CPU | Memory | Latency |
|---------------|-----|--------|---------|
| Disabled | 0% | 0 | 0ms |
| Metrics only | <1% | ~10MB | <0.1ms |
| Full (10% sample) | <1% | ~15MB | <0.5ms |

## Integrating with Your Code

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("my-component")
ctx, span := tracer.Start(ctx, "my-operation")
defer span.End()
```
