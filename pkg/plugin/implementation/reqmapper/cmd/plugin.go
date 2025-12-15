package main

import (
	"context"
	"net/http"

	"github.com/beckn-one/beckn-onix/pkg/plugin/implementation/reqmapper"
)

type provider struct{}

func (p provider) New(ctx context.Context, c map[string]string) (func(http.Handler) http.Handler, error) {
	config := &reqmapper.Config{}
	if role, ok := c["role"]; ok {
		config.Role = role
	}
	if mappingsFile, ok := c["mappingsFile"]; ok {
		config.MappingsFile = mappingsFile
	}
	return reqmapper.NewReqMapper(config)
}

var Provider = provider{}
