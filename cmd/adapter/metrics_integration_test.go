package main

import (
	"context"
	"net/http/httptest"
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

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	provider.MetricsHandler.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "# HELP")
	require.Contains(t, body, "# TYPE")
}
