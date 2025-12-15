package otelsetup

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	clientprom "github.com/prometheus/client_golang/prometheus"
	clientpromhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/plugin"
	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// Setup wires the telemetry provider. This is the concrete implementation
// behind the MetricsProvider interface.
type Setup struct{}

// Config represents OpenTelemetry related configuration.
type Config struct {
	ServiceName    string `yaml:"serviceName"`
	ServiceVersion string `yaml:"serviceVersion"`
	EnableMetrics  bool   `yaml:"enableMetrics"`
	Environment    string `yaml:"environment"`
	MetricsPort    string `yaml:"metricsPort"`
}

// DefaultConfig returns sensible defaults for telemetry configuration.
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "beckn-onix",
		ServiceVersion: "dev",
		EnableMetrics:  true,
		Environment:    "development",
		MetricsPort:    "9090",
	}
}

// ToPluginConfig converts Config to plugin.Config format.
func ToPluginConfig(cfg *Config) *plugin.Config {
	return &plugin.Config{
		ID: "otelsetup",
		Config: map[string]string{
			"serviceName":    cfg.ServiceName,
			"serviceVersion": cfg.ServiceVersion,
			"enableMetrics":  fmt.Sprintf("%t", cfg.EnableMetrics),
			"environment":    cfg.Environment,
			"metricsPort":    cfg.MetricsPort,
		},
	}
}

// New initializes the underlying telemetry provider. The returned provider
// exposes the HTTP handler and shutdown hooks that the core application can
// manage directly.
func (Setup) New(ctx context.Context, cfg *Config) (*telemetry.Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("telemetry config cannot be nil")
	}

	// Apply defaults if fields are empty
	if cfg.ServiceName == "" {
		cfg.ServiceName = DefaultConfig().ServiceName
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = DefaultConfig().ServiceVersion
	}
	if cfg.Environment == "" {
		cfg.Environment = DefaultConfig().Environment
	}
	if cfg.MetricsPort == "" {
		cfg.MetricsPort = DefaultConfig().MetricsPort
	}

	if !cfg.EnableMetrics {
		log.Info(ctx, "OpenTelemetry metrics disabled")
		return &telemetry.Provider{
			Shutdown: func(context.Context) error { return nil },
		}, nil
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry resource: %w", err)
	}

	registry := clientprom.NewRegistry()

	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutUnits(),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)
	log.Infof(ctx, "OpenTelemetry metrics initialized for service=%s version=%s env=%s",
		cfg.ServiceName, cfg.ServiceVersion, cfg.Environment)

	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(0)); err != nil {
		log.Warnf(ctx, "Failed to start Go runtime instrumentation: %v", err)
	}

	// Create metrics handler
	metricsHandler := clientpromhttp.HandlerFor(registry, clientpromhttp.HandlerOpts{})

	// Create and start metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricsHandler)

	metricsServer := &http.Server{
		Addr:         net.JoinHostPort("", cfg.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		log.Infof(ctx, "Metrics server listening on %s", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf(ctx, fmt.Errorf("metrics server ListenAndServe: %w", err), "error listening and serving metrics")
		}
	}()

	return &telemetry.Provider{
		MeterProvider: meterProvider,
		Shutdown: func(shutdownCtx context.Context) error {
			log.Infof(ctx, "Shutting down metrics server...")
			// Shutdown the metrics server
			serverShutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
			defer cancel()
			if err := metricsServer.Shutdown(serverShutdownCtx); err != nil {
				log.Errorf(ctx, fmt.Errorf("metrics server shutdown: %w", err), "error shutting down metrics server")
			}
			serverWg.Wait()
			// Shutdown the meter provider
			return meterProvider.Shutdown(shutdownCtx)
		},
	}, nil
}
