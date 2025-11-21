# Beckn-ONIX Metrics Runbook

## Quick Links
- Metrics: http://localhost:8081/metrics
- Health: http://localhost:8081/health
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## PromQL Cheat Sheet
- Request rate: `rate(http_server_requests_total[5m])`
- Error rate %: `100 * rate(http_server_requests_total{status="error"}[5m]) / rate(http_server_requests_total[5m])`
- Latency P95: `histogram_quantile(0.95, rate(http_server_request_duration_seconds_bucket[5m]))`
- Step errors: `rate(onix_step_errors_total[5m])`
- Cache hit rate %: `100 * rate(onix_cache_hits_total[5m]) / (rate(onix_cache_hits_total[5m]) + rate(onix_cache_misses_total[5m]))`
- Signature success %: `100 * rate(beckn_signature_validations_total{status="success"}[5m]) / rate(beckn_signature_validations_total[5m])`
- Schema success %: `100 * rate(beckn_schema_validations_total{status="success"}[5m]) / rate(beckn_schema_validations_total[5m])`

## Troubleshooting
- **High error rate**: inspect `http_server_requests_total{status="error"}` by module/action, review logs.
- **High latency**: check `http_server_request_duration_seconds` and `onix_step_execution_duration_seconds`, verify cache hit rate.
- **Step/plugin failures**: monitor `onix_step_errors_total` and `onix_plugin_errors_total`, identify failing step/plugin labels.
- **Low cache efficiency**: examine `onix_cache_hits_total` vs `onix_cache_misses_total`, verify Redis health.
- **Metrics missing**: ensure telemetry enabled in config, `/metrics` accessible, Prometheus scrape succeeds.

## Operations Checklist
1. Build plugins (includes otelmetrics): `./install/build-plugins.sh`
2. Start adapter with telemetry-enabled config.
3. Launch monitoring stack: `cd monitoring && docker-compose -f docker-compose-monitoring.yml up -d`
4. Validate `/metrics`, Prometheus targets, Grafana dashboard, alert rules.
5. Document incident learnings here.

