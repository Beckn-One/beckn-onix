package schemav2validator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const testSpec = `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /search:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [context, message]
              properties:
                context:
                  type: object
                  required: [action]
                  properties:
                    action:
                      const: search
                    domain:
                      type: string
                message:
                  type: object
  /select:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [context, message]
              properties:
                context:
                  allOf:
                    - type: object
                      properties:
                        action:
                          enum: [select]
                message:
                  type: object
                  required: [order]
                  properties:
                    order:
                      type: object
`

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{"nil config", nil, true},
		{"empty type", &Config{Type: "", Location: "http://example.com"}, true},
		{"empty location", &Config{Type: "url", Location: ""}, true},
		{"invalid type", &Config{Type: "invalid", Location: "http://example.com"}, true},
		{"invalid URL", &Config{Type: "url", Location: "http://invalid-domain-12345.com/spec.yaml"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := New(context.Background(), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ActionExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testSpec))
	}))
	defer server.Close()

	validator, _, err := New(context.Background(), &Config{Type: "url", Location: server.URL, CacheTTL: 3600})
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		payload string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid search action",
			payload: `{"context":{"action":"search","domain":"retail"},"message":{}}`,
			wantErr: false,
		},
		{
			name:    "valid select action with allOf",
			payload: `{"context":{"action":"select"},"message":{"order":{}}}`,
			wantErr: false,
		},
		{
			name:    "missing action",
			payload: `{"context":{},"message":{}}`,
			wantErr: true,
			errMsg:  "missing field Action",
		},
		{
			name:    "unsupported action",
			payload: `{"context":{"action":"unknown"},"message":{}}`,
			wantErr: true,
			errMsg:  "unsupported action: unknown",
		},
		{
			name:    "action as number",
			payload: `{"context":{"action":123},"message":{}}`,
			wantErr: true,
			errMsg:  "failed to parse JSON payload",
		},
		{
			name:    "invalid JSON",
			payload: `{invalid json}`,
			wantErr: true,
			errMsg:  "failed to parse JSON payload",
		},
		{
			name:    "missing required field",
			payload: `{"context":{"action":"search"}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), nil, []byte(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidate_NestedValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testSpec))
	}))
	defer server.Close()

	validator, _, err := New(context.Background(), &Config{Type: "url", Location: server.URL, CacheTTL: 3600})
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name:    "select missing required order",
			payload: `{"context":{"action":"select"},"message":{}}`,
			wantErr: true,
		},
		{
			name:    "select with order",
			payload: `{"context":{"action":"select"},"message":{"order":{}}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), nil, []byte(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSpec_LocalFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-spec-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(testSpec)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	validator, _, err := New(context.Background(), &Config{Type: "file", Location: tmpFile.Name(), CacheTTL: 3600})
	if err != nil {
		t.Fatalf("Failed to load local spec: %v", err)
	}

	validator.specMutex.RLock()
	defer validator.specMutex.RUnlock()

	if validator.spec == nil || validator.spec.doc == nil {
		t.Error("Spec not loaded from local file")
	}
}

func TestCacheTTL_DefaultValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testSpec))
	}))
	defer server.Close()

	validator, _, err := New(context.Background(), &Config{Type: "url", Location: server.URL})
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	if validator.config.CacheTTL != 3600 {
		t.Errorf("Expected default CacheTTL 3600, got %d", validator.config.CacheTTL)
	}
}

func TestValidate_EdgeCases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testSpec))
	}))
	defer server.Close()

	validator, _, err := New(context.Background(), &Config{Type: "url", Location: server.URL, CacheTTL: 3600})
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name:    "empty payload",
			payload: `{}`,
			wantErr: true,
		},
		{
			name:    "null context",
			payload: `{"context":null,"message":{}}`,
			wantErr: true,
		},
		{
			name:    "empty string action",
			payload: `{"context":{"action":""},"message":{}}`,
			wantErr: true,
		},
		{
			name:    "action with whitespace",
			payload: `{"context":{"action":" search "},"message":{}}`,
			wantErr: true,
		},
		{
			name:    "case sensitive action",
			payload: `{"context":{"action":"Search"},"message":{}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), nil, []byte(tt.payload))
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
