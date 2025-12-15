package definition

import (
	"context"
	"net/http"
)

// TransportWrapper is a plugin that wraps an http.RoundTripper,
// allowing modification of outbound requests (like adding auth).
type TransportWrapper interface {
	// Wrap takes a base transport and returns a new transport that wraps it.
	Wrap(base http.RoundTripper) http.RoundTripper
}

// TransportWrapperProvider defines the factory for a TransportWrapper.
type TransportWrapperProvider interface {
	New(ctx context.Context, config map[string]any) (TransportWrapper, func(), error)
}
