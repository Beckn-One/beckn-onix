package handler

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name     string
		config   HttpClientConfig
		expected struct {
			maxIdleConns          int
			maxIdleConnsPerHost   int
			idleConnTimeout       time.Duration
			responseHeaderTimeout time.Duration
		}
	}{
		{
			name: "all values configured",
			config: HttpClientConfig{
				MaxIdleConns:          1000,
				MaxIdleConnsPerHost:   200,
				IdleConnTimeout:       300 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
			},
			expected: struct {
				maxIdleConns          int
				maxIdleConnsPerHost   int
				idleConnTimeout       time.Duration
				responseHeaderTimeout time.Duration
			}{
				maxIdleConns:          1000,
				maxIdleConnsPerHost:   200,
				idleConnTimeout:       300 * time.Second,
				responseHeaderTimeout: 5 * time.Second,
			},
		},
		{
			name:   "zero values use defaults",
			config: HttpClientConfig{},
			expected: struct {
				maxIdleConns          int
				maxIdleConnsPerHost   int
				idleConnTimeout       time.Duration
				responseHeaderTimeout time.Duration
			}{
				maxIdleConns:          100, // Go default
				maxIdleConnsPerHost:   0,   // Go default (unlimited per host)
				idleConnTimeout:       90 * time.Second,
				responseHeaderTimeout: 0,
			},
		},
		{
			name: "partial configuration",
			config: HttpClientConfig{
				MaxIdleConns:    500,
				IdleConnTimeout: 180 * time.Second,
			},
			expected: struct {
				maxIdleConns          int
				maxIdleConnsPerHost   int
				idleConnTimeout       time.Duration
				responseHeaderTimeout time.Duration
			}{
				maxIdleConns:          500,
				maxIdleConnsPerHost:   0, // Go default (unlimited per host)
				idleConnTimeout:       180 * time.Second,
				responseHeaderTimeout: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newHTTPClient(&tt.config, nil)

			if client == nil {
				t.Fatal("newHTTPClient returned nil")
			}

			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("client transport is not *http.Transport")
			}

			if transport.MaxIdleConns != tt.expected.maxIdleConns {
				t.Errorf("MaxIdleConns = %d, want %d", transport.MaxIdleConns, tt.expected.maxIdleConns)
			}

			if transport.MaxIdleConnsPerHost != tt.expected.maxIdleConnsPerHost {
				t.Errorf("MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, tt.expected.maxIdleConnsPerHost)
			}

			if transport.IdleConnTimeout != tt.expected.idleConnTimeout {
				t.Errorf("IdleConnTimeout = %v, want %v", transport.IdleConnTimeout, tt.expected.idleConnTimeout)
			}

			if transport.ResponseHeaderTimeout != tt.expected.responseHeaderTimeout {
				t.Errorf("ResponseHeaderTimeout = %v, want %v", transport.ResponseHeaderTimeout, tt.expected.responseHeaderTimeout)
			}
		})
	}
}

func TestHttpClientConfigDefaults(t *testing.T) {
	// Test that zero config values don't override defaults
	config := &HttpClientConfig{}
	client := newHTTPClient(config, nil)

	transport := client.Transport.(*http.Transport)

	// Verify defaults are preserved when config values are zero
	if transport.MaxIdleConns == 0 {
		t.Error("MaxIdleConns should not be zero when using defaults")
	}

	// MaxIdleConnsPerHost default is 0 (unlimited), which is correct
	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected default MaxIdleConns=100, got %d", transport.MaxIdleConns)
	}
}

func TestHttpClientConfigPerformanceValues(t *testing.T) {
	// Test the specific performance-optimized values from the document
	config := &HttpClientConfig{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   200,
		IdleConnTimeout:       300 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	client := newHTTPClient(config, nil)
	transport := client.Transport.(*http.Transport)

	// Verify performance-optimized values
	if transport.MaxIdleConns != 1000 {
		t.Errorf("Expected MaxIdleConns=1000, got %d", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 200 {
		t.Errorf("Expected MaxIdleConnsPerHost=200, got %d", transport.MaxIdleConnsPerHost)
	}

	if transport.IdleConnTimeout != 300*time.Second {
		t.Errorf("Expected IdleConnTimeout=300s, got %v", transport.IdleConnTimeout)
	}

	if transport.ResponseHeaderTimeout != 5*time.Second {
		t.Errorf("Expected ResponseHeaderTimeout=5s, got %v", transport.ResponseHeaderTimeout)
	}
}

func TestNewHTTPClientWithTransportWrapper(t *testing.T) {
	wrappedTransport := &mockRoundTripper{}
	wrapper := &mockTransportWrapper{
		returnTransport: wrappedTransport,
	}

	client := newHTTPClient(&HttpClientConfig{}, wrapper)

	if !wrapper.wrapCalled {
		t.Fatal("expected transport wrapper to be invoked")
	}

	if wrapper.wrappedTransport == nil {
		t.Fatal("expected base transport to be passed to wrapper")
	}

	if client.Transport != wrappedTransport {
		t.Errorf("expected client transport to use wrapper transport")
	}
}

type mockTransportWrapper struct {
	wrapCalled       bool
	wrappedTransport http.RoundTripper
	returnTransport  http.RoundTripper
}

func (m *mockTransportWrapper) Wrap(base http.RoundTripper) http.RoundTripper {
	m.wrapCalled = true
	m.wrappedTransport = base
	if m.returnTransport != nil {
		return m.returnTransport
	}
	return base
}

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, nil
}
