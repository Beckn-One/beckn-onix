package definition

import (
	"context"

	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// MetricsProvider encapsulates initialization of OpenTelemetry metrics
// providers. Implementations wire exporters and return a Provider that the core
// application can manage.
type MetricsProvider interface {
	// New initializes a new telemetry provider instance with the given configuration.
	New(ctx context.Context, config map[string]string) (*telemetry.Provider, func() error, error)
}
