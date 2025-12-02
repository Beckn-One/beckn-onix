package otelsetup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/beckn-one/beckn-onix/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_New_Success(t *testing.T) {
	setup := Setup{}
	ctx := context.Background()

	tests := []struct {
		name string
		cfg  *telemetry.Config
	}{
		{
			name: "Valid config with all fields",
			cfg: &telemetry.Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				EnableMetrics:  true,
				Environment:    "test",
			},
		},
		{
			name: "Valid config with metrics disabled",
			cfg: &telemetry.Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				EnableMetrics:  false,
				Environment:    "test",
			},
		},
		{
			name: "Config with empty fields uses defaults",
			cfg: &telemetry.Config{
				ServiceName:    "",
				ServiceVersion: "",
				EnableMetrics:  true,
				Environment:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := setup.New(ctx, tt.cfg)

			require.NoError(t, err, "New() should not return error")
			require.NotNil(t, provider, "New() should return non-nil provider")
			require.NotNil(t, provider.Shutdown, "Provider should have shutdown function")

			if tt.cfg.EnableMetrics {
				assert.NotNil(t, provider.MetricsHandler, "MetricsHandler should be set when metrics enabled")
				assert.NotNil(t, provider.MeterProvider, "MeterProvider should be set when metrics enabled")

				// Test that metrics handler works
				rec := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/metrics", nil)
				provider.MetricsHandler.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Nil(t, provider.MetricsHandler, "MetricsHandler should be nil when metrics disabled")
			}

			// Test shutdown
			err = provider.Shutdown(ctx)
			assert.NoError(t, err, "Shutdown should not return error")
		})
	}
}

func TestSetup_New_Failure(t *testing.T) {
	setup := Setup{}
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     *telemetry.Config
		wantErr bool
	}{
		{
			name:    "Nil config",
			cfg:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := setup.New(ctx, tt.cfg)

			if tt.wantErr {
				assert.Error(t, err, "New() should return error")
				assert.Nil(t, provider, "New() should return nil provider on error")
			} else {
				assert.NoError(t, err, "New() should not return error")
				assert.NotNil(t, provider, "New() should return non-nil provider")
			}
		})
	}
}

func TestSetup_New_DefaultValues(t *testing.T) {
	setup := Setup{}
	ctx := context.Background()

	// Test with empty fields - should use defaults
	cfg := &telemetry.Config{
		ServiceName:    "",
		ServiceVersion: "",
		EnableMetrics:  true,
		Environment:    "",
	}

	provider, err := setup.New(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify defaults are applied by checking that provider is functional
	assert.NotNil(t, provider.MetricsHandler, "MetricsHandler should be set with defaults")
	assert.NotNil(t, provider.MeterProvider, "MeterProvider should be set with defaults")

	// Test metrics endpoint
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	provider.MetricsHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Cleanup
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestSetup_New_MetricsDisabled(t *testing.T) {
	setup := Setup{}
	ctx := context.Background()

	cfg := &telemetry.Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		EnableMetrics:  false,
		Environment:    "test",
	}

	provider, err := setup.New(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// When metrics are disabled, MetricsHandler should be nil
	assert.Nil(t, provider.MetricsHandler, "MetricsHandler should be nil when metrics disabled")
	assert.Nil(t, provider.MeterProvider, "MeterProvider should be nil when metrics disabled")

	// Shutdown should still work
	err = provider.Shutdown(ctx)
	assert.NoError(t, err, "Shutdown should work even when metrics disabled")
}

func TestToPluginConfig_Success(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *telemetry.Config
		expectedID     string
		expectedConfig map[string]string
	}{
		{
			name: "Valid config with all fields",
			cfg: &telemetry.Config{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				EnableMetrics:  true,
				Environment:    "test",
			},
			expectedID: "otelsetup",
			expectedConfig: map[string]string{
				"serviceName":    "test-service",
				"serviceVersion": "1.0.0",
				"enableMetrics":  "true",
				"environment":    "test",
			},
		},
		{
			name: "Config with enableMetrics false",
			cfg: &telemetry.Config{
				ServiceName:    "my-service",
				ServiceVersion: "2.0.0",
				EnableMetrics:  false,
				Environment:    "production",
			},
			expectedID: "otelsetup",
			expectedConfig: map[string]string{
				"serviceName":    "my-service",
				"serviceVersion": "2.0.0",
				"enableMetrics":  "false",
				"environment":    "production",
			},
		},
		{
			name: "Config with empty fields",
			cfg: &telemetry.Config{
				ServiceName:    "",
				ServiceVersion: "",
				EnableMetrics:  true,
				Environment:    "",
			},
			expectedID: "otelsetup",
			expectedConfig: map[string]string{
				"serviceName":    "",
				"serviceVersion": "",
				"enableMetrics":  "true",
				"environment":    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToPluginConfig(tt.cfg)

			require.NotNil(t, result, "ToPluginConfig should return non-nil config")
			assert.Equal(t, tt.expectedID, result.ID, "Plugin ID should be 'otelsetup'")
			assert.Equal(t, tt.expectedConfig, result.Config, "Config map should match expected values")
		})
	}
}

func TestToPluginConfig_NilConfig(t *testing.T) {
	// Test that ToPluginConfig handles nil config
	// Note: This will panic if nil is passed, which is acceptable behavior
	// as the function expects a valid config. In practice, callers should check for nil.
	assert.Panics(t, func() {
		ToPluginConfig(nil)
	}, "ToPluginConfig should panic when given nil config")
}

func TestToPluginConfig_BooleanConversion(t *testing.T) {
	tests := []struct {
		name          string
		enableMetrics bool
		expected      string
	}{
		{
			name:          "EnableMetrics true",
			enableMetrics: true,
			expected:      "true",
		},
		{
			name:          "EnableMetrics false",
			enableMetrics: false,
			expected:      "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &telemetry.Config{
				ServiceName:    "test",
				ServiceVersion: "1.0.0",
				EnableMetrics:  tt.enableMetrics,
				Environment:    "test",
			}

			result := ToPluginConfig(cfg)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result.Config["enableMetrics"], "enableMetrics should be converted to string correctly")
		})
	}
}
