package main

import (
	"context"
	"net/http"

	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/otelmetrics"
)

type middlewareProvider struct{}

func (middlewareProvider) New(ctx context.Context, cfg map[string]string) (func(http.Handler) http.Handler, error) {
	mw, err := otelmetrics.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return mw.Handler, nil
}

// Provider is exported for plugin loader.
var Provider = middlewareProvider{}
