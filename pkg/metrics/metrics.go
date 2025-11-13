package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	mp                 *metric.MeterProvider
	meter              otelmetric.Meter
	prometheusRegistry *prometheus.Registry
	once               sync.Once
	shutdownFunc       func(context.Context) error
	ErrInvalidExporter = errors.New("invalid metrics exporter type")
	ErrMetricsNotInit  = errors.New("metrics not initialized")
)

// ExporterType represents the type of metrics exporter.
type ExporterType string

const (
	// ExporterPrometheus exports metrics in Prometheus format.
	ExporterPrometheus ExporterType = "prometheus"
)

// Config represents the configuration for metrics.
type Config struct {
	Enabled        bool             `yaml:"enabled"`
	ExporterType   ExporterType     `yaml:"exporterType"`
	ServiceName    string           `yaml:"serviceName"`
	ServiceVersion string           `yaml:"serviceVersion"`
	Prometheus     PrometheusConfig `yaml:"prometheus"`
}

// PrometheusConfig represents Prometheus exporter configuration.
type PrometheusConfig struct {
	Port string `yaml:"port"`
	Path string `yaml:"path"`
}

// validate validates the metrics configuration.
func (c *Config) validate() error {
	if !c.Enabled {
		return nil
	}

	if c.ExporterType != ExporterPrometheus {
		return fmt.Errorf("%w: %s", ErrInvalidExporter, c.ExporterType)
	}

	if c.ServiceName == "" {
		c.ServiceName = "beckn-onix"
	}

	return nil
}

// InitMetrics initializes the OpenTelemetry metrics SDK.
func InitMetrics(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}

	var initErr error
	once.Do(func() {
		if initErr = cfg.validate(); initErr != nil {
			return
		}

		// Create resource with service information.
		attrs := []attribute.KeyValue{
			attribute.String("service.name", cfg.ServiceName),
		}
		if cfg.ServiceVersion != "" {
			attrs = append(attrs, attribute.String("service.version", cfg.ServiceVersion))
		}
		res, err := resource.New(
			context.Background(),
			resource.WithAttributes(attrs...),
		)
		if err != nil {
			initErr = fmt.Errorf("failed to create resource: %w", err)
			return
		}

		// Always create Prometheus exporter for /metrics endpoint
		// Create a custom registry for the exporter so we can use it for HTTP serving
		promRegistry := prometheus.NewRegistry()
		promExporter, err := otelprom.New(otelprom.WithRegisterer(promRegistry))
		if err != nil {
			initErr = fmt.Errorf("failed to create Prometheus exporter: %w", err)
			return
		}
		prometheusRegistry = promRegistry

		// Create readers based on configuration.
		var readers []metric.Reader

		// Always add Prometheus reader for /metrics endpoint
		readers = append(readers, promExporter)

		// Create meter provider with all readers
		opts := []metric.Option{
			metric.WithResource(res),
		}
		for _, reader := range readers {
			opts = append(opts, metric.WithReader(reader))
		}
		mp = metric.NewMeterProvider(opts...)

		// Set global meter provider.
		otel.SetMeterProvider(mp)

		// Create meter for this package.
		meter = mp.Meter("github.com/beckn-one/beckn-onix")

		// Store shutdown function.
		shutdownFunc = func(ctx context.Context) error {
			return mp.Shutdown(ctx)
		}
	})

	return initErr
}

// GetMeter returns the global meter instance.
func GetMeter() otelmetric.Meter {
	if meter == nil {
		// Return a no-op meter if not initialized.
		return otel.Meter("noop")
	}
	return meter
}

// Shutdown gracefully shuts down the metrics provider.
func Shutdown(ctx context.Context) error {
	if shutdownFunc == nil {
		return nil
	}
	return shutdownFunc(ctx)
}

// IsEnabled returns whether metrics are enabled.
func IsEnabled() bool {
	return mp != nil
}

// MetricsHandler returns the HTTP handler for the /metrics endpoint.
// Returns nil if metrics are not enabled.
func MetricsHandler() http.Handler {
	if prometheusRegistry == nil {
		return nil
	}
	// Use promhttp to serve the Prometheus registry
	return promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{})
}

// InitAllMetrics initializes all metrics subsystems.
// This includes request metrics and runtime metrics.
// Returns an error if any initialization fails.
func InitAllMetrics() error {
	if !IsEnabled() {
		return nil
	}

	if err := InitRequestMetrics(); err != nil {
		return fmt.Errorf("failed to initialize request metrics: %w", err)
	}
	if err := InitRuntimeMetrics(); err != nil {
		return fmt.Errorf("failed to initialize runtime metrics: %w", err)
	}

	return nil
}
