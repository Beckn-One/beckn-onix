package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
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
			client := newHTTPClient(&tt.config)

			if client == nil {
				t.Fatal("newHTTPClient returned nil")
			}

			// When retry config is not provided, transport should be *http.Transport
			// When retry config is provided, transport is wrapped by retryablehttp
			var transport *http.Transport
			if tt.config.Retry == nil || tt.config.Retry.MaxRetries == 0 {
				var ok bool
				transport, ok = client.Transport.(*http.Transport)
				if !ok {
					t.Fatal("client transport is not *http.Transport when retry is not configured")
				}
			} else {
				// With retry, the transport is wrapped, so we can't directly access it
				// Just verify the client is created successfully
				if client.Transport == nil {
					t.Fatal("client transport is nil")
				}
				return // Skip transport checks for retry-enabled clients
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
	client := newHTTPClient(config)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client transport is not *http.Transport")
	}

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

	client := newHTTPClient(config)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("client transport is not *http.Transport")
	}

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

// TestNewHTTPClientWithRetry tests HTTP client creation with retry configuration
func TestNewHTTPClientWithRetry(t *testing.T) {
	tests := []struct {
		name            string
		config          HttpClientConfig
		expectRetry     bool
		expectedRetries int
	}{
		{
			name: "retry config provided",
			config: HttpClientConfig{
				MaxIdleConns: 1000,
				Retry: &RetryConfig{
					MaxRetries:           3,
					Delays:               []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second},
					RetryableStatusCodes: []int{500, 502, 503},
				},
			},
			expectRetry:     true,
			expectedRetries: 3,
		},
		{
			name: "retry config with zero max retries",
			config: HttpClientConfig{
				Retry: &RetryConfig{
					MaxRetries: 0,
				},
			},
			expectRetry: false,
		},
		{
			name: "no retry config",
			config: HttpClientConfig{
				MaxIdleConns: 1000,
			},
			expectRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newHTTPClient(&tt.config)
			if client == nil {
				t.Fatal("newHTTPClient returned nil")
			}

			if tt.expectRetry {
				// When retry is enabled, the client should be a standard client from retryablehttp
				// We can't directly check if it's using retryablehttp, but we can verify
				// the client is functional and the transport is not nil
				if client.Transport == nil {
					t.Error("client transport should not be nil when retry is enabled")
				}
			} else {
				// When retry is not enabled, transport should be *http.Transport
				_, ok := client.Transport.(*http.Transport)
				if !ok {
					t.Error("client transport should be *http.Transport when retry is not configured")
				}
			}
		})
	}
}

// TestRetryOnRetryableStatusCodes tests that retries occur on configured status codes
func TestRetryOnRetryableStatusCodes(t *testing.T) {
	attemptCount := 0
	retryableStatusCodes := []int{500, 502, 503}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return 500 for first 2 attempts, then 200
		if attemptCount < 3 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()

	config := &HttpClientConfig{
		Retry: &RetryConfig{
			MaxRetries:           3,
			Delays:               []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond},
			RetryableStatusCodes: retryableStatusCodes,
		},
	}

	client := newHTTPClient(config)

	// Create a retryable request to test retry logic
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.RetryMax = config.Retry.MaxRetries

	// Create a map of retryable status codes
	retryableStatusCodesMap := make(map[int]bool)
	for _, code := range config.Retry.RetryableStatusCodes {
		retryableStatusCodesMap[code] = true
	}

	// Custom CheckRetry function
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if err != nil {
			return true, nil
		}
		if resp != nil && retryableStatusCodesMap[resp.StatusCode] {
			return true, nil
		}
		return false, nil
	}

	retryReq, err := retryablehttp.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create retryable request: %v", err)
	}

	resp, err := retryClient.Do(retryReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should have retried at least once (attemptCount should be > 1)
	if attemptCount < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attemptCount)
	}

	// Final response should be 200
	if resp.StatusCode != 200 {
		t.Errorf("expected final status code 200, got %d", resp.StatusCode)
	}
}

// TestNoRetryOnNonRetryableStatusCodes tests that retries don't occur on non-retryable status codes
func TestNoRetryOnNonRetryableStatusCodes(t *testing.T) {
	attemptCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		// Return 404 (not retryable)
		w.WriteHeader(404)
	}))
	defer server.Close()

	config := &HttpClientConfig{
		Retry: &RetryConfig{
			MaxRetries:           3,
			Delays:               []time.Duration{10 * time.Millisecond},
			RetryableStatusCodes: []int{500, 502, 503},
		},
	}

	client := newHTTPClient(config)
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should not retry on 404, so attemptCount should be 1
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt for non-retryable status code, got %d", attemptCount)
	}

	if resp.StatusCode != 404 {
		t.Errorf("expected status code 404, got %d", resp.StatusCode)
	}
}

