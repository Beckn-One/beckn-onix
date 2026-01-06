package definition

import (
	"context"

	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// OtelSetupMetricsProvider encapsulates initialization of OpenTelemetry metrics
// providers. Implementations wire exporters and return a Provider that the core
// application can manage.
type OtelSetupMetricsProvider interface {
	// New initializes a new telemetry provider instance with the given configuration.
	New(ctx context.Context, config map[string]string) (*telemetry.Provider, func() error, error)
}
