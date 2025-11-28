package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"
	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
)

// Config holds the sync plugin configuration
type Config struct {
	Enabled       bool          // Enable/disable sync functionality (default: false)
	Timeout       time.Duration // Max wait time for callback (default: 30s)
	ChannelPrefix string        // Redis channel prefix (default: "sync:")
	KeyTTL        time.Duration // Redis key TTL (default: 35s)
}

// syncImpl implements the Sync interface
type syncImpl struct {
	cache  definition.Cache // Cache plugin for Redis operations
	role   string           // "bap_caller" or "bap_receiver"
	config *Config
}

// New creates a new sync plugin instance
func New(ctx context.Context, cache definition.Cache, role string, cfg *Config) (definition.Sync, func() error, error) {
	if cache == nil {
		return nil, nil, fmt.Errorf("cache plugin is required for sync plugin")
	}

	if cfg == nil {
		cfg = &Config{
			Enabled:       false, // Default: disabled
			Timeout:       30 * time.Second,
			ChannelPrefix: "sync:",
			KeyTTL:        35 * time.Second,
		}
	}

	// Set defaults if not provided
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.ChannelPrefix == "" {
		cfg.ChannelPrefix = "sync:"
	}
	if cfg.KeyTTL == 0 {
		cfg.KeyTTL = 35 * time.Second
	}

	if cfg.Enabled {
		log.Infof(ctx, "Initializing Sync plugin for role: %s (enabled, timeout: %s)", role, cfg.Timeout)
	} else {
		log.Infof(ctx, "Initializing Sync plugin for role: %s (disabled)", role)
	}

	return &syncImpl{
		cache:  cache,
		role:   role,
		config: cfg,
	}, nil, nil
}

// Execute performs role-specific sync operations
// NOTE: ctx is actually *model.StepContext, not context.Context
func (s *syncImpl) Execute(ctx context.Context, txnID string, data []byte) ([]byte, bool, error) {
	// Check if sync is enabled
	if !s.config.Enabled {
		log.Debugf(ctx, "Sync plugin is disabled, skipping")
		return nil, false, nil
	}

	switch s.role {
	case "bap_caller":
		return s.handleBAPCaller(ctx, txnID, data)
	case "bap_receiver":
		return s.handleBAPReceiver(ctx, txnID, data)
	default:
		return nil, false, fmt.Errorf("unknown role: %s", s.role)
	}
}

// handleBAPCaller waits for callback from Redis
func (s *syncImpl) handleBAPCaller(ctx context.Context, txnID string, data []byte) ([]byte, bool, error) {
	// Extract message_id and action from data
	var reqData map[string]interface{}
	if err := json.Unmarshal(data, &reqData); err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to parse request data for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: failed to parse request data: %w", err)
	}

	contextData, ok := reqData["context"].(map[string]interface{})
	if !ok {
		log.Errorf(ctx, nil, "[SYNC PLUGIN ERROR] Context not found in request for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: context not found in request")
	}

	messageID, _ := contextData["message_id"].(string)
	action, _ := contextData["action"].(string)

	if messageID == "" || action == "" {
		log.Errorf(ctx, nil, "[SYNC PLUGIN ERROR] Missing message_id or action in context for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: message_id or action not found in context")
	}

	callbackAction := "on_" + action
	channel := fmt.Sprintf("%s%s:%s:%s", s.config.ChannelPrefix, txnID, messageID, callbackAction)

	log.Infof(ctx, "[SYNC PLUGIN] BAP Caller: Subscribing and waiting for callback on channel: %s (timeout: %s)", channel, s.config.Timeout)

	// INFO: Subscribe to Redis channel FIRST
	subscription := s.cache.Subscribe(ctx, channel)
	if subscription == nil {
		log.Errorf(ctx, nil, "[SYNC PLUGIN ERROR] Failed to subscribe to Redis channel: %s - Redis may be down - falling back to async flow", channel)
		return nil, false, fmt.Errorf("sync plugin: failed to subscribe to Redis channel (Redis connection error)")
	}
	defer subscription.Close()

	// INFO: Set waiting flag in Redis
	err := s.cache.Set(ctx, channel, "waiting", s.config.KeyTTL)
	if err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to set waiting flag in Redis for channel: %s - Redis connection error - falling back to async flow", channel)
		return nil, false, fmt.Errorf("sync plugin: failed to set waiting flag (Redis error): %w", err)
	}

	// INFO: TRIGGER FORWARDING IN BACKGROUND
	stepCtx, ok := ctx.(*model.StepContext)
	if !ok {
		log.Warnf(ctx, "[SYNC PLUGIN WARNING] Context is not *model.StepContext - cannot access Route info for background forwarding. Falling back to async flow.")
		// Cleanup
		s.cache.Delete(ctx, channel)
		return nil, false, nil
	}

	if stepCtx.Route == nil || stepCtx.Route.URL == nil {
		log.Warnf(ctx, "[SYNC PLUGIN WARNING] Route URL not found in context - cannot forward request. Falling back to async flow.")
		// Cleanup
		s.cache.Delete(ctx, channel)
		return nil, false, nil
	}

	targetURL := stepCtx.Route.URL.String()
	log.Infof(ctx, "[SYNC PLUGIN] Starting background forwarding to: %s", targetURL)

	go func() {
		// Create a new context for the background request since the main ctx might be cancelled if we timeout
		bgCtx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(bgCtx, "POST", targetURL, bytes.NewBuffer(data))
		if err != nil {
			log.Errorf(ctx, err, "[SYNC PLUGIN BACKGROUND ERROR] Failed to create request for forwarding to %s", targetURL)
			return
		}

		for k, v := range stepCtx.Request.Header {
			req.Header[k] = v
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: s.config.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			log.Errorf(ctx, err, "[SYNC PLUGIN BACKGROUND ERROR] Failed to forward request to %s", targetURL)
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		log.Infof(ctx, "[SYNC PLUGIN BACKGROUND] Forwarding completed. Status: %s, Response: %s", resp.Status, string(respBody))
	}()

	// INFO: Wait for callback with timeout
	select {
	case msg := <-subscription.Channel():
		// SUCCESS: Callback received
		log.Infof(ctx, "[SYNC PLUGIN SUCCESS] Received callback for txnID: %s (size: %d bytes) - returning synchronous response", txnID, len(msg.Payload))

		// Return true for skipForwarding because we already forwarded it in background!
		return []byte(msg.Payload), true, nil

	case <-time.After(s.config.Timeout):
		// TIMEOUT: No callback received
		log.Warnf(ctx, "[SYNC PLUGIN TIMEOUT] No callback received for txnID: %s after %s - returning timeout error", txnID, s.config.Timeout)

		// Cleanup: Delete waiting flag
		if err := s.cache.Delete(ctx, channel); err != nil {
			log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to cleanup Redis key on timeout for channel: %s", channel)
		}

		return nil, true, fmt.Errorf("sync plugin: timeout waiting for callback after %s", s.config.Timeout)

	case <-ctx.Done():
		// CANCELLED: Context cancelled
		log.Warnf(ctx, "[SYNC PLUGIN CANCELLED] Context cancelled while waiting for callback for txnID: %s", txnID)

		// Cleanup
		if err := s.cache.Delete(ctx, channel); err != nil {
			log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to cleanup Redis key on cancellation for channel: %s", channel)
		}

		return nil, true, ctx.Err()
	}
}

