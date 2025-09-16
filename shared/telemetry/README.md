# Shared Telemetry System

Este paquete proporciona una implementación unificada de telemetría usando OpenTelemetry que puede ser utilizada por cualquier servicio en el sistema.

## Características

- **Trazas (Tracing)**: Seguimiento distribuido de requests con OTLP
- **Métricas**: Counter, Histogram y Gauge con exportación a Prometheus y OTLP
- **Middleware HTTP**: Instrumentación automática de endpoints HTTP
- **Configuraciones predefinidas**: Para servicios conocidos del sistema
- **Flexibilidad**: Soporte para cualquier servicio personalizado

## Uso Básico

### 1. Configuración e Inicialización

```go
import "github.com/draftea/payment-system/shared/telemetry"

// Usar configuración predefinida para servicios conocidos
config := telemetry.WalletServiceConfig.WithOTLPEndpoint("http://localhost:4318")

// O crear configuración personalizada
config := telemetry.NewConfigForService("my-service", "1.0.0", "http://localhost:4318")

// Inicializar telemetría
ctx := context.Background()
tel, shutdown, err := telemetry.InitTelemetry(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer shutdown()
```

### 2. Middleware HTTP (Recomendado)

```go
// En tu router HTTP (Chi, Gin, etc.)
if deps.Telemetry != nil {
    r.Use(telemetry.Middleware(deps.Telemetry))
}

// Ahora todos los handlers tendrán telemetría automática en el contexto
func myHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Ya tiene telemetría inyectada

    // Crear span personalizado
    ctx, span := telemetry.StartSpan(ctx, "my_operation")
    defer span.End()

    // Registrar métricas
    telemetry.RecordCounter(ctx, "requests_processed", "Requests processed", 1,
        attribute.String("handler", "my_handler"),
    )
}
```

### 3. Métricas

```go
// Counter - para contar eventos
telemetry.RecordCounter(ctx, "operations_total", "Total operations", 1,
    attribute.String("operation", "create_user"),
    attribute.String("status", "success"),
)

// Histogram - para distribuciones (latencia, tamaños, etc.)
duration := time.Since(start)
telemetry.RecordHistogram(ctx, "operation_duration_seconds", "Operation duration",
    duration.Seconds(),
    attribute.String("operation", "create_user"),
)

// Gauge - para valores que suben y bajan (memoria, conexiones, etc.)
telemetry.RecordGauge(ctx, "active_connections", "Active connections", float64(connCount),
    attribute.String("service", telemetry.GetServiceName(ctx)),
)
```

### 4. Trazas (Spans)

```go
// Crear span
ctx, span := telemetry.StartSpan(ctx, "database_query",
    trace.WithAttributes(
        attribute.String("db.statement", "SELECT * FROM users WHERE id = ?"),
        attribute.String("db.table", "users"),
    ),
)
defer span.End()

// Agregar atributos durante la ejecución
span.SetAttributes(
    attribute.Int("db.rows_returned", rowCount),
)

// Marcar errores
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

## Configuraciones Predefinidas

El sistema incluye configuraciones pre-definidas para servicios conocidos:

- `telemetry.WalletServiceConfig`
- `telemetry.PaymentServiceConfig`
- `telemetry.DefaultConfig`

## Endpoints de Métricas

Los servicios que usen este sistema automáticamente exponen métricas en `/metrics` para Prometheus:

```go
// Agregar endpoint de métricas
r.Handle("/metrics", handlers.NewMetricsHandler())
```

## Variables de Entorno

- `OTLP_ENDPOINT`: Endpoint para exportar trazas y métricas (default: http://localhost:4318)
- `TELEMETRY_ENABLED`: Habilitar/deshabilitar telemetría (default: true)

## Integración en Servicios Existentes

### Wallet Service
Ya está integrado usando `telemetry.WalletServiceConfig`

### Payment Service
Ya está integrado usando `telemetry.PaymentServiceConfig`

### Servicios Nuevos

1. Agregar configuración de telemetría al config del servicio
2. Inicializar telemetría en dependencies.go
3. Agregar middleware HTTP en main.go
4. Usar funciones de telemetría en handlers/aplicación

## Exportadores

- **Prometheus**: Métricas expuestas en `/metrics`
- **OTLP**: Trazas y métricas enviadas a collector OpenTelemetry
- **Traces**: Trazado distribuido con correlación automática

La telemetría es opcional y el sistema continúa funcionando sin ella si falla la inicialización.