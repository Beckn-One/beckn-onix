package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/sdk/metric"
)

// Provider holds references to telemetry components that need coordinated shutdown.
type Provider struct {
	MeterProvider  *metric.MeterProvider
	MetricsHandler http.Handler
	Shutdown       func(context.Context) error
}
