# Schemav2Validator Plugin

Validates Beckn protocol requests against OpenAPI 3.1 specifications using kin-openapi library.

## Features

- Validates requests against OpenAPI 3.1 specs
- Supports remote URL and local file loading
- Automatic external $ref resolution
- TTL-based caching with automatic refresh
- Generic path matching (no hardcoded paths)
- Direct schema validation without router overhead

## Configuration

```yaml
schemaValidator:
  id: schemav2validator
  config:
    type: url
    location: https://example.com/openapi-spec.yaml
    cacheTTL: "3600"
```

### Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `type` | string | Yes | - | Type of spec source: "url" or "file" ("dir" reserved for future) |
| `location` | string | Yes | - | URL or file path to OpenAPI 3.1 spec |
| `cacheTTL` | string | No | "3600" | Cache TTL in seconds before reloading spec |



## How It Works

1. **Load Spec**: Loads OpenAPI spec from configured URL at startup
2. **Extract Action**: Extracts `action` from request `context.action` field
3. **Find Schema**: Searches all paths and HTTP methods in spec for schema with matching action:
   - Checks `properties.context.action.enum` for the action value
   - Also checks `properties.context.allOf[].properties.action.enum`
   - Stops at first match
4. **Validate**: Validates request body against matched schema using `Schema.VisitJSON()` with:
   - Required fields validation
   - Data type validation (string, number, boolean, object, array)
   - Format validation (email, uri, date-time, uuid, etc.)
   - Constraint validation (min/max, pattern, enum, const)
   - Nested object and array validation
5. **Return Errors**: Returns validation errors in ONIX format

## Action-Based Matching

The validator uses action-based schema matching, not URL path matching. It searches for schemas where the `context.action` field has an enum constraint containing the request's action value.

### Example OpenAPI Schema

```yaml
paths:
  /beckn/search:
    post:
      requestBody:
        content:
          application/json:
            schema:
              properties:
                context:
                  properties:
                    action:
                      enum: ["search"]  # ← Matches action="search"
```

### Matching Examples

| Request Action | Schema Enum | Match |
|----------------|-------------|-------|
| `search` | `enum: ["search"]` | ✅ Matches |
| `select` | `enum: ["select", "init"]` | ✅ Matches |
| `discover` | `enum: ["search"]` | ❌ No match |
| `on_search` | `enum: ["on_search"]` | ✅ Matches |

## External References

The validator automatically resolves external `$ref` references in OpenAPI specs:

```yaml
# Main spec at https://example.com/api.yaml
paths:
  /search:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: 'https://example.com/schemas/search.yaml#/SearchRequest'
```

The loader will automatically fetch and resolve the external reference.

## Example Usage

### Remote URL

```yaml
schemaValidator:
  id: schemav2validator
  config:
    type: url
    location: https://raw.githubusercontent.com/beckn/protocol-specifications/master/api/beckn-2.0.0.yaml
    cacheTTL: "7200"
```

### Local File

```yaml
schemaValidator:
  id: schemav2validator
  config:
    type: file
    location: ./validation-scripts/l2-config/mobility_1.1.0_openapi_3.1.yaml
    cacheTTL: "3600"
```



## Dependencies

- `github.com/getkin/kin-openapi` - OpenAPI 3 parser and validator

## Error Messages

| Scenario | Error Message |
|----------|---------------|
| Action is number | `"failed to parse JSON payload: json: cannot unmarshal number into Go struct field .context.action of type string"` |
| Action is empty | `"missing field Action in context"` |
| Action not in spec | `"unsupported action: <action>"` |
| Invalid URL | `"Invalid URL or unreachable: <url>"` |
| Schema validation fails | Returns detailed field-level errors |

