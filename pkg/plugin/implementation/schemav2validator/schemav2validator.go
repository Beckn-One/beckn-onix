package schemav2validator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"

	"github.com/getkin/kin-openapi/openapi3"
)

// payload represents the structure of the data payload with context information.
type payload struct {
	Context struct {
		Action string `json:"action"`
	} `json:"context"`
}

// schemav2Validator implements the SchemaValidator interface.
type schemav2Validator struct {
	config    *Config
	spec      *cachedSpec
	specMutex sync.RWMutex
}

// cachedSpec holds a cached OpenAPI spec.
type cachedSpec struct {
	doc           *openapi3.T
	actionSchemas map[string]*openapi3.SchemaRef // O(1) action lookup
	loadedAt      time.Time
}

// Config struct for Schemav2Validator.
type Config struct {
	Type     string // "url", "file", or "dir"
	Location string // URL, file path, or directory path
	CacheTTL int
}

// New creates a new Schemav2Validator instance.
func New(ctx context.Context, config *Config) (*schemav2Validator, func() error, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("config cannot be nil")
	}
	if config.Type == "" {
		return nil, nil, fmt.Errorf("config type cannot be empty")
	}
	if config.Location == "" {
		return nil, nil, fmt.Errorf("config location cannot be empty")
	}
	if config.Type != "url" && config.Type != "file" && config.Type != "dir" {
		return nil, nil, fmt.Errorf("config type must be 'url', 'file', or 'dir'")
	}

	if config.CacheTTL == 0 {
		config.CacheTTL = 3600
	}

	v := &schemav2Validator{
		config: config,
	}

	if err := v.initialise(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to initialise schemav2Validator: %v", err)
	}

	go v.refreshLoop(ctx)

	return v, nil, nil
}

// Validate validates the given data against the OpenAPI schema.
func (v *schemav2Validator) Validate(ctx context.Context, reqURL *url.URL, data []byte) error {
	var payloadData payload
	err := json.Unmarshal(data, &payloadData)
	if err != nil {
		return model.NewBadReqErr(fmt.Errorf("failed to parse JSON payload: %v", err))
	}

	if payloadData.Context.Action == "" {
		return model.NewBadReqErr(fmt.Errorf("missing field Action in context"))
	}

	v.specMutex.RLock()
	spec := v.spec
	v.specMutex.RUnlock()

	if spec == nil || spec.doc == nil {
		return model.NewBadReqErr(fmt.Errorf("no OpenAPI spec loaded"))
	}

	action := payloadData.Context.Action

	// O(1) lookup from action index
	schema := spec.actionSchemas[action]
	if schema == nil || schema.Value == nil {
		return model.NewBadReqErr(fmt.Errorf("unsupported action: %s", action))
	}

	log.Debugf(ctx, "Validating action: %s", action)

	var jsonData any
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return model.NewBadReqErr(fmt.Errorf("invalid JSON: %v", err))
	}

	opts := []openapi3.SchemaValidationOption{
		openapi3.VisitAsRequest(),
		openapi3.MultiErrors(),
		openapi3.EnableFormatValidation(),
	}
	if err := schema.Value.VisitJSON(jsonData, opts...); err != nil {
		log.Debugf(ctx, "Schema validation failed: %v", err)
		return v.formatValidationError(err)
	}

	return nil
}

// initialise loads the OpenAPI spec from the configuration.
func (v *schemav2Validator) initialise(ctx context.Context) error {
	return v.loadSpec(ctx)
}

// loadSpec loads the OpenAPI spec from URL or local path.
func (v *schemav2Validator) loadSpec(ctx context.Context) error {
	loader := openapi3.NewLoader()

	// Allow external references
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error

	switch v.config.Type {
	case "url":
		u, parseErr := url.Parse(v.config.Location)
		if parseErr != nil {
			return fmt.Errorf("failed to parse URL: %v", parseErr)
		}
		doc, err = loader.LoadFromURI(u)
	case "file":
		doc, err = loader.LoadFromFile(v.config.Location)
	case "dir":
		return fmt.Errorf("directory loading not yet implemented")
	default:
		return fmt.Errorf("unsupported type: %s", v.config.Type)
	}

	if err != nil {
		log.Errorf(ctx, err, "Failed to load from %s: %s", v.config.Type, v.config.Location)
		return fmt.Errorf("failed to load OpenAPI document: %v", err)
	}

	// Validate spec (skip strict validation to allow JSON Schema keywords)
	if err := doc.Validate(ctx); err != nil {
		log.Debugf(ctx, "Spec validation warnings (non-fatal): %v", err)
	} else {
		log.Debugf(ctx, "Spec validation passed")
	}

	// Build action→schema index for O(1) lookup
	actionSchemas := v.buildActionIndex(ctx, doc)

	v.specMutex.Lock()
	v.spec = &cachedSpec{
		doc:           doc,
		actionSchemas: actionSchemas,
		loadedAt:      time.Now(),
	}
	v.specMutex.Unlock()

	log.Debugf(ctx, "Loaded OpenAPI spec from %s: %s with %d actions indexed", v.config.Type, v.config.Location, len(actionSchemas))
	return nil
}

