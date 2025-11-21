package telemetry

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics exposes strongly typed metric instruments used across the adapter.
type Metrics struct {
	HTTPRequestsTotal    metric.Int64Counter
	HTTPRequestDuration  metric.Float64Histogram
	HTTPRequestsInFlight metric.Int64UpDownCounter
	HTTPRequestSize      metric.Int64Histogram
	HTTPResponseSize     metric.Int64Histogram

	StepExecutionDuration metric.Float64Histogram
	StepExecutionTotal    metric.Int64Counter
	StepErrorsTotal       metric.Int64Counter

	PluginExecutionDuration metric.Float64Histogram
	PluginErrorsTotal       metric.Int64Counter

	BecknMessagesTotal        metric.Int64Counter
	SignatureValidationsTotal metric.Int64Counter
	SchemaValidationsTotal    metric.Int64Counter
	CacheOperationsTotal      metric.Int64Counter
	CacheHitsTotal            metric.Int64Counter
	CacheMissesTotal          metric.Int64Counter
	RoutingDecisionsTotal     metric.Int64Counter
}

var (
	metricsInstance *Metrics
	metricsOnce     sync.Once
	metricsErr      error
)

// Attribute keys shared across instruments.
var (
	AttrModule        = attribute.Key("module")
	AttrSubsystem     = attribute.Key("subsystem")
	AttrName          = attribute.Key("name")
	AttrStep          = attribute.Key("step")
	AttrRole          = attribute.Key("role")
	AttrAction        = attribute.Key("action")
	AttrHTTPMethod    = attribute.Key("http_method")
	AttrHTTPStatus    = attribute.Key("http_status_code")
	AttrStatus        = attribute.Key("status")
	AttrErrorType     = attribute.Key("error_type")
	AttrPluginID      = attribute.Key("plugin_id")
	AttrPluginType    = attribute.Key("plugin_type")
	AttrOperation     = attribute.Key("operation")
	AttrRouteType     = attribute.Key("route_type")
	AttrTargetType    = attribute.Key("target_type")
	AttrSchemaVersion = attribute.Key("schema_version")
)

// GetMetrics lazily initializes instruments and returns a cached reference.
func GetMetrics(ctx context.Context) (*Metrics, error) {
	metricsOnce.Do(func() {
		metricsInstance, metricsErr = newMetrics()
	})
	return metricsInstance, metricsErr
}

func newMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter(
		"github.com/beckn-one/beckn-onix/telemetry",
		metric.WithInstrumentationVersion("1.0.0"),
	)

	m := &Metrics{}
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

	if m.StepExecutionDuration, err = meter.Float64Histogram(
		"onix_step_execution_duration_seconds",
		metric.WithDescription("Duration of individual processing steps"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5),
	); err != nil {
		return nil, fmt.Errorf("onix_step_execution_duration_seconds: %w", err)
	}

	if m.StepExecutionTotal, err = meter.Int64Counter(
		"onix_step_executions_total",
		metric.WithDescription("Total processing step executions"),
		metric.WithUnit("{execution}"),
	); err != nil {
		return nil, fmt.Errorf("onix_step_executions_total: %w", err)
	}

	if m.StepErrorsTotal, err = meter.Int64Counter(
		"onix_step_errors_total",
		metric.WithDescription("Processing step errors"),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, fmt.Errorf("onix_step_errors_total: %w", err)
	}

	if m.PluginExecutionDuration, err = meter.Float64Histogram(
		"onix_plugin_execution_duration_seconds",
		metric.WithDescription("Plugin execution time"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1),
	); err != nil {
		return nil, fmt.Errorf("onix_plugin_execution_duration_seconds: %w", err)
	}

	if m.PluginErrorsTotal, err = meter.Int64Counter(
		"onix_plugin_errors_total",
		metric.WithDescription("Plugin level errors"),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, fmt.Errorf("onix_plugin_errors_total: %w", err)
	}

	if m.BecknMessagesTotal, err = meter.Int64Counter(
		"beckn_messages_total",
		metric.WithDescription("Total Beckn protocol messages processed"),
		metric.WithUnit("{message}"),
	); err != nil {
		return nil, fmt.Errorf("beckn_messages_total: %w", err)
	}

	if m.SignatureValidationsTotal, err = meter.Int64Counter(
		"beckn_signature_validations_total",
		metric.WithDescription("Signature validation attempts"),
		metric.WithUnit("{validation}"),
	); err != nil {
		return nil, fmt.Errorf("beckn_signature_validations_total: %w", err)
	}

	if m.SchemaValidationsTotal, err = meter.Int64Counter(
		"beckn_schema_validations_total",
		metric.WithDescription("Schema validation attempts"),
		metric.WithUnit("{validation}"),
	); err != nil {
		return nil, fmt.Errorf("beckn_schema_validations_total: %w", err)
	}

	if m.CacheOperationsTotal, err = meter.Int64Counter(
		"onix_cache_operations_total",
		metric.WithDescription("Redis cache operations"),
		metric.WithUnit("{operation}"),
	); err != nil {
		return nil, fmt.Errorf("onix_cache_operations_total: %w", err)
	}

	if m.CacheHitsTotal, err = meter.Int64Counter(
		"onix_cache_hits_total",
		metric.WithDescription("Redis cache hits"),
		metric.WithUnit("{hit}"),
	); err != nil {
		return nil, fmt.Errorf("onix_cache_hits_total: %w", err)
	}

	if m.CacheMissesTotal, err = meter.Int64Counter(
		"onix_cache_misses_total",
		metric.WithDescription("Redis cache misses"),
		metric.WithUnit("{miss}"),
	); err != nil {
		return nil, fmt.Errorf("onix_cache_misses_total: %w", err)
	}

	if m.RoutingDecisionsTotal, err = meter.Int64Counter(
		"onix_routing_decisions_total",
		metric.WithDescription("Routing decisions taken by handler"),
		metric.WithUnit("{decision}"),
	); err != nil {
		return nil, fmt.Errorf("onix_routing_decisions_total: %w", err)
	}

	return m, nil
}
