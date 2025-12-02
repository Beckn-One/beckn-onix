package telemetry

import (
	"context"

	clientprom "github.com/prometheus/client_golang/prometheus"
	clientpromhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewTestProvider creates a minimal telemetry provider for testing purposes.
// This avoids import cycles by not depending on the otelsetup package.
func NewTestProvider(ctx context.Context) (*Provider, error) {
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "test-service"),
			attribute.String("service.version", "test"),
			attribute.String("deployment.environment", "test"),
		),
	)
	if err != nil {
		return nil, err
	}

	registry := clientprom.NewRegistry()
	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutUnits(),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)

	return &Provider{
		MeterProvider:  meterProvider,
		MetricsHandler: clientpromhttp.HandlerFor(registry, clientpromhttp.HandlerOpts{}),
		Shutdown: func(ctx context.Context) error {
			return meterProvider.Shutdown(ctx)
		},
	}, nil
}
