package main

import (
	"context"
	"errors"
	"strconv"

	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/schemav2validator"
)

// schemav2ValidatorProvider provides instances of schemav2Validator.
type schemav2ValidatorProvider struct{}

// New initialises a new Schemav2Validator instance.
func (vp schemav2ValidatorProvider) New(ctx context.Context, config map[string]string) (definition.SchemaValidator, func() error, error) {
	if ctx == nil {
		return nil, nil, errors.New("context cannot be nil")
	}

	typeVal, hasType := config["type"]
	locVal, hasLoc := config["location"]

	if !hasType || typeVal == "" {
		return nil, nil, errors.New("type not configured")
	}
	if !hasLoc || locVal == "" {
		return nil, nil, errors.New("location not configured")
	}

	cfg := &schemav2validator.Config{
		Type:     typeVal,
		Location: locVal,
		CacheTTL: 3600,
	}

	if ttlStr, ok := config["cacheTTL"]; ok {
		if ttl, err := strconv.Atoi(ttlStr); err == nil && ttl > 0 {
			cfg.CacheTTL = ttl
		}
	}

	return schemav2validator.New(ctx, cfg)
}

// Provider is the exported plugin provider.
var Provider schemav2ValidatorProvider
