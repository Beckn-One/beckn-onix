package reqmapper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func resetEngineState(t *testing.T) {
	t.Helper()
	engineInstance = nil
	engineOnce = sync.Once{}
}

func testMappingsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "mappings.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("test mappings file missing: %v", err)
	}
	return path
}

func initTestEngine(t *testing.T) *MappingEngine {
	t.Helper()
	resetEngineState(t)
	engine, err := initMappingEngine(&Config{
		Role:         "bap",
		MappingsFile: testMappingsFile(t),
	})
	if err != nil {
		t.Fatalf("failed to init mapping engine: %v", err)
	}
	return engine
}

func TestNewReqMapper_InvalidConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		if _, err := NewReqMapper(nil); err == nil {
			t.Fatalf("expected error for nil config")
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		if _, err := NewReqMapper(&Config{Role: "invalid"}); err == nil {
			t.Fatalf("expected error for invalid role")
		}
	})
}

func TestNewReqMapper_MiddlewareTransformsRequest(t *testing.T) {
	resetEngineState(t)
	mw, err := NewReqMapper(&Config{
		Role:         "bap",
		MappingsFile: testMappingsFile(t),
	})
	if err != nil {
		t.Fatalf("NewReqMapper returned error: %v", err)
	}

	startLocation := map[string]interface{}{
		"gps":  "12.9716,77.5946",
		"city": "Bengaluru",
	}
	endLocation := map[string]interface{}{
		"gps":  "13.0827,80.2707",
		"city": "Chennai",
	}

	requestPayload := map[string]interface{}{
		"context": map[string]interface{}{
			"domain":         "retail",
			"action":         "search",
			"version":        "1.1.0",
			"bap_id":         "bap.example",
			"bap_uri":        "https://bap.example/api",
			"transaction_id": "txn-1",
			"message_id":     "msg-1",
			"timestamp":      "2023-01-01T10:00:00Z",
		},
		"message": map[string]interface{}{
			"intent": map[string]interface{}{
				"item": map[string]interface{}{
					"id": "item-1",
				},
				"provider": map[string]interface{}{
					"id": "provider-1",
				},
				"fulfillment": map[string]interface{}{
					"start": map[string]interface{}{
						"location": startLocation,
					},
					"end": map[string]interface{}{
						"location": endLocation,
					},
				},
			},
		},
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		t.Fatalf("failed to marshal request payload: %v", err)
	}

	var captured map[string]interface{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request in handler: %v", err)
		}
		if err := json.Unmarshal(data, &captured); err != nil {
			t.Fatalf("failed to unmarshal transformed payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if captured == nil {
		t.Fatalf("middleware did not forward request to next handler")
	}

	message, ok := captured["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected message field in transformed payload")
	}

	filters, ok := message["filters"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected filters in transformed payload")
	}

	if pickup := filters["pickup"]; !reflect.DeepEqual(pickup, startLocation) {
		t.Fatalf("pickup location mismatch\ngot: %#v\nwant: %#v", pickup, startLocation)
	}
	if drop := filters["drop"]; !reflect.DeepEqual(drop, endLocation) {
		t.Fatalf("drop location mismatch\ngot: %#v\nwant: %#v", drop, endLocation)
	}
}

func TestMappingEngine_TransformFallbackForUnknownAction(t *testing.T) {
	engine := initTestEngine(t)
	req := map[string]interface{}{
		"context": map[string]interface{}{
			"action": "unknown_action",
		},
		"message": map[string]interface{}{},
	}

	expected, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal expected payload: %v", err)
	}

	result, err := engine.Transform(context.Background(), "unknown_action", req, "bap")
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	if !bytes.Equal(result, expected) {
		t.Fatalf("expected Transform to return original payload")
	}
}

func TestMappingEngine_TransformFallbackForUnknownRole(t *testing.T) {
	engine := initTestEngine(t)
	req := map[string]interface{}{
		"context": map[string]interface{}{
			"action": "search",
		},
		"message": map[string]interface{}{},
	}

	expected, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal expected payload: %v", err)
	}

	result, err := engine.Transform(context.Background(), "search", req, "unknown-role")
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}

	if !bytes.Equal(result, expected) {
		t.Fatalf("expected Transform to return original payload when role is unknown")
	}
}

func TestMappingEngine_ReloadMappings(t *testing.T) {
	engine := initTestEngine(t)

	engine.mutex.RLock()
	originalBAP := len(engine.bapMaps)
	originalBPP := len(engine.bppMaps)
	engine.mutex.RUnlock()

	if originalBAP == 0 || originalBPP == 0 {
		t.Fatalf("expected test mappings to be loaded")
	}

	engine.mutex.Lock()
	for action := range engine.bapMaps {
		delete(engine.bapMaps, action)
		break
	}
	engine.mutex.Unlock()

	engine.mutex.RLock()
	if len(engine.bapMaps) == originalBAP {
		engine.mutex.RUnlock()
		t.Fatalf("expected BAP map to be altered before reload")
	}
	engine.mutex.RUnlock()

	if err := engine.ReloadMappings(); err != nil {
		t.Fatalf("ReloadMappings returned error: %v", err)
	}

	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	if len(engine.bapMaps) != originalBAP {
		t.Fatalf("expected BAP mappings to be reloaded, got %d want %d", len(engine.bapMaps), originalBAP)
	}
	if len(engine.bppMaps) != originalBPP {
		t.Fatalf("expected BPP mappings to be reloaded, got %d want %d", len(engine.bppMaps), originalBPP)
	}
}
