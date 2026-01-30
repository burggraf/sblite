package observability

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns middleware that instruments HTTP requests with OpenTelemetry.
// If telemetry is disabled, it acts as a pass-through middleware.
func HTTPMiddleware(tel *Telemetry, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			tracer := tel.TracerProvider().Tracer(serviceName)

			// Build span attributes
			attrs := []attribute.KeyValue{
				AttrHTTPMethod.String(r.Method),
				AttrHTTPTarget.String(r.URL.Path),
				AttrHTTPScheme.String(r.URL.Scheme),
			}
			if r.Host != "" {
				attrs = append(attrs, AttrHTTPHost.String(r.Host))
			}
			if r.RemoteAddr != "" {
				attrs = append(attrs, AttrHTTPRemoteAddr.String(r.RemoteAddr))
			}

			// Create span name from method and route
			spanName := r.Method + " " + r.URL.Path

			// Start span
			ctx, span := tracer.Start(
				r.Context(),
				spanName,
				trace.WithAttributes(attrs...),
			)

			// Wrap response writer to capture status code and size
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Calculate duration
			duration := time.Since(start)

			// Record metrics
			if tel.Metrics() != nil {
				metrics := tel.Metrics()

				// Build attribute set for metrics
				attrs := []attribute.KeyValue{
					AttrHTTPMethod.String(r.Method),
					AttrHTTPStatusCode.Int(rw.status),
				}

				// Record request count
				metrics.HTTPRequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))

				// Record request duration
				metrics.HTTPRequestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

				// Record response size
				if rw.size > 0 {
					metrics.HTTPResponseSize.Record(ctx, int64(rw.size), metric.WithAttributes(attrs...))
				}

				// Store metrics to database for dashboard visualization
				timestamp := start.Unix()
				tags := fmt.Sprintf("http.method:%s,http.status_code:%d", r.Method, rw.status)
				go tel.StoreMetric(timestamp, "http.server.request_count", 1, tags)
				go tel.StoreMetric(timestamp, "http.server.request_duration_ms", float64(duration.Milliseconds()), tags)
			} else {
				// Metrics not initialized - skip recording
			}

			// Set span status based on HTTP status
			if rw.status >= 400 {
				span.SetStatus(codes.Error, http.StatusText(rw.status))
			} else {
				span.SetStatus(codes.Ok, "")
			}
			span.SetAttributes(AttrHTTPStatusCode.Int(rw.status))
			span.End()
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and size.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}
