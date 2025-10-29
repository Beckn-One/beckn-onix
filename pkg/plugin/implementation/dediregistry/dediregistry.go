package dediregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"
	"github.com/hashicorp/go-retryablehttp"
)

// Config holds configuration parameters for the DeDi registry client.
type Config struct {
	BaseURL string `yaml:"baseURL" json:"baseURL"`
	Timeout int    `yaml:"timeout" json:"timeout"`
}

// DeDiRegistryClient encapsulates the logic for calling the DeDi registry endpoints.
type DeDiRegistryClient struct {
	config *Config
	client *retryablehttp.Client
}

// validate checks if the provided DeDi registry configuration is valid.
func validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("DeDi registry config cannot be nil")
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("baseURL cannot be empty")
	}
	return nil
}

// New creates a new instance of DeDiRegistryClient.
func New(ctx context.Context, cfg *Config) (*DeDiRegistryClient, func() error, error) {
	log.Debugf(ctx, "Initializing DeDi Registry client with config: %+v", cfg)

	if err := validate(cfg); err != nil {
		return nil, nil, err
	}

	retryClient := retryablehttp.NewClient()

	// Configure timeout if provided
	if cfg.Timeout > 0 {
		retryClient.HTTPClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	client := &DeDiRegistryClient{
		config: cfg,
		client: retryClient,
	}

	// Cleanup function
	closer := func() error {
		log.Debugf(ctx, "Cleaning up DeDi Registry client resources")
		if client.client != nil {
			client.client.HTTPClient.CloseIdleConnections()
		}
		return nil
	}

	log.Infof(ctx, "DeDi Registry client connection established successfully")
	return client, closer, nil
}

// Lookup implements RegistryLookup interface - calls the DeDi wrapper lookup endpoint and returns Subscription.
func (c *DeDiRegistryClient) Lookup(ctx context.Context, req *model.Subscription) ([]model.Subscription, error) {
	// Extract subscriber ID and key ID from request (both come from Authorization header parsing)
	subscriberID := req.SubscriberID
	keyID := req.KeyID
	log.Infof(ctx, "DeDi Registry: Looking up subscriber ID: %s, key ID: %s", subscriberID, keyID)
	if subscriberID == "" {
		return nil, fmt.Errorf("subscriber_id is required for DeDi lookup")
	}
	if keyID == "" {
		return nil, fmt.Errorf("key_id is required for DeDi lookup")
	}

	lookupURL := fmt.Sprintf("%s/dedi/lookup/%s/subscribers.beckn.one/%s",
		c.config.BaseURL, subscriberID, keyID)

	httpReq, err := retryablehttp.NewRequest("GET", lookupURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq = httpReq.WithContext(ctx)

	log.Debugf(ctx, "Making DeDi lookup request to: %s", lookupURL)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send DeDi lookup request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Errorf(ctx, nil, "DeDi lookup request failed with status: %s, response: %s", resp.Status, string(body))
		return nil, fmt.Errorf("DeDi lookup request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	log.Debugf(ctx, "DeDi lookup request successful, parsing response")

	// Extract data field
	data, ok := responseData["data"].(map[string]interface{})
	if !ok {
		log.Errorf(ctx, nil, "Invalid DeDi response format: missing or invalid data field")
		return nil, fmt.Errorf("invalid response format: missing data field")
	}

	// Extract details field
	details, ok := data["details"].(map[string]interface{})
	if !ok {
		log.Errorf(ctx, nil, "Invalid DeDi response format: missing or invalid details field")
		return nil, fmt.Errorf("invalid response format: missing details field")
	}

	// Extract required fields from details
	signingPublicKey, ok := details["signing_public_key"].(string)
	if !ok || signingPublicKey == "" {
		return nil, fmt.Errorf("invalid or missing signing_public_key in response")
	}

	// Extract fields from details
	detailsURL, _ := details["url"].(string)
	detailsType, _ := details["type"].(string)
	detailsDomain, _ := details["domain"].(string)
	detailsSubscriberID, _ := details["subscriber_id"].(string)

	// Extract encr_public_key if available (optional field)
	encrPublicKey, _ := details["encr_public_key"].(string)

	// Extract fields from data level
	createdAt, _ := data["created_at"].(string)
	updatedAt, _ := data["updated_at"].(string)
	isRevoked, _ := data["is_revoked"].(bool)

	// Determine status from is_revoked flag
	status := "SUBSCRIBED"
	if isRevoked {
		status = "UNSUBSCRIBED"
	}

	// Convert to Subscription format
	subscription := model.Subscription{
		Subscriber: model.Subscriber{
			SubscriberID: detailsSubscriberID,
			URL:          detailsURL,
			Domain:       detailsDomain,
			Type:         detailsType,
		},
		KeyID:            keyID, // Use original keyID from request
		SigningPublicKey: signingPublicKey,
		EncrPublicKey:    encrPublicKey, // May be empty if not provided
		Status:           status,
		Created:          parseTime(createdAt),
		Updated:          parseTime(updatedAt),
	}

	log.Debugf(ctx, "DeDi lookup successful, found subscription for subscriber: %s", detailsSubscriberID)
	return []model.Subscription{subscription}, nil
}

// parseTime converts string timestamp to time.Time
func parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}
	}
	return parsedTime
}
