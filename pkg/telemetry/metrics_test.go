package telemetry

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewProviderAndMetrics(t *testing.T) {
	ctx := context.Background()
	provider, err := NewProvider(ctx, &Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		EnableMetrics:  true,
		Environment:    "test",
	})
	require.NoError(t, err)
	require.NotNil(t, provider)
	require.NotNil(t, provider.MetricsHandler)

	metrics, err := GetMetrics(ctx)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	provider.MetricsHandler.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)

	require.NoError(t, provider.Shutdown(context.Background()))
}
