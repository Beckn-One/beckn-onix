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
// Note: Most metrics have been moved to their respective modules. Only plugin-level
// metrics remain here. See:
// - OTel setup: pkg/plugin/implementation/otelsetup
// - Step metrics: pkg/telemetry/step_metrics.go
// - Cache metrics: pkg/plugin/implementation/cache/metrics.go
// - Handler metrics: core/module/handler/metrics.go
type Metrics struct {
	PluginExecutionDuration metric.Float64Histogram
	PluginErrorsTotal       metric.Int64Counter
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

	return m, nil
}
