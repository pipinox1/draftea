# Telemetry System Overview

The Draftea payment system implements a **unified telemetry system** using OpenTelemetry that provides comprehensive observability across all services.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│  Payment Service │    │  Wallet Service │
│    Port: 8080    │    │   Port: 8081    │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          └──────┬─────────────────┘
                 │
        ┌────────▼─────────┐
        │ Shared Telemetry │
        │  /shared/        │
        └────────┬─────────┘
                 │
    ┌────────────▼─────────────┐
    │     Export Targets       │
    ├──────────┬───────────────┤
    │Prometheus│     OTLP      │
    │:9090/    │ (Jaeger/      │
    │metrics   │  Zipkin)      │
    └──────────┴───────────────┘
```

## Key Features

### 🎯 **Unified System**
- Single telemetry package shared across all services
- Consistent instrumentation patterns
- Predefined service configurations

### 📊 **Complete Observability**
- **Traces**: Distributed request tracing with correlation IDs
- **Metrics**: Business and infrastructure metrics
- **Context**: Automatic propagation across service boundaries

### 🔄 **Dual Export**
- **Prometheus**: `/metrics` endpoints for monitoring
- **OTLP**: Traces and metrics to OpenTelemetry collectors

### ⚙️ **Easy Integration**
- HTTP middleware for automatic instrumentation
- Context-based telemetry injection
- Minimal configuration required

## Service Integration

### Payment Service (Port 8080)
```go
// Configuration
config := telemetry.PaymentServiceConfig.WithOTLPEndpoint(endpoint)

// Automatic HTTP instrumentation + metrics at /metrics
```

### Wallet Service (Port 8081)
```go
// Configuration
config := telemetry.WalletServiceConfig.WithOTLPEndpoint(endpoint)

// Automatic HTTP instrumentation + metrics at /metrics
```

## Metrics Available

### HTTP Metrics (Automatic)
- `http_requests_total` - Request counter by method, path, status
- `http_request_duration_seconds` - Request latency histogram

### Business Metrics (Custom)
- `wallet_operations_total` - Wallet operations counter
- `wallet_operation_duration_seconds` - Operation latency
- `wallet_balance` - Current wallet balances (gauge)
- `events_published_total` - Domain events published
- `payment_operations_total` - Payment operations counter

## Configuration

### Environment Variables
```bash
# Enable/disable telemetry
TELEMETRY_ENABLED=true

# OTLP collector endpoint
OTLP_ENDPOINT=http://localhost:4318
```

### Service-Specific Overrides
```bash
# Payment service specific
PAYMENT_TELEMETRY_ENABLED=false

# Wallet service specific
WALLET_TELEMETRY_ENABLED=false
```

## Accessing Metrics

### Prometheus Endpoints
- Payment Service: http://localhost:8080/metrics
- Wallet Service: http://localhost:8081/metrics

### Sample Prometheus Queries
```promql
# Request rate by service
sum(rate(http_requests_total[5m])) by (service)

# Average wallet operation latency
avg(wallet_operation_duration_seconds) by (operation)

# Total wallet balance across all wallets
sum(wallet_balance) by (currency)

# Error rate by service
sum(rate(http_requests_total{status_class="5xx"}[5m])) by (service)
```

## Trace Correlation

All requests are automatically traced with:
- **Service identification**: Automatic service name tagging
- **Request correlation**: Unique trace IDs across service calls
- **Operation spans**: Database queries, external calls, business logic
- **Error tracking**: Automatic error capture and status codes

## Development

### Local Setup
1. **OTLP Collector** (optional): http://localhost:4318
2. **Prometheus** (optional): http://localhost:9090
3. **Jaeger** (optional): http://localhost:16686

### Adding Custom Metrics
```go
// In any handler or use case
telemetry.RecordCounter(ctx, "custom_operations", "Custom operations", 1,
    attribute.String("operation", "custom"),
    attribute.String("status", "success"),
)
```

### Adding Custom Traces
```go
// Create custom span
ctx, span := telemetry.StartSpan(ctx, "custom_operation")
defer span.End()

// Add attributes
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.Int("items.count", itemCount),
)
```

## Production Considerations

### Performance
- Minimal overhead with sampling and batching
- Async export to avoid blocking requests
- Graceful degradation if exporters fail

### Resource Usage
- Configurable metric intervals (default: 30s)
- Automatic span batching for efficiency
- Memory-efficient metric storage

### Reliability
- Services continue operating if telemetry fails
- Non-blocking telemetry initialization
- Automatic retry for failed exports

## Monitoring Stack Integration

The telemetry system is designed to work with:
- **Prometheus** + Grafana for metrics
- **Jaeger** or **Zipkin** for traces
- **OpenTelemetry Collector** for data pipeline
- **Alertmanager** for notifications

See [Telemetry Implementation Guide](../shared/telemetry/README.md) for detailed usage examples.