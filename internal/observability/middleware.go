package observability

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns middleware that instruments HTTP requests with OpenTelemetry.
// If telemetry is disabled, it acts as a pass-through middleware.
func HTTPMiddleware(tel *Telemetry, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

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

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	// WriteHeader may not be called explicitly, ensure status is set
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}