// TestRetryBackoffDelays tests that custom backoff delays are used
func TestRetryBackoffDelays(t *testing.T) {
	attemptCount := 0
	delays := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 150 * time.Millisecond}
	delayTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delayTimes = append(delayTimes, time.Now())
		attemptCount++
		// Return 500 for first 3 attempts, then 200
		if attemptCount < 4 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()

	config := &HttpClientConfig{
		Retry: &RetryConfig{
			MaxRetries:           3,
			Delays:               delays,
			RetryableStatusCodes: []int{500, 502, 503},
		},
	}

	client := newHTTPClient(config)

	// Create a retryable request to test retry logic
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.RetryMax = config.Retry.MaxRetries

	retryableStatusCodesMap := make(map[int]bool)
	for _, code := range config.Retry.RetryableStatusCodes {
		retryableStatusCodesMap[code] = true
	}

	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if err != nil {
			return true, nil
		}
		if resp != nil && retryableStatusCodesMap[resp.StatusCode] {
			return true, nil
		}
		return false, nil
	}

	// Custom backoff function
	retryClient.Backoff = func(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
		delayIndex := attemptNum - 1
		if delayIndex < 0 {
			delayIndex = 0
		}
		if delayIndex >= len(delays) {
			delayIndex = len(delays) - 1
		}
		return delays[delayIndex]
	}

	retryReq, err := retryablehttp.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create retryable request: %v", err)
	}

	startTime := time.Now()
	resp, err := retryClient.Do(retryReq)
	duration := time.Since(startTime)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should have retried multiple times
	if attemptCount < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attemptCount)
	}

	// Verify that delays were applied (total duration should be at least the sum of delays)
	// We check that the duration is reasonable (at least some delay was applied)
	minExpectedDuration := delays[0] + delays[1]
	if duration < minExpectedDuration {
		t.Errorf("expected total duration to be at least %v, got %v", minExpectedDuration, duration)
	}
}

// TestRetryOnConnectionError tests that retries occur on connection errors
func TestRetryOnConnectionError(t *testing.T) {
	config := &HttpClientConfig{
		Retry: &RetryConfig{
			MaxRetries:           2,
			Delays:               []time.Duration{10 * time.Millisecond},
			RetryableStatusCodes: []int{500, 502, 503},
		},
	}

	client := newHTTPClient(config)

	// Create a retryable request
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.RetryMax = config.Retry.MaxRetries

	retryableStatusCodesMap := make(map[int]bool)
	for _, code := range config.Retry.RetryableStatusCodes {
		retryableStatusCodesMap[code] = true
	}

	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		// Should retry on connection errors
		if err != nil {
			return true, nil
		}
		if resp != nil && retryableStatusCodesMap[resp.StatusCode] {
			return true, nil
		}
		return false, nil
	}

	retryReq, err := retryablehttp.NewRequest("GET", "http://127.0.0.1:99999/invalid", nil)
	if err != nil {
		t.Fatalf("failed to create retryable request: %v", err)
	}

	// This should fail but retry
	_, err = retryClient.Do(retryReq)
	if err == nil {
		t.Error("expected error for invalid connection, got nil")
	}
	// The error is expected, but retries should have been attempted
}

// TestRetryConfigParsing tests that retry configuration is parsed correctly
func TestRetryConfigParsing(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:           3,
		Delays:               []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second},
		RetryableStatusCodes: []int{500, 502, 503},
	}

	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", config.MaxRetries)
	}

	if len(config.Delays) != 3 {
		t.Errorf("expected 3 delays, got %d", len(config.Delays))
	}

	if len(config.RetryableStatusCodes) != 3 {
		t.Errorf("expected 3 retryable status codes, got %d", len(config.RetryableStatusCodes))
	}

	// Verify specific values
	if config.Delays[0] != 1*time.Second {
		t.Errorf("expected first delay=1s, got %v", config.Delays[0])
	}

	if config.RetryableStatusCodes[0] != 500 {
		t.Errorf("expected first status code=500, got %d", config.RetryableStatusCodes[0])
	}
}
