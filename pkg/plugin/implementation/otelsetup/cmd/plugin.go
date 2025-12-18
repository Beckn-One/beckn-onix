package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/otelsetup"
	"github.com/beckn-one/beckn-onix/pkg/telemetry"
)

// metricsProvider implements the OtelSetupMetricsProvider interface for the otelsetup plugin.
type metricsProvider struct {
	impl otelsetup.Setup
}

// New creates a new telemetry provider instance.
func (m metricsProvider) New(ctx context.Context, config map[string]string) (*telemetry.Provider, func() error, error) {
	if ctx == nil {
		return nil, nil, errors.New("context cannot be nil")
	}

	// Convert map[string]string to otelsetup.Config
	telemetryConfig := &otelsetup.Config{
		ServiceName:    config["serviceName"],
		ServiceVersion: config["serviceVersion"],
		Environment:    config["environment"],
		MetricsPort:    config["metricsPort"],
	}

	// Parse enableMetrics as boolean
	if enableMetricsStr, ok := config["enableMetrics"]; ok && enableMetricsStr != "" {
		enableMetrics, err := strconv.ParseBool(enableMetricsStr)
		if err != nil {
			log.Warnf(ctx, "Invalid enableMetrics value '%s', defaulting to true: %v", enableMetricsStr, err)
			telemetryConfig.EnableMetrics = true
		} else {
			telemetryConfig.EnableMetrics = enableMetrics
		}
	} else {
		telemetryConfig.EnableMetrics = true // Default to true if not specified or empty
	}

	// Apply defaults if fields are empty
	if telemetryConfig.ServiceName == "" {
		telemetryConfig.ServiceName = otelsetup.DefaultConfig().ServiceName
	}
	if telemetryConfig.ServiceVersion == "" {
		telemetryConfig.ServiceVersion = otelsetup.DefaultConfig().ServiceVersion
	}
	if telemetryConfig.Environment == "" {
		telemetryConfig.Environment = otelsetup.DefaultConfig().Environment
	}

	log.Debugf(ctx, "Telemetry config mapped: %+v", telemetryConfig)
	provider, err := m.impl.New(ctx, telemetryConfig)
	if err != nil {
		log.Errorf(ctx, err, "Failed to create telemetry provider instance")
		return nil, nil, err
	}

	// Wrap the Shutdown function to match the closer signature
	var closer func() error
	if provider != nil && provider.Shutdown != nil {
		closer = func() error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return provider.Shutdown(shutdownCtx)
		}
	}

	log.Infof(ctx, "Telemetry provider instance created successfully")
	return provider, closer, nil
}

// Provider is the exported plugin instance
var Provider = metricsProvider{}
