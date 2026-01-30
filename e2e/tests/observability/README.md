# OpenTelemetry E2E Tests

End-to-end tests for OpenTelemetry integration in sblite.

## Overview

This directory contains comprehensive E2E tests for OpenTelemetry observability features in sblite, including:

- Configuration tests (CLI flags and environment variables)
- Metrics collection tests
- Distributed tracing tests

## Test Files

### `otel-configuration.test.ts`

Tests for OTel configuration via CLI flags and environment variables (15 tests):

- **CLI Flags**: Tests all `--otel-*` flags including exporter, service name, sample rate, and signal enable/disable
- **Environment Variables**: Tests `SBLITE_OTEL_*` environment variable configuration
- **Combined Configuration**: Tests combinations of configuration options
- **Priority**: Verifies CLI flags take priority over environment variables

### `otel-metrics.test.ts`

Tests for metrics collection and reporting (25 tests):

- **HTTP Request Metrics**: Request counting, method differentiation (GET, POST, PATCH, DELETE)
- **Request Duration Metrics**: Latency tracking, histogram buckets
- **Response Size Metrics**: Response byte tracking
- **Status Code Metrics**: Classification by 2xx, 4xx, 5xx status codes
- **Metric Labels and Attributes**: HTTP method, route, status code, endpoint type
- **Concurrent Request Metrics**: Handling parallel requests

### `otel-traces.test.ts`

Tests for distributed tracing (28 tests):

- **Span Creation**: Verifying spans are created for all HTTP methods
- **Span Attributes**: HTTP method, route, status code, URL, host, scheme
- **Span Status**: OK for success, ERROR for failures (4xx, 5xx)
- **Trace Sampling**: Configurable sampling rates (0%, 10%, 100%)
- **Trace Propagation**: traceparent header handling
- **Span Timing**: Start time, end time, duration accuracy
- **Different Endpoint Traces**: Auth, REST, storage, RPC endpoints

## Running Tests

### Run all observability tests

```bash
cd e2e
npm test -- tests/observability/
```

### Run specific test file

```bash
cd e2e
npm test -- tests/observability/otel-configuration.test.ts
npm test -- tests/observability/otel-metrics.test.ts
npm test -- tests/observability/otel-traces.test.ts
```

## Test Requirements

- sblite server must be running on `http://localhost:8080`
- Tests use the standard test database setup
- No external OTLP collector required (tests verify behavior, not export)

## What's Tested

### Configuration
- CLI flags for all OTel settings
- Environment variable configuration
- Service name customization
- Exporter selection (none, stdout, otlp)
- Sample rate configuration
- Individual signal enable/disable (metrics/traces)
- CLI flag priority over environment variables

### Metrics
- HTTP request count (counter)
- Request duration (histogram with p50, p95, p99)
- Response size (histogram)
- Status code classification
- Metric attributes (method, route, status_code)
- Concurrent request handling

### Traces
- Span creation for all HTTP methods
- Span attributes (Semantic Conventions)
- Span status based on HTTP status
- Trace sampling (configurable rates)
- Trace propagation (traceparent header)
- Span timing accuracy
- Different endpoint types (auth, REST, storage, RPC)

## Known Limitations

### E2E Test Scope

These tests verify system behavior that generates metrics and traces. They do not:

- Verify actual metric values (requires OTLP collector/prometheus)
- Inspect span data directly (requires trace collector like Tempo)
- Test performance overhead (requires benchmarking)
- Test OTLP export (requires real collector endpoint)

### Production Verification

In production, you would verify:

1. **Metrics** appear in Prometheus/Grafana with correct values
2. **Traces** appear in Tempo/Jaeger with proper span hierarchy
3. **Performance** meets the specified overhead guarantees

## Compatibility Summary

| Feature | Tests | Status |
|---------|-------|--------|
| Configuration (CLI) | 8 tests | Tested |
| Configuration (Env) | 5 tests | Tested |
| Configuration (Combined) | 4 tests | Tested |
| Request Metrics | 5 tests | Tested |
| Duration Metrics | 3 tests | Tested |
| Size Metrics | 3 tests | Tested |
| Status Metrics | 3 tests | Tested |
| Metric Labels | 6 tests | Tested |
| Concurrent Metrics | 2 tests | Tested |
| Span Creation | 5 tests | Tested |
| Span Attributes | 6 tests | Tested |
| Span Status | 4 tests | Tested |
| Trace Sampling | 3 tests | Tested |
| Trace Propagation | 3 tests | Tested |
| Span Timing | 4 tests | Tested |
| Endpoint Traces | 4 tests | Tested |

## See Also

- [OpenTelemetry Documentation](../../../docs/observability.md)
- [Main Project README](../../../README.md)
- [CLAUDE.md - Observability Section](../../../CLAUDE.md)
- [Implementation Plan](../../../docs/plans/2026-01-30-opentelemetry-implementation.md)
