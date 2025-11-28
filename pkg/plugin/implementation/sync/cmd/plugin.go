package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/sync"
)

// syncProvider implements the SyncProvider interface
type syncProvider struct{}

// New creates a new Sync plugin instance with dependencies
func (p syncProvider) New(ctx context.Context, cache definition.Cache, role string, config map[string]string) (definition.Sync, func() error, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("context cannot be nil")
	}

	if cache == nil {
		return nil, nil, fmt.Errorf("cache plugin is required")
	}

	// Parse configuration from map[string]string
	cfg := &sync.Config{
		Enabled:       false,            // default: disabled
		Timeout:       30 * time.Second, // default
		ChannelPrefix: "sync:",          // default
		KeyTTL:        35 * time.Second, // default
	}

	// Parse enabled flag if provided
	if enabledStr, ok := config["enabled"]; ok && enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			cfg.Enabled = enabled
		} else {
			log.Warnf(ctx, "Invalid enabled value '%s', using default: false", enabledStr)
		}
	}

	// Parse timeout if provided
	if timeoutStr, ok := config["timeout"]; ok && timeoutStr != "" {
		if duration, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.Timeout = duration
		} else {
			log.Warnf(ctx, "Invalid timeout value '%s', using default: %s", timeoutStr, cfg.Timeout)
		}
	}

	// Parse channel prefix if provided
	if prefix, ok := config["channelPrefix"]; ok && prefix != "" {
		cfg.ChannelPrefix = prefix
	}

	// Parse key TTL if provided
	if ttlStr, ok := config["keyTTL"]; ok && ttlStr != "" {
		if duration, err := time.ParseDuration(ttlStr); err == nil {
			cfg.KeyTTL = duration
		} else {
			log.Warnf(ctx, "Invalid keyTTL value '%s', using default: %s", ttlStr, cfg.KeyTTL)
		}
	}

	// Create sync instance
	syncPlugin, closer, err := sync.New(ctx, cache, role, cfg)
	if err != nil {
		log.Errorf(ctx, err, "Failed to create sync plugin instance")
		return nil, nil, err
	}

	log.Infof(ctx, "Sync plugin created successfully for role: %s", role)
	return syncPlugin, closer, nil
}

var Provider = syncProvider{}
