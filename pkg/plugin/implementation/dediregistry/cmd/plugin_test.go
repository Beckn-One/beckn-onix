package main

import (
	"context"
	"testing"
)

func TestDediRegistryProvider_New(t *testing.T) {
	ctx := context.Background()
	provider := dediRegistryProvider{}

	config := map[string]string{
		"baseURL":      "https://test.com",
		"apiKey":       "test-key",
		"namespaceID":  "test-namespace",
		"registryName": "test-registry",
		"recordName":   "test-record",
		"timeout":      "30",
	}

	dediRegistry, closer, err := provider.New(ctx, config)
	if err != nil {
		t.Errorf("New() error = %v", err)
		return
	}

	if dediRegistry == nil {
		t.Error("New() returned nil dediRegistry")
	}

	if closer == nil {
		t.Error("New() returned nil closer")
	}

	// Test cleanup
	if err := closer(); err != nil {
		t.Errorf("closer() error = %v", err)
	}
}

func TestDediRegistryProvider_New_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	provider := dediRegistryProvider{}

	tests := []struct {
		name   string
		config map[string]string
	}{
		{
			name:   "missing baseURL",
			config: map[string]string{"apiKey": "test-key"},
		},
		{
			name:   "missing apiKey",
			config: map[string]string{"baseURL": "https://test.com"},
		},
		{
			name:   "empty config",
			config: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := provider.New(ctx, tt.config)
			if err == nil {
				t.Errorf("New() with %s should return error", tt.name)
			}
		})
	}
}

func TestDediRegistryProvider_New_InvalidTimeout(t *testing.T) {
	ctx := context.Background()
	provider := dediRegistryProvider{}

	config := map[string]string{
		"baseURL":      "https://test.com",
		"apiKey":       "test-key",
		"namespaceID":  "test-namespace",
		"registryName": "test-registry",
		"recordName":   "test-record",
		"timeout":      "invalid",
	}

	// Invalid timeout should be ignored, not cause error
	dediRegistry, closer, err := provider.New(ctx, config)
	if err != nil {
		t.Errorf("New() with invalid timeout should not return error, got: %v", err)
	}
	if dediRegistry == nil {
		t.Error("New() should return valid registry even with invalid timeout")
	}
	if closer != nil {
		closer()
	}
}