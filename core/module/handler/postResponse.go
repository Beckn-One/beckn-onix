package handler

import (
	"net/http"
)

type PostResponseHook func()

type PostResponseKey struct{}

// RegisterPostResponseHook registers a function to be executed
// after the response is written and all middleware has completed.
func RegisterPostResponseHook(r *http.Request, fn PostResponseHook) {
	hooks, ok := r.Context().Value(PostResponseKey{}).(*[]PostResponseHook)
	if !ok || hooks == nil {
		// PostResponseMiddleware not installed or already executed
		return
	}
	*hooks = append(*hooks, fn)
}