// handleBAPReceiver publishes callback to waiting caller
func (s *syncImpl) handleBAPReceiver(ctx context.Context, txnID string, data []byte) ([]byte, bool, error) {
	// Extract message_id and action from callback data
	var callbackData map[string]interface{}
	if err := json.Unmarshal(data, &callbackData); err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to parse callback data for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: failed to parse callback data: %w", err)
	}

	contextData, ok := callbackData["context"].(map[string]interface{})
	if !ok {
		log.Errorf(ctx, nil, "[SYNC PLUGIN ERROR] Context not found in callback for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: context not found in callback")
	}

	messageID, _ := contextData["message_id"].(string)
	action, _ := contextData["action"].(string)

	if messageID == "" || action == "" {
		log.Errorf(ctx, nil, "[SYNC PLUGIN ERROR] Missing message_id or action in callback context for txnID: %s - falling back to async flow", txnID)
		return nil, false, fmt.Errorf("sync plugin: message_id or action not found in context")
	}

	// Build Redis key: sync:{transaction_id}:{message_id}:{action}
	channel := fmt.Sprintf("%s%s:%s:%s", s.config.ChannelPrefix, txnID, messageID, action)

	log.Debugf(ctx, "[SYNC PLUGIN] BAP Receiver: Checking for waiting caller on channel: %s", channel)

	// INFO: Check if anyone is waiting
	exists, err := s.cache.Exists(ctx, channel)
	if err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to check Redis key existence for channel: %s - Redis connection error - falling back to async flow (will forward to backend)", channel)
		return nil, false, fmt.Errorf("sync plugin: failed to check for waiting caller (Redis error): %w", err)
	}

	if !exists {
		// INFO: No one waiting - normal flow (forward to backend)
		log.Debugf(ctx, "[SYNC PLUGIN] No waiting caller for txnID: %s on channel: %s - continuing normal async flow (will forward to backend)", txnID, channel)
		return nil, false, nil
	}

	// INFO: Someone is waiting - publish callback to Redis
	log.Infof(ctx, "[SYNC PLUGIN] Found waiting caller for txnID: %s, publishing callback to channel: %s", txnID, channel)

	err = s.cache.Publish(ctx, channel, data)
	if err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN ERROR] Failed to publish callback to Redis channel: %s - Redis connection error - falling back to async flow (will forward to backend)", channel)
		return nil, false, fmt.Errorf("sync plugin: failed to publish callback (Redis error): %w", err)
	}

	// INFO: Cleanup: Delete the waiting flag
	err = s.cache.Delete(ctx, channel)
	if err != nil {
		log.Errorf(ctx, err, "[SYNC PLUGIN WARNING] Failed to delete Redis key after publishing for channel: %s - callback was published successfully, but cleanup failed", channel)
		// Don't fail - callback already published
	}

	log.Infof(ctx, "[SYNC PLUGIN SUCCESS] Successfully published callback via Redis for txnID: %s - skipping backend forwarding", txnID)

	// INFO: Signal to skip forwarding to backend
	return nil, true, nil
}