// refreshLoop periodically reloads expired specs based on TTL.
func (v *schemav2Validator) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(v.config.CacheTTL) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.reloadExpiredSpec(ctx)
		}
	}
}

// reloadExpiredSpec reloads spec if it has exceeded its TTL.
func (v *schemav2Validator) reloadExpiredSpec(ctx context.Context) {
	v.specMutex.RLock()
	if v.spec == nil {
		v.specMutex.RUnlock()
		return
	}
	needsReload := time.Since(v.spec.loadedAt) >= time.Duration(v.config.CacheTTL)*time.Second
	v.specMutex.RUnlock()

	if needsReload {
		if err := v.loadSpec(ctx); err != nil {
			log.Errorf(ctx, err, "Failed to reload spec")
		} else {
			log.Debugf(ctx, "Reloaded spec from %s: %s", v.config.Type, v.config.Location)
		}
	}
}

// formatValidationError converts kin-openapi validation errors to ONIX error format.
func (v *schemav2Validator) formatValidationError(err error) error {
	var schemaErrors []model.Error

	// Check if it's a MultiError (collection of errors)
	if multiErr, ok := err.(openapi3.MultiError); ok {
		for _, e := range multiErr {
			v.extractSchemaErrors(e, &schemaErrors)
		}
	} else {
		v.extractSchemaErrors(err, &schemaErrors)
	}

	return &model.SchemaValidationErr{Errors: schemaErrors}
}

// extractSchemaErrors recursively extracts detailed error information from SchemaError.
func (v *schemav2Validator) extractSchemaErrors(err error, schemaErrors *[]model.Error) {
	if schemaErr, ok := err.(*openapi3.SchemaError); ok {
		// If there's an origin error, recursively extract from it
		if schemaErr.Origin != nil {
			v.extractSchemaErrors(schemaErr.Origin, schemaErrors)
		} else {
			// Leaf error - extract the actual validation failure
			pathParts := schemaErr.JSONPointer()
			path := strings.Join(pathParts, "/")
			if path == "" {
				path = schemaErr.SchemaField
			}
			*schemaErrors = append(*schemaErrors, model.Error{
				Paths:   path,
				Message: schemaErr.Reason,
			})
		}
	} else if multiErr, ok := err.(openapi3.MultiError); ok {
		// Nested MultiError
		for _, e := range multiErr {
			v.extractSchemaErrors(e, schemaErrors)
		}
	} else {
		// Generic error
		*schemaErrors = append(*schemaErrors, model.Error{
			Paths:   "",
			Message: err.Error(),
		})
	}
}

// buildActionIndex builds a map of action→schema for O(1) lookup.
func (v *schemav2Validator) buildActionIndex(ctx context.Context, doc *openapi3.T) map[string]*openapi3.SchemaRef {
	actionSchemas := make(map[string]*openapi3.SchemaRef)

	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		// Check all HTTP methods
		for _, op := range []*openapi3.Operation{item.Post, item.Get, item.Put, item.Patch, item.Delete} {
			if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			content := op.RequestBody.Value.Content.Get("application/json")
			if content == nil || content.Schema == nil || content.Schema.Value == nil {
				continue
			}

			// Extract action from schema
			action := v.extractActionFromSchema(content.Schema.Value)
			if action != "" {
				actionSchemas[action] = content.Schema
				log.Debugf(ctx, "Indexed action '%s' from path %s", action, path)
			}
		}
	}

	return actionSchemas
}

// extractActionFromSchema extracts the action value from a schema.
func (v *schemav2Validator) extractActionFromSchema(schema *openapi3.Schema) string {
	// Check direct properties
	if ctxProp := schema.Properties["context"]; ctxProp != nil && ctxProp.Value != nil {
		if action := v.getActionValue(ctxProp.Value); action != "" {
			return action
		}
	}

	// Check allOf at schema level
	for _, allOfSchema := range schema.AllOf {
		if allOfSchema.Value != nil {
			if ctxProp := allOfSchema.Value.Properties["context"]; ctxProp != nil && ctxProp.Value != nil {
				if action := v.getActionValue(ctxProp.Value); action != "" {
					return action
				}
			}
		}
	}

	return ""
}

// getActionValue extracts action value from context schema.
func (v *schemav2Validator) getActionValue(contextSchema *openapi3.Schema) string {
	if actionProp := contextSchema.Properties["action"]; actionProp != nil && actionProp.Value != nil {
		// Check const field
		if constVal, ok := actionProp.Value.Extensions["const"]; ok {
			if action, ok := constVal.(string); ok {
				return action
			}
		}
		// Check enum field (return first value)
		if len(actionProp.Value.Enum) > 0 {
			if action, ok := actionProp.Value.Enum[0].(string); ok {
				return action
			}
		}
	}

	// Check allOf in context
	for _, allOfSchema := range contextSchema.AllOf {
		if allOfSchema.Value != nil {
			if action := v.getActionValue(allOfSchema.Value); action != "" {
				return action
			}
		}
	}

	return ""
}
