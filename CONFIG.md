# Beckn-ONIX Configuration Guide

## Table of Contents
1. [Overview](#overview)
2. [Configuration File Structure](#configuration-file-structure)
3. [Top-Level Configuration](#top-level-configuration)
4. [HTTP Configuration](#http-configuration)
5. [Logging Configuration](#logging-configuration)
6. [Metrics Configuration](#metrics-configuration)
7. [Plugin Manager Configuration](#plugin-manager-configuration)
8. [Module Configuration](#module-configuration)
9. [Handler Configuration](#handler-configuration)
10. [Plugin Configuration](#plugin-configuration)
11. [Routing Configuration](#routing-configuration)
12. [Deployment Scenarios](#deployment-scenarios)
13. [Configuration Examples](#configuration-examples)

---

## Overview

Beckn-ONIX uses YAML configuration files to define its behavior. The plugin based architecture enables beckn enabled network participants to deploy onix adapter in variety of scenarios, all of which can be controlled and configured using the adapter config files and hence, understanding how the configuration is strctured is very critical.

let's start with understanding following key concepts first.

### Module Handler
A Module Handler or simply handler encapsulates execution logic (steps and plugins) for partcular scenario. Scenario could be typically classified in 2 buckets, `txn caller`, outgoing call into Beckn network and `txn receiver`, incoming call from Beckn network. The handler enables flexibility to execute different steps (peformed by different plugins) for both scenarios. A single deployed onix adapter service can run multiple handlers and hence a single adapter can support multiple BAPs and/or BPPs. Each handler defines its own set of steps and plugin configurations.

### Plugins
This is a sub section to the handler, meaning each handler can load a separate set of plugins. Plugins section enlists all plugins to be loaded at the start, and further includes configuration parameters for each of the listed plugin.

### Steps
This is a list of steps that executed by the handler in the same given order.

The configuration file is an input to the onix adapter and it is recommended to use different files for different deployment scenarios (local development, production, BAP-only, BPP-only, or combined).

### Configuration File Locations

```
config/
├── local-dev.yaml                    # Local development (with Vault)
├── local-simple.yaml                 # Local development (embedded keys)
├── local-routing.yaml                # Local routing rules
├── onix/                             # Production combined mode
│   ├── adapter.yaml
│   ├── bapTxnCaller-routing.yaml
│   ├── bapTxnReciever-routing.yaml
│   ├── bppTxnCaller-routing.yaml
│   ├── bppTxnReciever-routing.yaml
│   └── plugin.yaml
├── onix-bap/                         # Production BAP-only mode
│   ├── adapter.yaml
│   ├── bapTxnCaller-routing.yaml
│   ├── bapTxnReciever-routing.yaml
│   └── plugin.yaml
└── onix-bpp/                         # Production BPP-only mode
    ├── adapter.yaml
    ├── bppTxnCaller-routing.yaml
    ├── bppTxnReciever-routing.yaml
    └── plugin.yaml
```

---

## Configuration File Structure

### Main Configuration File (adapter.yaml)

The main configuration file follows this structure:

```yaml
appName: "onix-local"
log: {...}
metrics: {...}
http: {...}
pluginManager: {...}
modules: [...]
```

---

## Top-Level Configuration

### `appName`
**Type**: `string`  
**Required**: Yes  
**Description**: Application identifier used in logging and monitoring.

**Example**:
```yaml
appName: "onix-local"
```

---

## HTTP Configuration

### `http`
**Type**: `object`  
**Required**: Yes  
**Description**: HTTP server configuration including port and timeout settings.

#### Parameters:

##### `port`
**Type**: `string`  
**Required**: Yes  
**Description**: Port number on which the HTTP server listens.  
**Example**: `"8081"`, `"8080"`

##### `timeout`
**Type**: `object`  
**Required**: Yes  
**Description**: HTTP timeout configurations.

###### `timeout.read`
**Type**: `integer` (seconds)  
**Required**: Yes  
**Default**: `30`  
**Description**: Maximum duration for reading the entire request, including the body.

###### `timeout.write`
**Type**: `integer` (seconds)  
**Required**: Yes  
**Default**: `30`  
**Description**: Maximum duration before timing out writes of the response.

###### `timeout.idle`
**Type**: `integer` (seconds)  
**Required**: Yes  
**Default**: `30`  
**Description**: Maximum amount of time to wait for the next request when keep-alives are enabled.

**Example**:
```yaml
http:
  port: 8081
  timeout:
    read: 30
    write: 30
    idle: 30
```

---

## Logging Configuration

### `log`
**Type**: `object`  
**Required**: Yes  
**Description**: Logging configuration for the application.

#### Parameters:

##### `level`
**Type**: `string`  
**Required**: Yes  
**Options**: `debug`, `info`, `warn`, `error`, `fatal`  
**Description**: Sets the minimum log level. Messages below this level will not be logged.

##### `destinations`
**Type**: `array`  
**Required**: Yes  
**Description**: List of log output destinations.

###### `destinations[].type`
**Type**: `string`  
**Options**: `stdout`, `file`  
**Description**: Type of log destination.

##### `contextKeys`
**Type**: `array` of `string`  
**Required**: No  
**Description**: Context keys to include in structured logs for request tracing.  
**Common Values**: `transaction_id`, `message_id`, `subscriber_id`, `module_id`

**Example**:
```yaml
log:
  level: debug
  destinations:
    - type: stdout
  contextKeys:
    - transaction_id
    - message_id
    - subscriber_id
    - module_id
```

---

## Metrics Configuration

### `metrics`
**Type**: `object`  
**Required**: No  
**Description**: OpenTelemetry metrics configuration for observability and monitoring.

**Important**: When `enabled: true`, metrics are automatically exposed at the `/metrics` endpoint in Prometheus format. This allows Prometheus or any HTTP client to scrape metrics directly from the application.

#### Parameters:

##### `enabled`
**Type**: `boolean`  
**Required**: No  
**Default**: `false`  
**Description**: Enable or disable metrics collection. When enabled:
- Metrics are collected automatically
- Metrics are exposed at `/metrics` endpoint in Prometheus format
- All metrics subsystems are initialized (request metrics, runtime metrics)

When disabled, no metrics are collected and the `/metrics` endpoint is not available.

##### `exporterType`
**Type**: `string`  
**Required**: Yes (if `enabled` is `true`)  
**Options**: `prometheus`  
**Default**: `prometheus`  
**Description**: Metrics exporter type. Currently only `prometheus` is supported, which exposes metrics at the `/metrics` endpoint.

**Note**: The `/metrics` endpoint is always available when `enabled: true`.

##### `serviceName`
**Type**: `string`  
**Required**: No  
**Default**: `"beckn-onix"`  
**Description**: Service name used in metrics resource attributes. Helps identify the service in observability platforms.

##### `serviceVersion`
**Type**: `string`  
**Required**: No  
**Description**: Service version used in metrics resource attributes. Useful for tracking different versions of the service.

##### `prometheus`
**Type**: `object`  
**Required**: No  
**Description**: Prometheus exporter configuration (reserved for future use).

**Example - Enable Metrics**:
```yaml
metrics:
  enabled: true
  exporterType: prometheus
  serviceName: beckn-onix
  serviceVersion: "1.0.0"
```
**Note**: Metrics are available at `/metrics` endpoint in Prometheus format.

**Example - Disabled Metrics**:
```yaml
metrics:
  enabled: false
```
**Note**: No metrics are collected and `/metrics` endpoint is not available.

### Accessing Metrics

When `metrics.enabled: true`, metrics are automatically available at:

```
http://your-server:port/metrics
```

The endpoint returns metrics in Prometheus format and can be:
- Scraped by Prometheus
- Accessed via `curl http://localhost:8081/metrics`
- Viewed in a web browser

### Metrics Collected

The adapter automatically collects the following metrics:

#### HTTP Metrics (Automatic via OpenTelemetry HTTP Middleware)
- `http.server.duration` - Request duration histogram
- `http.server.request.size` - Request body size
- `http.server.response.size` - Response body size
- `http.server.active_requests` - Active request counter

#### Request Metrics (Automatic)
**Inbound Requests:**
- `beckn.inbound.requests.total` - Total inbound requests per host
- `beckn.inbound.sign_validation.total` - Requests with sign validation per host
- `beckn.inbound.schema_validation.total` - Requests with schema validation per host

**Outbound Requests:**
- `beckn.outbound.requests.total` - Total outbound requests per host
- `beckn.outbound.requests.2xx` - 2XX responses per host
- `beckn.outbound.requests.4xx` - 4XX responses per host
- `beckn.outbound.requests.5xx` - 5XX responses per host
- `beckn.outbound.request.duration` - Request duration histogram (supports p99, p95, p75 percentiles) per host

#### Go Runtime Metrics (Automatic)
- `go_cpu_*` - CPU usage metrics
- `go_memstats_*` - Memory allocation and heap statistics
- `go_memstats_gc_*` - Garbage collection statistics
- `go_goroutines` - Goroutine count

#### Redis Metrics (Automatic via redisotel)
- `redis_commands_duration_seconds` - Redis command duration
- `redis_commands_total` - Total Redis commands
- `redis_connections_active` - Active Redis connections
- Additional Redis-specific metrics

All metrics include relevant attributes (labels) such as:
- `host` - Request hostname
- `status_code` - HTTP status code
- `operation` - HTTP operation name
- `service.name` - Service identifier
- `service.version` - Service version

---

## Plugin Manager Configuration

### `pluginManager`
**Type**: `object`  
**Required**: Yes  
**Description**: Configuration for the plugin management system.

#### Parameters:

##### `root`
**Type**: `string`  
**Required**: Yes  
**Description**: Local directory path where plugin binaries (`.so` files) are stored.  
**Example**: `./plugins`, `/app/plugins`

##### `remoteRoot`
**Type**: `string`  
**Required**: No  
**Description**: Path to remote plugin bundle (typically in GCS or S3) for production deployments.  
**Example**: `/mnt/gcs/plugins/plugins_bundle.zip`

**Example**:
```yaml
pluginManager:
  root: ./plugins
  remoteRoot: /mnt/gcs/plugins/plugins_bundle.zip
```

---

## Module Configuration

### `modules`
**Type**: `array`  
**Required**: Yes  
**Description**: List of transaction processing modules. Each module represents an HTTP endpoint handler with its own configuration.

#### Module Types:

There are four main module types in Beckn-ONIX:

1. **bapTxnReceiver**: Receives callback responses at BAP from BPP
2. **bapTxnCaller**: Sends outgoing requests from BAP to BPP/Gateway
3. **bppTxnReceiver**: Receives incoming requests at BPP from BAP
4. **bppTxnCaller**: Sends callback responses from BPP to BAP

#### Parameters:

##### `name`
**Type**: `string`  
**Required**: Yes  
**Description**: Unique identifier for the module.  
**Example**: `bapTxnReceiver`, `bapTxnCaller`, `bppTxnReceiver`, `bppTxnCaller`

##### `path`
**Type**: `string`  
**Required**: Yes  
**Description**: HTTP path prefix for this module's endpoints.  
**Example**: `/bap/receiver/`, `/bap/caller/`, `/bpp/receiver/`, `/bpp/caller/`

##### `handler`
**Type**: `object`  
**Required**: Yes  
**Description**: Handler configuration for processing requests. See [Handler Configuration](#handler-configuration).

**Example**:
```yaml
modules:
  - name: bapTxnReceiver
    path: /bap/receiver/
    handler:
      type: std
      role: bap
      # ... handler configuration
```

---

## Handler Configuration

### `handler`
**Type**: `object`  
**Required**: Yes  
**Description**: Defines how requests are processed by a module.

#### Parameters:

##### `type`
**Type**: `string`  
**Required**: Yes  
**Options**: `std` (standard handler)  
**Description**: Type of handler. Currently only `std` is supported.

##### `role`
**Type**: `string`  
**Required**: Yes  
**Options**: `bap`, `bpp`  
**Description**: Role of this handler in the Beckn protocol.

##### `subscriberId`
**Type**: `string`  
**Required**: No  
**Description**: Subscriber ID for the participant. Used primarily for BPP modules.  
**Example**: `bpp1`

##### `httpClientConfig`
**Type**: `object`  
**Required**: Yes  
**Description**: HTTP client configuration for outgoing requests.

###### `maxIdleConns`
**Type**: `integer`  
**Default**: `1000`  
**Description**: Maximum number of idle connections across all hosts.

###### `maxIdleConnsPerHost`
**Type**: `integer`  
**Default**: `200`  
**Description**: Maximum idle connections to keep per host.

###### `idleConnTimeout`
**Type**: `duration`  
**Default**: `300s`  
**Description**: Maximum time an idle connection remains open.

###### `responseHeaderTimeout`
**Type**: `duration`  
**Default**: `5s`  
**Description**: Time to wait for server response headers.

##### `plugins`
**Type**: `object`  
**Required**: Yes  
**Description**: Plugin configurations. See [Plugin Configuration](#plugin-configuration).

##### `steps`
**Type**: `array` of `string`  
**Required**: Yes  
**Description**: Ordered list of processing steps to execute for each request.  
**Common Steps**:
- `validateSign` - Validate digital signature
- `addRoute` - Determine routing destination
- `validateSchema` - Validate against JSON schema
- `sign` - Sign outgoing request
- `publish` - Publish to message queue

**Example**:
```yaml
handler:
  type: std
  role: bap
  httpClientConfig:
    maxIdleConns: 1000
    maxIdleConnsPerHost: 200
    idleConnTimeout: 300s
    responseHeaderTimeout: 5s
  plugins:
    # ... plugin configurations
  steps:
    - validateSign
    - addRoute
    - validateSchema
```

---

## Plugin Configuration

### Plugin Structure

Each plugin configuration follows this structure:

```yaml
pluginName:
  id: plugin-identifier
  config:
    key1: value1
    key2: value2
```

### Available Plugins

#### 1. Registry Plugin

**Purpose**: Lookup participant information from Beckn registry.

**Configuration**:
```yaml
registry:
  id: registry
  config:
    url: http://localhost:8080/reg
    retry_max: 3
    retry_wait_min: 100ms
    retry_wait_max: 500ms
```

**Parameters**:
- `url`: Registry endpoint URL
- `retry_max`: Maximum number of retry attempts
- `retry_wait_min`: Minimum wait time between retries
- `retry_wait_max`: Maximum wait time between retries

---

#### 2. Key Manager Plugin

**Purpose**: Manage cryptographic keys for signing and verification.

##### Vault-based Key Manager (Production)

```yaml
keyManager:
  id: keymanager
  config:
    projectID: beckn-onix-local
    vaultAddr: http://localhost:8200
    kvVersion: v2
    mountPath: beckn
```

**Parameters**:
- `projectID`: GCP project ID or identifier
- `vaultAddr`: HashiCorp Vault address
- `kvVersion`: Vault KV secrets engine version (`v1` or `v2`)
- `mountPath`: Vault mount path for secrets

##### Secrets Manager Key Manager (Production)

```yaml
keyManager:
  id: secretskeymanager
  config:
    projectID: ${projectID}
```

**Parameters**:
- `projectID`: GCP project ID (supports environment variable substitution)

##### Simple Key Manager (Development)

```yaml
keyManager:
  id: simplekeymanager
  config: {}
```

**Parameters**: None required. Uses embedded Ed25519 keys stored in the binary.

---

#### 3. Cache Plugin

**Purpose**: Redis-based caching for responses.

```yaml
cache:
  id: cache
  config:
    addr: localhost:6379
```

**Or for production with Redis cluster:**

```yaml
cache:
  id: redis
  config:
    addr: 10.81.192.4:6379
```

**Parameters**:
- `addr`: Redis server address and port

---

#### 4. Schema Validator Plugin

**Purpose**: Validate requests/responses against JSON schemas.

```yaml
schemaValidator:
  id: schemavalidator
  config:
    schemaDir: ./schemas
```

**Parameters**:
- `schemaDir`: Directory containing JSON schema files organized by domain and version

---

#### 5. Sign Validator Plugin

**Purpose**: Validate Ed25519 digital signatures on incoming requests.

```yaml
signValidator:
  id: signvalidator
```

**Parameters**: None required. Uses key manager for public key lookup.

---

#### 6. Router Plugin

**Purpose**: Determine routing destination based on rules.

```yaml
router:
  id: router
  config:
    routingConfig: ./config/local-routing.yaml
```

**Or for production:**

```yaml
router:
  id: router
  config:
    routingConfigPath: /mnt/gcs/configs/bapTxnCaller-routing.yaml
```

**Parameters**:
- `routingConfig` or `routingConfigPath`: Path to routing rules YAML file

---

#### 7. Signer Plugin

**Purpose**: Sign outgoing requests with Ed25519 signature.

```yaml
signer:
  id: signer
```

**Parameters**: None required. Uses key manager for private key.

---

#### 8. Publisher Plugin

**Purpose**: Publish messages to RabbitMQ or Pub/Sub for asynchronous processing.

```yaml
publisher:
  id: publisher
  config:
    project: ${projectID}
    topic: bapNetworkReciever
```

**Parameters**:
- `project`: GCP project ID for Pub/Sub
- `topic`: Pub/Sub topic name

---

#### 9. Middleware Plugin

**Purpose**: Request preprocessing like UUID generation and header manipulation.

```yaml
middleware:
  - id: reqpreprocessor
    config:
      uuidKeys: transaction_id,message_id
      role: bap
```

**Parameters**:
- `uuidKeys`: Comma-separated list of fields to auto-generate UUIDs for if missing
- `role`: BAP or BPP role for request processing

---

## Routing Configuration

### Routing Rules File Structure

Routing configuration is stored in separate YAML files referenced by the router plugin.

```yaml
routingRules:
  - domain: "retail:1.1.0"
    version: "1.1.0"
    targetType: "url"
    target:
      url: "http://localhost:9001/beckn"
      excludeAction: false
    endpoints:
      - search
      - select
      - init
      - confirm
```

### Routing Rule Parameters

#### `domain`
**Type**: `string`  
**Required**: Yes  
**Description**: Beckn domain identifier (e.g., `retail:1.1.0`, `ONDC:TRV10`, `nic2004:60221`)

#### `version`
**Type**: `string`  
**Required**: Yes  
**Description**: Protocol version for this domain

#### `targetType`
**Type**: `string`  
**Required**: Yes  
**Options**: `url`, `bpp`, `bap`, `msgq`  
**Description**: Type of routing destination

##### Target Types Explained:

1. **`url`**: Route to a specific URL
   ```yaml
   targetType: "url"
   target:
     url: "http://backend-service:3000/api"
     excludeAction: false  # If true, don't append endpoint to URL
   ```

2. **`bpp`**: Route to BPP specified in request's `bpp_uri`
   ```yaml
   targetType: "bpp"
   target:
     url: "https://gateway.example.com"  # Optional fallback URL
   endpoints:
     - search
   ```

3. **`bap`**: Route to BAP specified in request's `bap_uri`
   ```yaml
   targetType: "bap"
   endpoints:
     - on_search
     - on_select
   ```

4. **`msgq`**: Route to message queue (Pub/Sub)
   ```yaml
   targetType: "msgq"
   target:
     topic_id: "search_requests"
   ```

#### `target`
**Type**: `object`  
**Required**: Depends on `targetType`  
**Description**: Target destination details

##### `target.url`
**Type**: `string`  
**Description**: Target URL for `url` type, or fallback URL for `bpp`/`bap` types

##### `target.excludeAction`
**Type**: `boolean`  
**Default**: `false`  
**Description**: For `url` type, whether to exclude appending endpoint name to URL path

##### `target.topic_id`
**Type**: `string`  
**Description**: Pub/Sub topic ID for `msgq` type

##### `target.publisherId`
**Type**: `string`  
**Description**: Publisher ID for `publisher` type (deprecated in favor of `msgq`)

#### `endpoints`
**Type**: `array` of `string`  
**Required**: Yes  
**Description**: List of Beckn protocol endpoints this rule applies to

**Common Endpoints**:
- BAP Caller: `search`, `select`, `init`, `confirm`, `status`, `track`, `cancel`, `update`, `rating`, `support`
- BPP Caller: `on_search`, `on_select`, `on_init`, `on_confirm`, `on_status`, `on_track`, `on_cancel`, `on_update`, `on_rating`, `on_support`

### Routing Configuration Examples

#### Example 1: Simple URL Routing

```yaml
routingRules:
  - domain: "retail:1.1.0"
    version: "1.1.0"
    targetType: "url"
    target:
      url: "http://backend-service:3000/retail/v1"
    endpoints:
      - search
      - select
      - init
```

**Behavior**: All `search`, `select`, and `init` requests for `retail:1.1.0` will be routed to:
- `http://backend-service:3000/retail/v1/search`
- `http://backend-service:3000/retail/v1/select`
- `http://backend-service:3000/retail/v1/init`

#### Example 2: BPP Dynamic Routing

```yaml
routingRules:
  - domain: "ONDC:TRV10"
    version: "2.0.0"
    targetType: "bpp"
    target:
      url: "https://gateway.example.com"
    endpoints:
      - search
  
  - domain: "ONDC:TRV10"
    version: "2.0.0"
    targetType: "bpp"
    endpoints:
      - select
      - init
      - confirm
```

**Behavior**: 
- For `search`: Route to gateway URL if `bpp_uri` is missing from request
- For other endpoints: Route to `bpp_uri` from request context (required)

#### Example 3: Mixed Routing

```yaml
routingRules:
  - domain: "ONDC:TRV10"
    version: "2.0.0"
    targetType: "url"
    target:
      url: "https://services-backend/trv/v1"
    endpoints:
      - select
      - init
      - confirm

  - domain: "ONDC:TRV10"
    version: "2.0.0"
    targetType: "msgq"
    target:
      topic_id: "trv_search_requests"
    endpoints:
      - search
```

**Behavior**:
- `select`, `init`, `confirm`: Routed to backend URL
- `search`: Published to Pub/Sub topic for asynchronous processing

#### Example 4: URL with excludeAction

```yaml
routingRules:
  - domain: "retail:1.1.0"
    version: "1.1.0"
    targetType: "url"
    target:
      url: "http://backend:3000/webhook"
      excludeAction: true
    endpoints:
      - search
      - select
```

**Behavior**: All endpoints route to exactly `http://backend:3000/webhook` without appending the endpoint name.

---

## Deployment Scenarios

### 1. Local Development (Simple Mode)

**File**: `config/local-simple.yaml`

**Characteristics**:
- Uses `simplekeymanager` (no Vault required)
- Embedded Ed25519 keys
- Local Redis
- Simplified routing
- Optional metrics collection (available at `/metrics` when enabled)

**Use Case**: Quick local development and testing

```yaml
appName: "onix-local"
log:
  level: debug
metrics:
  enabled: true
  exporterType: prometheus
  serviceName: onix-local
http:
  port: 8081
modules:
  - name: bapTxnReceiver
    handler:
      plugins:
        keyManager:
          id: simplekeymanager
          config: {}
```

**Metrics Access**: When enabled, access metrics at `http://localhost:8081/metrics`

### 2. Local Development (Vault Mode)

**File**: `config/local-dev.yaml`

**Characteristics**:
- Uses `keymanager` with local Vault
- Full production-like setup
- Local Redis and Vault

**Use Case**: Testing with production key management

```yaml
modules:
  - name: bapTxnReceiver
    handler:
      plugins:
        keyManager:
          id: keymanager
          config:
            projectID: beckn-onix-local
            vaultAddr: http://localhost:8200
```

### 3. Production Combined Mode

**File**: `config/onix/adapter.yaml`

**Characteristics**:
- Handles both BAP and BPP
- GCP Secrets Manager for keys
- Production Redis
- Remote plugin loading
- Pub/Sub integration
- OpenTelemetry metrics enabled (available at `/metrics` endpoint)

**Use Case**: Single deployment serving both roles

```yaml
appName: "onix-production"
log:
  level: info
  destinations:
    - type: stdout
metrics:
  enabled: true
  exporterType: prometheus
  serviceName: beckn-onix
  serviceVersion: "1.0.0"
pluginManager:
  root: /app/plugins
  remoteRoot: /mnt/gcs/plugins/plugins_bundle.zip
modules:
  - name: bapTxnReciever
    handler:
      plugins:
        keyManager:
          id: secretskeymanager
          config:
            projectID: ${projectID}
        cache:
          id: redis
          config:
            addr: 10.81.192.4:6379
        publisher:
          id: publisher
          config:
            project: ${projectID}
            topic: bapNetworkReciever
```

**Metrics Access**: 
- Prometheus scraping: `http://your-server:port/metrics`

### 4. Production BAP-Only Mode

**File**: `config/onix-bap/adapter.yaml`

**Characteristics**:
- Only BAP modules (bapTxnReceiver, bapTxnCaller)
- Dedicated BAP deployment
- Production infrastructure

**Use Case**: Separate BAP service for scalability

### 5. Production BPP-Only Mode

**File**: `config/onix-bpp/adapter.yaml`

**Characteristics**:
- Only BPP modules (bppTxnReceiver, bppTxnCaller)
- Dedicated BPP deployment
- Production infrastructure

**Use Case**: Separate BPP service for scalability

---

## Configuration Examples

### Complete BAP Receiver Configuration

```yaml
appName: "onix-bap"
log:
  level: info
  destinations:
    - type: stdout
  contextKeys:
    - transaction_id
    - message_id
    - subscriber_id

http:
  port: 8080
  timeout:
    read: 30
    write: 30
    idle: 30

pluginManager:
  root: /app/plugins

modules:
  - name: bapTxnReceiver
    path: /bap/receiver/
    handler:
      type: std
      role: bap
      httpClientConfig:
        maxIdleConns: 1000
        maxIdleConnsPerHost: 200
        idleConnTimeout: 300