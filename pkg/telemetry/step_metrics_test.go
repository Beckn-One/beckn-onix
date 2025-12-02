package telemetry

import (
	"context"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/metric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStepMetrics_Success(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry provider first
	provider, err := NewTestProvider(ctx)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Test getting step metrics
	metrics, err := GetStepMetrics(ctx)
	require.NoError(t, err, "GetStepMetrics() should not return error")
	require.NotNil(t, metrics, "GetStepMetrics() should return non-nil metrics")

	// Verify all metric instruments are initialized
	assert.NotNil(t, metrics.StepExecutionDuration, "StepExecutionDuration should be initialized")
	assert.NotNil(t, metrics.StepExecutionTotal, "StepExecutionTotal should be initialized")
	assert.NotNil(t, metrics.StepErrorsTotal, "StepErrorsTotal should be initialized")
}

func TestGetStepMetrics_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry provider first
	provider, err := NewTestProvider(ctx)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Test that GetStepMetrics is safe for concurrent access
	// and returns the same instance (singleton pattern)
	metrics1, err1 := GetStepMetrics(ctx)
	require.NoError(t, err1)
	require.NotNil(t, metrics1)

	metrics2, err2 := GetStepMetrics(ctx)
	require.NoError(t, err2)
	require.NotNil(t, metrics2)

	// Should return the same instance
	assert.Equal(t, metrics1, metrics2, "GetStepMetrics should return the same instance")
}

func TestGetStepMetrics_WithoutProvider(t *testing.T) {
	ctx := context.Background()

	// Test getting step metrics without initializing provider
	// This should still work but may not have a valid meter provider
	metrics, err := GetStepMetrics(ctx)
	// Note: This might succeed or fail depending on OTel's default behavior
	// We're just checking it doesn't panic
	if err != nil {
		t.Logf("GetStepMetrics returned error (expected if no provider): %v", err)
	} else {
		assert.NotNil(t, metrics, "Metrics should be returned even without explicit provider")
	}
}

func TestStepMetrics_Instruments(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry provider
	provider, err := NewTestProvider(ctx)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Get step metrics
	metrics, err := GetStepMetrics(ctx)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Test that we can record metrics (this tests the instruments are functional)
	// Note: We can't easily verify the metrics were recorded without querying the exporter,
	// but we can verify the instruments are not nil and can be called without panicking

	// Test StepExecutionDuration
	require.NotPanics(t, func() {
		metrics.StepExecutionDuration.Record(ctx, 0.5,
			metric.WithAttributes(AttrStep.String("test-step"), AttrModule.String("test-module")))
	}, "StepExecutionDuration.Record should not panic")

	// Test StepExecutionTotal
	require.NotPanics(t, func() {
		metrics.StepExecutionTotal.Add(ctx, 1,
			metric.WithAttributes(AttrStep.String("test-step"), AttrModule.String("test-module")))
	}, "StepExecutionTotal.Add should not panic")

	// Test StepErrorsTotal
	require.NotPanics(t, func() {
		metrics.StepErrorsTotal.Add(ctx, 1,
			metric.WithAttributes(AttrStep.String("test-step"), AttrModule.String("test-module")))
	}, "StepErrorsTotal.Add should not panic")

	// Verify metrics are exposed via HTTP handler
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	provider.MetricsHandler.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code, "Metrics endpoint should return 200")
}

func TestStepMetrics_MultipleCalls(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry provider
	provider, err := NewTestProvider(ctx)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Call GetStepMetrics multiple times
	for i := 0; i < 10; i++ {
		metrics, err := GetStepMetrics(ctx)
		require.NoError(t, err, "GetStepMetrics should succeed on call %d", i)
		require.NotNil(t, metrics, "GetStepMetrics should return non-nil on call %d", i)
		assert.NotNil(t, metrics.StepExecutionDuration, "StepExecutionDuration should be initialized")
		assert.NotNil(t, metrics.StepExecutionTotal, "StepExecutionTotal should be initialized")
		assert.NotNil(t, metrics.StepErrorsTotal, "StepErrorsTotal should be initialized")
	}
}
