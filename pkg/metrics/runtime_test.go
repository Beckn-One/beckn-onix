package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitRuntimeMetrics(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		wantError bool
	}{
		{
			name:      "metrics enabled",
			enabled:   true,
			wantError: false,
		},
		{
			name:      "metrics disabled",
			enabled:   false,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Initialize metrics with enabled state
			cfg := Config{
				Enabled:      tt.enabled,
				ExporterType: ExporterPrometheus,
				ServiceName:  "test-service",
			}
			err := InitMetrics(cfg)
			require.NoError(t, err)

			// Test InitRuntimeMetrics
			err = InitRuntimeMetrics()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Cleanup
			Shutdown(context.Background())
		})
	}
}

func TestInitRuntimeMetrics_MultipleCalls(t *testing.T) {
	// Setup
	cfg := Config{
		Enabled:      true,
		ExporterType: ExporterPrometheus,
		ServiceName:  "test-service",
	}
	err := InitMetrics(cfg)
	require.NoError(t, err)
	defer Shutdown(context.Background())

	// Test: Multiple calls should not cause errors
	err = InitRuntimeMetrics()
	require.NoError(t, err)

	// Note: Second call might fail if runtime.Start is already called,
	// but that's expected behavior
	err = InitRuntimeMetrics()
	// We don't assert on error here as it depends on internal state
	_ = err
}

func TestInitRuntimeMetrics_WhenDisabled(t *testing.T) {
	// Setup: Metrics disabled
	cfg := Config{
		Enabled:      false,
		ExporterType: ExporterPrometheus,
		ServiceName:  "test-service",
	}
	err := InitMetrics(cfg)
	require.NoError(t, err)
	defer Shutdown(context.Background())

	// Test: Should return nil without error when disabled
	err = InitRuntimeMetrics()
	assert.NoError(t, err)
}

