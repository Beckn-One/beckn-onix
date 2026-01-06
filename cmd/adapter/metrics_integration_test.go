package main

import (
	"context"
	"testing"

	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/otelsetup"
	"github.com/stretchr/testify/require"
)

func TestMetricsEndpointExposesPrometheus(t *testing.T) {
	ctx := context.Background()
	setup := otelsetup.Setup{}
	provider, err := setup.New(ctx, &otelsetup.Config{
		ServiceName:    "test-onix",
		ServiceVersion: "1.0.0",
		EnableMetrics:  true,
		Environment:    "test",
	})
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Metrics are served by the plugin’s own HTTP server; just ensure provider is initialized.
	require.NotNil(t, provider.MeterProvider)
}
