# Observability with OpenTelemetry

sblite includes production-grade observability using OpenTelemetry for metrics, distributed tracing, and log correlation.

## Quick Start

```bash
# Development: see traces and metrics in stdout
./sblite serve --otel-exporter stdout --otel-sample-rate 1.0

# Production: send to Grafana/Tempo
./sblite serve --otel-exporter otlp --otel-endpoint grafana:4317

# View metrics and traces in the dashboard
./sblite serve --otel-exporter stdout
# Then open http://localhost:8080/_ and navigate to Observability
```

## Features

| Feature | Description |
|---------|-------------|
| **Metrics** | HTTP request rate, latency histograms (p50/p95/p99), response sizes |
| **Traces** | Distributed tracing with automatic HTTP instrumentation |
| **Dashboard UI** | Built-in web interface for viewing metrics and traces |
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

## Dashboard Observability

When OpenTelemetry is enabled, the web dashboard (`/_`) includes an **Observability** section that provides real-time visibility into your application's performance.

### Accessing Observability

1. Start sblite with OTel enabled:
   ```bash
   ./sblite serve --otel-exporter stdout
   ```

2. Navigate to `http://localhost:8080/_`

3. Click **Observability** in the sidebar

### Dashboard Features

| Feature | Description |
|---------|-------------|
| **Status Overview** | Shows OTel configuration (exporter, sample rate, enabled features) |
| **Metrics Charts** | Time-series graphs for request rate, latency, response size |
| **Traces List** | Recent HTTP requests with method, path, status code, duration |
| **Filters** | Filter traces by method (GET/POST/...), path, status code |
| **Auto-Refresh** | Toggle automatic data refresh every 5 seconds |

### API Endpoints

The dashboard observability feature exposes these API endpoints:

| Endpoint | Description |
|----------|-------------|
| `GET /_/api/observability/status` | Get OTel configuration and status |
| `GET /_/api/observability/metrics` | Get time-series metrics (supports `?minutes=N`) |
| `GET /_/api/observability/traces` | Get recent traces (supports `?method`, `?path`, `?status` filters) |

### Data Storage

Metrics are stored in the `_observability_metrics` table:

```sql
CREATE TABLE _observability_metrics (
    timestamp INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    value REAL,
    tags TEXT,
    PRIMARY KEY (timestamp, metric_name, tags)
);
```

### Example Dashboard Usage

```javascript
// Fetch observability status
const status = await fetch('/_/api/observability/status').then(r => r.json())
// { enabled: true, exporter: "stdout", sampleRate: 0.1, ... }

// Fetch metrics for last 5 minutes
const metrics = await fetch('/_/api/observability/metrics?minutes=5').then(r => r.json())

// Fetch traces filtered by GET method
const traces = await fetch('/_/api/observability/traces?method=GET').then(r => r.json())
```
