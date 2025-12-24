package ondcvalidator

import (
	"context"
	"fmt"
	"net/url"

	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
)

type Config struct {
	StateFullValidations bool `json:"stateFullValidations" yaml:"stateFullValidations"`
	DebugMode bool `json:"debugMode" yaml:"debugMode"`
}

type ondcValidator struct {
	config *Config
	cache  definition.Cache
}

func (v *ondcValidator) ValidatePayload(ctx context.Context, url *url.URL, payload []byte) error {
	return nil
}

func (v *ondcValidator) SaveValidationData(ctx context.Context, url *url.URL, payload []byte) error {
	return nil
}

func New(ctx context.Context, cache definition.Cache, config *Config) (definition.OndcValidator, func() error, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("config cannot be nil")
	}
	v := &ondcValidator{
		config: config,
		cache:  cache,
	}
	return v, nil, nil
}