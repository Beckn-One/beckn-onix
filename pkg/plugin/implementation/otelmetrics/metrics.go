package otelmetrics

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics exposes HTTP-related metric instruments.
type HTTPMetrics struct {
	HTTPRequestsTotal    metric.Int64Counter
	HTTPRequestDuration  metric.Float64Histogram
	HTTPRequestsInFlight metric.Int64UpDownCounter
	HTTPRequestSize      metric.Int64Histogram
	HTTPResponseSize     metric.Int64Histogram
	BecknMessagesTotal   metric.Int64Counter
}

var (
	httpMetricsInstance *HTTPMetrics
	httpMetricsOnce     sync.Once
	httpMetricsErr      error
)

// GetHTTPMetrics lazily initializes HTTP metric instruments and returns a cached reference.
func GetHTTPMetrics(ctx context.Context) (*HTTPMetrics, error) {
	httpMetricsOnce.Do(func() {
		httpMetricsInstance, httpMetricsErr = newHTTPMetrics()
	})
	return httpMetricsInstance, httpMetricsErr
}

func newHTTPMetrics() (*HTTPMetrics, error) {
	meter := otel.GetMeterProvider().Meter(
		"github.com/beckn-one/beckn-onix/otelmetrics",
		metric.WithInstrumentationVersion("1.0.0"),
	)

	m := &HTTPMetrics{}
	var err error

	if m.HTTPRequestsTotal, err = meter.Int64Counter(
		"http_server_requests_total",
		metric.WithDescription("Total number of HTTP requests processed"),
		metric.WithUnit("{request}"),
	); err != nil {
		return nil, fmt.Errorf("http_server_requests_total: %w", err)
	}

	if m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http_server_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	); err != nil {
		return nil, fmt.Errorf("http_server_request_duration_seconds: %w", err)
	}

	if m.HTTPRequestsInFlight, err = meter.Int64UpDownCounter(
		"http_server_requests_in_flight",
		metric.WithDescription("Number of HTTP requests currently being processed"),
		metric.WithUnit("{request}"),
	); err != nil {
		return nil, fmt.Errorf("http_server_requests_in_flight: %w", err)
	}

	if m.HTTPRequestSize, err = meter.Int64Histogram(
		"http_server_request_size_bytes",
		metric.WithDescription("Size of HTTP request payloads"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000),
	); err != nil {
		return nil, fmt.Errorf("http_server_request_size_bytes: %w", err)
	}

	if m.HTTPResponseSize, err = meter.Int64Histogram(
		"http_server_response_size_bytes",
		metric.WithDescription("Size of HTTP responses"),
		metric.WithUnit("By"),
		metric.WithExplicitBucketBoundaries(100, 1000, 10000, 100000, 1000000),
	); err != nil {
		return nil, fmt.Errorf("http_server_response_size_bytes: %w", err)
	}

	if m.BecknMessagesTotal, err = meter.Int64Counter(
		"beckn_messages_total",
		metric.WithDescription("Total Beckn protocol messages processed"),
		metric.WithUnit("{message}"),
	); err != nil {
		return nil, fmt.Errorf("beckn_messages_total: %w", err)
	}

	return m, nil
}
