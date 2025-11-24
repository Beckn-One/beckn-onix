package otelmetrics

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// Middleware instruments inbound HTTP handlers with OpenTelemetry metrics.
type Middleware struct {
	metrics *HTTPMetrics
	enabled bool
}

// New constructs middleware based on plugin configuration.
func New(ctx context.Context, cfg map[string]string) (*Middleware, error) {
	enabled := cfg["enabled"] != "false"

	metrics, err := GetHTTPMetrics(ctx)
	if err != nil {
		log.Warnf(ctx, "OpenTelemetry metrics unavailable: %v", err)
	}

	return &Middleware{
		metrics: metrics,
		enabled: enabled,
	}, nil
}

// Handler returns an http.Handler middleware compatible with plugin expectations.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if !m.enabled || m.metrics == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		action := extractAction(r.URL.Path)
		module := r.Header.Get("X-Module-Name")
		role := r.Header.Get("X-Role")

		attrs := []attribute.KeyValue{
			telemetry.AttrModule.String(module),
			telemetry.AttrRole.String(role),
			telemetry.AttrAction.String(action),
			telemetry.AttrHTTPMethod.String(r.Method),
		}

		m.metrics.HTTPRequestsInFlight.Add(ctx, 1, metric.WithAttributes(attrs...))
		defer m.metrics.HTTPRequestsInFlight.Add(ctx, -1, metric.WithAttributes(attrs...))

		if r.ContentLength > 0 {
			m.metrics.HTTPRequestSize.Record(ctx, r.ContentLength, metric.WithAttributes(attrs...))
		}

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		status := "success"
		if rw.statusCode >= 400 {
			status = "error"
		}

		statusAttrs := append(attrs,
			telemetry.AttrHTTPStatus.Int(rw.statusCode),
			telemetry.AttrStatus.String(status),
		)

		m.metrics.HTTPRequestsTotal.Add(ctx, 1, metric.WithAttributes(statusAttrs...))
		m.metrics.HTTPRequestDuration.Record(ctx, duration, metric.WithAttributes(statusAttrs...))
		if rw.bytesWritten > 0 {
			m.metrics.HTTPResponseSize.Record(ctx, int64(rw.bytesWritten), metric.WithAttributes(statusAttrs...))
		}

		if isBecknAction(action) {
			m.metrics.BecknMessagesTotal.Add(ctx, 1,
				metric.WithAttributes(
					telemetry.AttrAction.String(action),
					telemetry.AttrRole.String(role),
					telemetry.AttrStatus.String(status),
				))
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

func extractAction(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "root"
	}
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func isBecknAction(action string) bool {
	actions := []string{
		"discover", "select", "init", "confirm", "status", "track",
		"cancel", "update", "rating", "support",
		"on_discover", "on_select", "on_init", "on_confirm", "on_status",
		"on_track", "on_cancel", "on_update", "on_rating", "on_support",
	}
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}
