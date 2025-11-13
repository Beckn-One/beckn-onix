package metrics

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPMiddleware wraps an HTTP handler with OpenTelemetry instrumentation.
func HTTPMiddleware(handler http.Handler, operation string) http.Handler {
	if !IsEnabled() {
		return handler
	}

	return otelhttp.NewHandler(
		handler,
		operation,
	)
}

// HTTPHandler wraps an HTTP handler function with OpenTelemetry instrumentation.
func HTTPHandler(handler http.HandlerFunc, operation string) http.Handler {
	return HTTPMiddleware(handler, operation)
}
