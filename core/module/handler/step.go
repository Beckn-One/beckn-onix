package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"
	"github.com/beckn-one/beckn-onix/pkg/plugin/definition"
)

// signStep represents the signing step in the processing pipeline.
type signStep struct {
	signer definition.Signer
	km     definition.KeyManager
}

// newSignStep initializes and returns a new signing step.
func newSignStep(signer definition.Signer, km definition.KeyManager) (definition.Step, error) {
	if signer == nil {
		return nil, fmt.Errorf("invalid config: Signer plugin not configured")
	}
	if km == nil {
		return nil, fmt.Errorf("invalid config: KeyManager plugin not configured")
	}

	return &signStep{signer: signer, km: km}, nil
}

// Run executes the signing step.
func (s *signStep) Run(ctx *model.StepContext) error {
	if len(ctx.SubID) == 0 {
		return model.NewBadReqErr(fmt.Errorf("subscriberID not set"))
	}
	keySet, err := s.km.Keyset(ctx, ctx.SubID)
	if err != nil {
		return fmt.Errorf("failed to get signing key: %w", err)
	}
	createdAt := time.Now().Unix()
	validTill := time.Now().Add(5 * time.Minute).Unix()
	sign, err := s.signer.Sign(ctx, ctx.Body, keySet.SigningPrivate, createdAt, validTill)
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	authHeader := s.generateAuthHeader(ctx.SubID, keySet.UniqueKeyID, createdAt, validTill, sign)
	log.Debugf(ctx, "Signature generated: %v", sign)
	header := model.AuthHeaderSubscriber
	if ctx.Role == model.RoleGateway {
		header = model.AuthHeaderGateway
	}
	ctx.Request.Header.Set(header, authHeader)
	return nil
}

// generateAuthHeader constructs the authorization header for the signed request.
// It includes key ID, algorithm, creation time, expiration time, required headers, and signature.
func (s *signStep) generateAuthHeader(subID, keyID string, createdAt, validTill int64, signature string) string {
	return fmt.Sprintf(
		"Signature keyId=\"%s|%s|ed25519\",algorithm=\"ed25519\",created=\"%d\",expires=\"%d\",headers=\"(created) (expires) digest\",signature=\"%s\"",
		subID, keyID, createdAt, validTill, signature,
	)
}

// validateSignStep represents the signature validation step.
type validateSignStep struct {
	validator definition.SignValidator
	km        definition.KeyManager
}

// newValidateSignStep initializes and returns a new validate sign step.
func newValidateSignStep(signValidator definition.SignValidator, km definition.KeyManager) (definition.Step, error) {
	if signValidator == nil {
		return nil, fmt.Errorf("invalid config: SignValidator plugin not configured")
	}
	if km == nil {
		return nil, fmt.Errorf("invalid config: KeyManager plugin not configured")
	}
	return &validateSignStep{validator: signValidator, km: km}, nil
}

// Run executes the validation step.
func (s *validateSignStep) Run(ctx *model.StepContext) error {
	unauthHeader := fmt.Sprintf("Signature realm=\"%s\",headers=\"(created) (expires) digest\"", ctx.SubID)
	headerValue := ctx.Request.Header.Get(model.AuthHeaderGateway)
	if len(headerValue) != 0 {
		log.Debugf(ctx, "Validating %v Header", model.AuthHeaderGateway)
		if err := s.validate(ctx, headerValue); err != nil {
			ctx.RespHeader.Set(model.UnaAuthorizedHeaderGateway, unauthHeader)
			return model.NewSignValidationErr(fmt.Errorf("failed to validate %s: %w", model.AuthHeaderGateway, err))
		}
	}

	log.Debugf(ctx, "Validating %v Header", model.AuthHeaderSubscriber)
	headerValue = ctx.Request.Header.Get(model.AuthHeaderSubscriber)
	if len(headerValue) == 0 {
		ctx.RespHeader.Set(model.UnaAuthorizedHeaderSubscriber, unauthHeader)
		return model.NewSignValidationErr(fmt.Errorf("%s missing", model.UnaAuthorizedHeaderSubscriber))
	}
	if err := s.validate(ctx, headerValue); err != nil {
		ctx.RespHeader.Set(model.UnaAuthorizedHeaderSubscriber, unauthHeader)
		return model.NewSignValidationErr(fmt.Errorf("failed to validate %s: %w", model.AuthHeaderSubscriber, err))
	}
	return nil
}

// validate checks the validity of the provided signature header.
func (s *validateSignStep) validate(ctx *model.StepContext, value string) error {
	headerVals, err := parseHeader(value)
	if err != nil {
		return fmt.Errorf("failed to parse header")
	}
	log.Debugf(ctx, "Validating Signature for subscriberID: %v", headerVals.SubscriberID)
	signingPublicKey, _, err := s.km.LookupNPKeys(ctx, headerVals.SubscriberID, headerVals.UniqueID)
	if err != nil {
		return fmt.Errorf("failed to get validation key: %w", err)
	}
	if err := s.validator.Validate(ctx, ctx.Body, value, signingPublicKey); err != nil {
		return fmt.Errorf("sign validation failed: %w", err)
	}
	return nil
}

// ParsedKeyID holds the components from the parsed Authorization header's keyId.
type authHeader struct {
	SubscriberID string
	UniqueID     string
	Algorithm    string
}

