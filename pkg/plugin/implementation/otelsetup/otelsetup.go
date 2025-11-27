package otelsetup

import (
	"context"
	"fmt"

	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// Setup wires the telemetry provider using the shared telemetry package. This
// is the concrete implementation behind the MetricsProvider interface.
type Setup struct{}

// New initializes the underlying telemetry provider. The returned provider
// exposes the HTTP handler and shutdown hooks that the core application can
// manage directly.
func (Setup) New(ctx context.Context, cfg *telemetry.Config) (*telemetry.Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("telemetry config cannot be nil")
	}
	return telemetry.NewProvider(ctx, cfg)
}
