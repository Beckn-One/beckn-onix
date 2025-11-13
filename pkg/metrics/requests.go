package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

var (
	// Inbound request metrics
	inboundRequestsTotal         otelmetric.Int64Counter
	inboundSignValidationTotal   otelmetric.Int64Counter
	inboundSchemaValidationTotal otelmetric.Int64Counter

	// Outbound request metrics
	outboundRequestsTotal   otelmetric.Int64Counter
	outboundRequests2XX     otelmetric.Int64Counter
	outboundRequests4XX     otelmetric.Int64Counter
	outboundRequests5XX     otelmetric.Int64Counter
	outboundRequestDuration otelmetric.Float64Histogram
)

// InitRequestMetrics initializes request-related metrics instruments.
func InitRequestMetrics() error {
	if !IsEnabled() {
		return nil
	}

	meter := GetMeter()
	var err error

	// Inbound request metrics
	inboundRequestsTotal, err = meter.Int64Counter(
		"beckn.inbound.requests.total",
		otelmetric.WithDescription("Total number of inbound requests per host"),
	)
	if err != nil {
		return err
	}

	inboundSignValidationTotal, err = meter.Int64Counter(
		"beckn.inbound.sign_validation.total",
		otelmetric.WithDescription("Total number of inbound requests with sign validation per host"),
	)
	if err != nil {
		return err
	}

	inboundSchemaValidationTotal, err = meter.Int64Counter(
		"beckn.inbound.schema_validation.total",
		otelmetric.WithDescription("Total number of inbound requests with schema validation per host"),
	)
	if err != nil {
		return err
	}

	// Outbound request metrics
	outboundRequestsTotal, err = meter.Int64Counter(
		"beckn.outbound.requests.total",
		otelmetric.WithDescription("Total number of outbound requests per host"),
	)
	if err != nil {
		return err
	}

	outboundRequests2XX, err = meter.Int64Counter(
		"beckn.outbound.requests.2xx",
		otelmetric.WithDescription("Total number of outbound requests with 2XX status code per host"),
	)
	if err != nil {
		return err
	}

	outboundRequests4XX, err = meter.Int64Counter(
		"beckn.outbound.requests.4xx",
		otelmetric.WithDescription("Total number of outbound requests with 4XX status code per host"),
	)
	if err != nil {
		return err
	}

	outboundRequests5XX, err = meter.Int64Counter(
		"beckn.outbound.requests.5xx",
		otelmetric.WithDescription("Total number of outbound requests with 5XX status code per host"),
	)
	if err != nil {
		return err
	}

	// Outbound request duration histogram (for p99, p95, p75)
	outboundRequestDuration, err = meter.Float64Histogram(
		"beckn.outbound.request.duration",
		otelmetric.WithDescription("Duration of outbound requests in milliseconds"),
		otelmetric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	return nil
}

// RecordInboundRequest records an inbound request.
func RecordInboundRequest(ctx context.Context, host string) {
	if inboundRequestsTotal == nil {
		return
	}
	inboundRequestsTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("host", host),
	))
}

// RecordInboundSignValidation records an inbound request with sign validation.
func RecordInboundSignValidation(ctx context.Context, host string) {
	if inboundSignValidationTotal == nil {
		return
	}
	inboundSignValidationTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("host", host),
	))
}

// RecordInboundSchemaValidation records an inbound request with schema validation.
func RecordInboundSchemaValidation(ctx context.Context, host string) {
	if inboundSchemaValidationTotal == nil {
		return
	}
	inboundSchemaValidationTotal.Add(ctx, 1, otelmetric.WithAttributes(
		attribute.String("host", host),
	))
}

// RecordOutboundRequest records an outbound request with status code and duration.
func RecordOutboundRequest(ctx context.Context, host string, statusCode int, duration time.Duration) {
	if outboundRequestsTotal == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("host", host),
		attribute.String("status_code", strconv.Itoa(statusCode)),
	}

	// Record total
	outboundRequestsTotal.Add(ctx, 1, otelmetric.WithAttributes(attrs...))

	// Record by status code category
	statusClass := statusCode / 100
	switch statusClass {
	case 2:
		outboundRequests2XX.Add(ctx, 1, otelmetric.WithAttributes(attrs...))
	case 4:
		outboundRequests4XX.Add(ctx, 1, otelmetric.WithAttributes(attrs...))
	case 5:
		outboundRequests5XX.Add(ctx, 1, otelmetric.WithAttributes(attrs...))
	}

	// Record duration for percentile calculations (p99, p95, p75)
	if outboundRequestDuration != nil {
		outboundRequestDuration.Record(ctx, float64(duration.Milliseconds()), otelmetric.WithAttributes(attrs...))
	}
}

// HTTPTransport wraps an http.RoundTripper to track outbound request metrics.
type HTTPTransport struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface and tracks metrics.
func (t *HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	host := req.URL.Host

	resp, err := t.Transport.RoundTrip(req)

	duration := time.Since(start)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	} else if err != nil {
		// Network error - treat as 5XX
		statusCode = 500
	}

	RecordOutboundRequest(req.Context(), host, statusCode, duration)

	return resp, err
}

// WrapHTTPTransport wraps an http.RoundTripper with metrics tracking.
func WrapHTTPTransport(transport http.RoundTripper) http.RoundTripper {
	if !IsEnabled() {
		return transport
	}
	return &HTTPTransport{Transport: transport}
}