// keyID extracts subscriber_id and unique_key_id from the Authorization header.
// Example keyId format: "{subscriber_id}|{unique_key_id}|{algorithm}"
func parseHeader(header string) (*authHeader, error) {
	// Example: Signature keyId="bpp.example.com|key-1|ed25519",algorithm="ed25519",...
	keyIDPart := ""
	// Look for keyId="<value>"
	const keyIdPrefix = `keyId="`
	startIndex := strings.Index(header, keyIdPrefix)
	if startIndex != -1 {
		startIndex += len(keyIdPrefix)
		endIndex := strings.Index(header[startIndex:], `"`)
		if endIndex != -1 {
			keyIDPart = strings.TrimSpace(header[startIndex : startIndex+endIndex])
		}
	}

	if keyIDPart == "" {
		return nil, fmt.Errorf("keyId parameter not found in Authorization header")
	}

	keyIDComponents := strings.Split(keyIDPart, "|")
	if len(keyIDComponents) != 3 {
		return nil, fmt.Errorf("keyId parameter has incorrect format, expected 3 components separated by '|', got %d for '%s'", len(keyIDComponents), keyIDPart)
	}

	return &authHeader{
		SubscriberID: strings.TrimSpace(keyIDComponents[0]),
		UniqueID:     strings.TrimSpace(keyIDComponents[1]),
		Algorithm:    strings.TrimSpace(keyIDComponents[2]),
	}, nil
}

// validateSchemaStep represents the schema validation step.
type validateSchemaStep struct {
	validator definition.SchemaValidator
}

// newValidateSchemaStep creates and returns the validateSchema step after validation.
func newValidateSchemaStep(schemaValidator definition.SchemaValidator) (definition.Step, error) {
	if schemaValidator == nil {
		return nil, fmt.Errorf("invalid config: SchemaValidator plugin not configured")
	}
	log.Debug(context.Background(), "adding schema validator")
	return &validateSchemaStep{validator: schemaValidator}, nil
}

// Run executes the schema validation step.
func (s *validateSchemaStep) Run(ctx *model.StepContext) error {
	if err := s.validator.Validate(ctx, ctx.Request.URL, ctx.Body); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// addRouteStep represents the route determination step.
type addRouteStep struct {
	router definition.Router
}

// newAddRouteStep creates and returns the addRoute step after validation.
func newAddRouteStep(router definition.Router) (definition.Step, error) {
	if router == nil {
		return nil, fmt.Errorf("invalid config: Router plugin not configured")
	}
	return &addRouteStep{router: router}, nil
}

// Run executes the routing step.
func (s *addRouteStep) Run(ctx *model.StepContext) error {
	route, err := s.router.Route(ctx, ctx.Request.URL, ctx.Body)
	if err != nil {
		return fmt.Errorf("failed to determine route: %w", err)
	}
	ctx.Route = &model.Route{
		TargetType:  route.TargetType,
		PublisherID: route.PublisherID,
		URL:         route.URL,
	}
	return nil
}

// INFO: addOnsStep represents a generic container for optional add-on features
type addOnsStep struct {
	sync definition.Sync
}

func newAddOnsStep(sync definition.Sync) (definition.Step, error) {
	return &addOnsStep{sync: sync}, nil
}

func (s *addOnsStep) Run(ctx *model.StepContext) error {
	if s.sync == nil {
		log.Debugf(ctx, "Sync plugin not configured")
	} else {
		s.executeSyncPlugin(ctx)
	}

	//INFO: Add handling for new plugins here
	return nil
}

func (s *addOnsStep) executeSyncPlugin(ctx *model.StepContext) error {
	txnID, err := extractTransactionID(ctx.Body)
	if err != nil {
		log.Warnf(ctx, "Failed to extract transaction ID, skipping sync plugin: %v", err)
		return nil
	}

	callbackData, skipForwarding, err := s.sync.Execute(ctx, txnID, ctx.Body)
	if err != nil {
		log.Warnf(ctx, "SYNC PLUGIN FAILED for txnID: %s | Error: %v | FALLING BACK TO ASYNC FLOW", txnID, err)
		return nil
	}

	// INFO: BAP Caller case
	if callbackData != nil {
		log.Infof(ctx, "SYNC SUCCESS: Received callback data for txnID: %s ", txnID)
		ctx.CallbackResponse = callbackData
		return nil
	}

	// INFO: BAP Receiver case
	if skipForwarding {
		log.Infof(ctx, "SYNC SUCCESS: Callback published to waiting caller for txnID: %s", txnID)
		ctx.Route = nil
		return nil
	}

	log.Debugf(ctx, "Sync plugin completed - no sync operation needed for txnID: %s (no waiting caller) - continuing with async flow", txnID)
	return nil
}

// extractTransactionID extracts transaction_id from the request body
func extractTransactionID(body []byte) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse body: %w", err)
	}

	// Try context.transaction_id first (standard Beckn location)
	if context, ok := data["context"].(map[string]interface{}); ok {
		if txnID, ok := context["transaction_id"].(string); ok && txnID != "" {
			return txnID, nil
		}
	}

	// Fallback: Try message.order.id or other common locations
	if message, ok := data["message"].(map[string]interface{}); ok {
		if order, ok := message["order"].(map[string]interface{}); ok {
			if orderID, ok := order["id"].(string); ok && orderID != "" {
				return orderID, nil
			}
		}
	}

	return "", fmt.Errorf("transaction_id not found in body")
}
