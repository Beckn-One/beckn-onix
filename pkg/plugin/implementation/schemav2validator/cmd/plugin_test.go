package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSpec = `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                context:
                  type: object
                  properties:
                    action:
                      const: test
`

func TestProvider_New(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testSpec))
	}))
	defer server.Close()

	tests := []struct {
		name    string
		ctx     context.Context
		config  map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil context",
			ctx:     nil,
			config:  map[string]string{"url": server.URL},
			wantErr: true,
			errMsg:  "context cannot be nil",
		},
		{
			name:    "missing type",
			ctx:     context.Background(),
			config:  map[string]string{"location": server.URL},
			wantErr: true,
			errMsg:  "type not configured",
		},
		{
			name:    "missing location",
			ctx:     context.Background(),
			config:  map[string]string{"type": "url"},
			wantErr: true,
			errMsg:  "location not configured",
		},
		{
			name:    "empty type",
			ctx:     context.Background(),
			config:  map[string]string{"type": "", "location": server.URL},
			wantErr: true,
			errMsg:  "type not configured",
		},
		{
			name:    "empty location",
			ctx:     context.Background(),
			config:  map[string]string{"type": "url", "location": ""},
			wantErr: true,
			errMsg:  "location not configured",
		},
		{
			name:    "valid config with default TTL",
			ctx:     context.Background(),
			config:  map[string]string{"type": "url", "location": server.URL},
			wantErr: false,
		},
		{
			name: "valid config with custom TTL",
			ctx:  context.Background(),
			config: map[string]string{
				"type":     "url",
				"location": server.URL,
				"cacheTTL": "7200",
			},
			wantErr: false,
		},
		{
			name: "valid file type",
			ctx:  context.Background(),
			config: map[string]string{
				"type":     "file",
				"location": "/tmp/spec.yaml",
			},
			wantErr: true, // file doesn't exist
		},
		{
			name: "invalid TTL falls back to default",
			ctx:  context.Background(),
			config: map[string]string{
				"type":     "url",
				"location": server.URL,
				"cacheTTL": "invalid",
			},
			wantErr: false,
		},
		{
			name: "negative TTL falls back to default",
			ctx:  context.Background(),
			config: map[string]string{
				"type":     "url",
				"location": server.URL,
				"cacheTTL": "-100",
			},
			wantErr: false,
		},
		{
			name: "zero TTL falls back to default",
			ctx:  context.Background(),
			config: map[string]string{
				"type":     "url",
				"location": server.URL,
				"cacheTTL": "0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := schemav2ValidatorProvider{}
			validator, cleanup, err := provider.New(tt.ctx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("New() error = %v, want error containing %v", err, tt.errMsg)
				}
			}

			if !tt.wantErr {
				if validator == nil {
					t.Error("Expected validator instance, got nil")
				}
				if cleanup != nil {
					t.Error("Expected nil cleanup function, got non-nil")
				}
			}
		})
	}
}

func TestProvider_ExportedVariable(t *testing.T) {
	if Provider == (schemav2ValidatorProvider{}) {
		t.Log("Provider variable is properly exported")
	} else {
		t.Error("Provider variable has unexpected value")
	}
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
