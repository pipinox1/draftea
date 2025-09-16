package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Middleware injects telemetry into the request context
func Middleware(tel *Telemetry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Inject telemetry into context
			ctx := WithTelemetry(r.Context(), tel)
			r = r.WithContext(ctx)

			// Start tracing span for HTTP request
			ctx, span := StartSpan(ctx, "HTTP "+r.Method+" "+r.URL.Path,
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
					attribute.String("http.route", r.URL.Path),
					attribute.String("user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			// Create response writer wrapper to capture status code
			wrappedWriter := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // Default status code
			}

			// Process request
			next.ServeHTTP(wrappedWriter, r.WithContext(ctx))

			// Record metrics
			duration := time.Since(start)

			// Add response attributes to span
			span.SetAttributes(
				attribute.Int("http.status_code", wrappedWriter.statusCode),
				attribute.String("http.status_class", getStatusClass(wrappedWriter.statusCode)),
			)

			// Record HTTP metrics using generic functions
			RecordCounter(ctx, "http_requests_total", "Total HTTP requests", 1,
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.Int("status_code", wrappedWriter.statusCode),
				attribute.String("status_class", getStatusClass(wrappedWriter.statusCode)),
			)

			RecordHistogram(ctx, "http_request_duration_seconds", "HTTP request duration", duration.Seconds(),
				attribute.String("method", r.Method),
				attribute.String("path", r.URL.Path),
				attribute.String("status_class", getStatusClass(wrappedWriter.statusCode)),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

