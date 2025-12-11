package schemav2validator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	config      *Config
	spec        *cachedSpec
	specMutex   sync.RWMutex
	schemaCache *schemaCache // NEW: cache for referenced schemas
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

	// NEW: Referenced schema configuration
	EnableReferencedSchemas bool
	ReferencedSchemaConfig  ReferencedSchemaConfig
}

// ReferencedSchemaConfig holds configuration for referenced schema validation.
type ReferencedSchemaConfig struct {
	CacheTTL        int      // seconds, default 86400 (24h)
	MaxCacheSize    int      // default 100
	DownloadTimeout int      // seconds, default 30
	AllowedDomains  []string // whitelist (empty = all allowed)
	URLTransform    string   // e.g. "context.jsonld->attributes.yaml"
}

// referencedObject represents ANY object with @context in the request.
type referencedObject struct {
	Path    string
	Context string
	Type    string
	Data    map[string]interface{}
}

// schemaCache caches loaded domain schemas with LRU eviction.
type schemaCache struct {
	mu      sync.RWMutex
	schemas map[string]*cachedDomainSchema
	maxSize int
}

// cachedDomainSchema holds a cached domain schema with metadata.
type cachedDomainSchema struct {
	doc          *openapi3.T
	loadedAt     time.Time
	expiresAt    time.Time
	lastAccessed time.Time
	accessCount  int64
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

	// NEW: Initialize referenced schema cache if enabled
	if config.EnableReferencedSchemas {
		maxSize := 100
		if config.ReferencedSchemaConfig.MaxCacheSize > 0 {
			maxSize = config.ReferencedSchemaConfig.MaxCacheSize
		}
		v.schemaCache = newSchemaCache(maxSize)
		log.Infof(ctx, "Initialized referenced schema cache with max size: %d", maxSize)
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
		openapi3.EnableFormatValidation(),
	}
	if err := schema.Value.VisitJSON(jsonData, opts...); err != nil {
		log.Debugf(ctx, "Schema validation failed: %v", err)
		return v.formatValidationError(err)
	}

	log.Debugf(ctx, "LEVEL 1 validation passed for action: %s", action)

	// NEW: LEVEL 2 - Referenced schema validation (if enabled)
	if v.config.EnableReferencedSchemas && v.schemaCache != nil {
		log.Debugf(ctx, "Starting LEVEL 2 validation for action: %s", action)
		if err := v.validateReferencedSchemas(ctx, jsonData); err != nil {
			// Level 2 failure - return error (same behavior as Level 1)
			log.Debugf(ctx, "LEVEL 2 validation failed for action %s: %v", action, err)
			return v.formatValidationError(err)
		}
		log.Debugf(ctx, "LEVEL 2 validation passed for action: %s", action)
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
	coreTicker := time.NewTicker(time.Duration(v.config.CacheTTL) * time.Second)
	defer coreTicker.Stop()

	// NEW: Ticker for referenced schema cleanup
	var refTicker *time.Ticker
	if v.config.EnableReferencedSchemas {
		ttl := v.config.ReferencedSchemaConfig.CacheTTL
		if ttl <= 0 {
			ttl = 86400 // Default 24 hours
		}
		refTicker = time.NewTicker(time.Duration(ttl) * time.Second)
		defer refTicker.Stop()
	}

	for {
		if refTicker != nil {
			select {
			case <-ctx.Done():
				return
			case <-coreTicker.C:
				v.reloadExpiredSpec(ctx)
			case <-refTicker.C:
				if v.schemaCache != nil {
					count := v.schemaCache.cleanupExpired()
					if count > 0 {
						log.Debugf(ctx, "Cleaned up %d expired referenced schemas", count)
					}
				}
			}
		} else {
			select {
			case <-ctx.Done():
				return
			case <-coreTicker.C:
				v.reloadExpiredSpec(ctx)
			}
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
		// Extract path from current error and message from Origin if available
		pathParts := schemaErr.JSONPointer()
		path := strings.Join(pathParts, "/")
		if path == "" {
			path = schemaErr.SchemaField
		}

		message := schemaErr.Reason
		if schemaErr.Origin != nil {
			originMsg := schemaErr.Origin.Error()
			// Extract specific field error from nested message
			if strings.Contains(originMsg, "Error at \"/") {
				// Find last "Error at" which has the specific field error
				parts := strings.Split(originMsg, "Error at \"")
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					// Extract field path and update both path and message
					if idx := strings.Index(lastPart, "\":"); idx > 0 {
						fieldPath := lastPart[:idx]
						fieldMsg := strings.TrimSpace(lastPart[idx+2:])
						path = strings.TrimPrefix(fieldPath, "/")
						message = fieldMsg
					}
				}
			} else {
				message = originMsg
			}
		}

		*schemaErrors = append(*schemaErrors, model.Error{
			Paths:   path,
			Message: message,
		})
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

// validateReferencedSchemas validates all objects with @context against their schemas.
func (v *schemav2Validator) validateReferencedSchemas(ctx context.Context, body interface{}) error {
	// Extract "message" object - only scan inside message, not root
	bodyMap, ok := body.(map[string]interface{})
	if !ok {
		return fmt.Errorf("body is not a valid JSON object")
	}

	message, hasMessage := bodyMap["message"]
	if !hasMessage {
		return fmt.Errorf("missing 'message' field in request body")
	}

	// Find all objects with @context starting from message
	objects := findReferencedObjects(message, "message")

	if len(objects) == 0 {
		log.Debugf(ctx, "No objects with @context found in message, skipping LEVEL 2 validation")
		return nil
	}

	log.Debugf(ctx, "Found %d objects with @context for LEVEL 2 validation", len(objects))

	// Get config with defaults
	urlTransform := "context.jsonld->attributes.yaml"
	ttl := 86400 * time.Second // 24 hours default
	timeout := 30 * time.Second
	var allowedDomains []string

	refConfig := v.config.ReferencedSchemaConfig
	if refConfig.URLTransform != "" {
		urlTransform = refConfig.URLTransform
	}
	if refConfig.CacheTTL > 0 {
		ttl = time.Duration(refConfig.CacheTTL) * time.Second
	}
	if refConfig.DownloadTimeout > 0 {
		timeout = time.Duration(refConfig.DownloadTimeout) * time.Second
	}
	allowedDomains = refConfig.AllowedDomains

	log.Debugf(ctx, "LEVEL 2 config: urlTransform=%s, ttl=%v, timeout=%v, allowedDomains=%v",
		urlTransform, ttl, timeout, allowedDomains)

	// Validate each object and collect errors
	var errors []string
	for _, obj := range objects {
		log.Debugf(ctx, "Validating object at path: %s, @context: %s, @type: %s",
			obj.Path, obj.Context, obj.Type)

		if err := v.schemaCache.validateReferencedObject(ctx, obj, urlTransform, ttl, timeout, allowedDomains); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
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

// newSchemaCache creates a new schema cache.
func newSchemaCache(maxSize int) *schemaCache {
	return &schemaCache{
		schemas: make(map[string]*cachedDomainSchema),
		maxSize: maxSize,
	}
}

// hashURL creates a SHA256 hash of the URL for use as cache key.
func hashURL(urlStr string) string {
	hash := sha256.Sum256([]byte(urlStr))
	return hex.EncodeToString(hash[:])
}

// isValidSchemaPath validates if the schema path is safe to load.
func isValidSchemaPath(schemaPath string) bool {
	u, err := url.Parse(schemaPath)
	if err != nil {
		// Could be a simple file path
		return schemaPath != ""
	}
	// Support: http://, https://, file://, or no scheme (local path)
	return u.Scheme == "http" || u.Scheme == "https" ||
		u.Scheme == "file" || u.Scheme == ""
}

// get retrieves a cached schema and updates access tracking.
func (c *schemaCache) get(urlHash string) (*openapi3.T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached, exists := c.schemas[urlHash]
	if !exists || time.Now().After(cached.expiresAt) {
		return nil, false
	}

	// Update access tracking
	cached.lastAccessed = time.Now()
	cached.accessCount++

	return cached.doc, true
}

// set stores a schema in the cache with TTL and LRU eviction.
func (c *schemaCache) set(urlHash string, doc *openapi3.T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// LRU eviction if cache is full
	if len(c.schemas) >= c.maxSize {
		var oldest string
		var oldestTime time.Time
		for k, v := range c.schemas {
			if oldest == "" || v.lastAccessed.Before(oldestTime) {
				oldest, oldestTime = k, v.lastAccessed
			}
		}
		if oldest != "" {
			delete(c.schemas, oldest)
		}
	}

	c.schemas[urlHash] = &cachedDomainSchema{
		doc:          doc,
		loadedAt:     time.Now(),
		expiresAt:    time.Now().Add(ttl),
		lastAccessed: time.Now(),
		accessCount:  1,
	}
}

// cleanupExpired removes expired schemas from cache.
func (c *schemaCache) cleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expired := make([]string, 0)

	for urlHash, cached := range c.schemas {
		if now.After(cached.expiresAt) {
			expired = append(expired, urlHash)
		}
	}

	for _, urlHash := range expired {
		delete(c.schemas, urlHash)
	}

	return len(expired)
}

// loadSchemaFromPath loads a schema from URL or local file with timeout and caching.
func (c *schemaCache) loadSchemaFromPath(ctx context.Context, schemaPath string, ttl, timeout time.Duration) (*openapi3.T, error) {
	urlHash := hashURL(schemaPath)

	// Check cache first
	if doc, found := c.get(urlHash); found {
		log.Debugf(ctx, "Schema cache hit for: %s", schemaPath)
		return doc, nil
	}

	log.Debugf(ctx, "Schema cache miss, loading from: %s", schemaPath)

	// Validate path format
	if !isValidSchemaPath(schemaPath) {
		return nil, fmt.Errorf("invalid schema path: %s", schemaPath)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error

	u, parseErr := url.Parse(schemaPath)
	if parseErr == nil && (u.Scheme == "http" || u.Scheme == "https") {
		// Load from URL with timeout
		loadCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		loader.Context = loadCtx
		doc, err = loader.LoadFromURI(u)
	} else {
		// Load from local file (file:// or path)
		filePath := schemaPath
		if u != nil && u.Scheme == "file" {
			filePath = u.Path
		}
		doc, err = loader.LoadFromFile(filePath)
	}

	if err != nil {
		log.Errorf(ctx, err, "Failed to load schema from: %s", schemaPath)
		return nil, fmt.Errorf("failed to load schema from %s: %w", schemaPath, err)
	}

	// Validate loaded schema (non-blocking, just log warnings)
	if err := doc.Validate(ctx); err != nil {
		log.Debugf(ctx, "Schema validation warnings for %s: %v", schemaPath, err)
	}

	c.set(urlHash, doc, ttl)
	log.Debugf(ctx, "Loaded and cached schema from: %s", schemaPath)

	return doc, nil
}

// findReferencedObjects recursively finds ALL objects with @context in the data.
func findReferencedObjects(data interface{}, path string) []referencedObject {
	var results []referencedObject

	switch v := data.(type) {
	case map[string]interface{}:
		// Check for @context and @type
		if contextVal, hasContext := v["@context"].(string); hasContext {
			if typeVal, hasType := v["@type"].(string); hasType {
				results = append(results, referencedObject{
					Path:    path,
					Context: contextVal,
					Type:    typeVal,
					Data:    v,
				})
			}
		}

		// Recurse into nested objects
		for key, val := range v {
			newPath := key
			if path != "" {
				newPath = path + "." + key
			}
			results = append(results, findReferencedObjects(val, newPath)...)
		}

	case []interface{}:
		// Recurse into arrays
		for i, item := range v {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			results = append(results, findReferencedObjects(item, newPath)...)
		}
	}

	return results
}

// transformContextToSchemaURL transforms @context URL to schema URL.
func transformContextToSchemaURL(contextURL, transform string) string {
	parts := strings.Split(transform, "->")
	if len(parts) != 2 {
		// Default transformation
		return strings.Replace(contextURL, "context.jsonld", "attributes.yaml", 1)
	}
	return strings.Replace(contextURL, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), 1)
}

// findSchemaByType finds a schema in the document by @type value.
func findSchemaByType(doc *openapi3.T, typeName string) (*openapi3.SchemaRef, error) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, fmt.Errorf("no schemas found in document")
	}

	// Try direct match by schema name
	if schema, exists := doc.Components.Schemas[typeName]; exists {
		return schema, nil
	}

	// Fallback: Try x-jsonld.@type match
	for _, schema := range doc.Components.Schemas {
		if schema.Value == nil {
			continue
		}
		if xJsonld, ok := schema.Value.Extensions["x-jsonld"].(map[string]interface{}); ok {
			if atType, ok := xJsonld["@type"].(string); ok && atType == typeName {
				return schema, nil
			}
		}
	}

	return nil, fmt.Errorf("no schema found for @type: %s", typeName)
}

// isAllowedDomain checks if the URL domain is in the whitelist.
func isAllowedDomain(schemaURL string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true // No whitelist = all allowed
	}
	for _, domain := range allowedDomains {
		if strings.Contains(schemaURL, domain) {
			return true
		}
	}
	return false
}

// validateReferencedObject validates a single object with @context.
func (c *schemaCache) validateReferencedObject(
	ctx context.Context,
	obj referencedObject,
	urlTransform string,
	ttl, timeout time.Duration,
	allowedDomains []string,
) error {
	// Domain whitelist check
	if !isAllowedDomain(obj.Context, allowedDomains) {
		log.Warnf(ctx, "Domain not in whitelist: %s", obj.Context)
		return fmt.Errorf("domain not allowed: %s", obj.Context)
	}

	// Transform @context to schema path (URL or file)
	schemaPath := transformContextToSchemaURL(obj.Context, urlTransform)
	log.Debugf(ctx, "Transformed %s -> %s", obj.Context, schemaPath)

	// Load schema with timeout (supports URL or local file)
	doc, err := c.loadSchemaFromPath(ctx, schemaPath, ttl, timeout)
	if err != nil {
		return fmt.Errorf("at %s: %w", obj.Path, err)
	}

	// Find schema by @type
	schema, err := findSchemaByType(doc, obj.Type)
	if err != nil {
		log.Errorf(ctx, err, "Schema not found for @type: %s at path: %s", obj.Type, obj.Path)
		return fmt.Errorf("at %s: %w", obj.Path, err)
	}

	// Validate object against schema (same options as Level 1)
	opts := []openapi3.SchemaValidationOption{
		openapi3.VisitAsRequest(),
		openapi3.EnableFormatValidation(),
	}
	if err := schema.Value.VisitJSON(obj.Data, opts...); err != nil {
		log.Debugf(ctx, "Validation failed for @type: %s at path: %s: %v", obj.Type, obj.Path, err)
		return fmt.Errorf("at %s: %w", obj.Path, err)
	}

	log.Debugf(ctx, "Validation passed for @type: %s at path: %s", obj.Type, obj.Path)
	return nil
}
