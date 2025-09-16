# Wallet Service Telemetry

## Descripción

Esta implementación de OpenTelemetry permite el seguimiento completo de spans, trazas y métricas para el wallet service, usando inyección de contexto para evitar pasar telemetría como parámetro en los constructores.

## Características

### 🔍 Tracing (Trazas)
- Spans automáticos para todas las operaciones de wallet
- Tracing de requests HTTP con detalles completos
- Propagación de contexto entre servicios
- Exportación a OTLP endpoint configurable

### 📊 Métricas
- **Operaciones de Wallet**: Contador y duración
- **Publicación de Eventos**: Contador y duración
- **Balance de Wallets**: Gauge en tiempo real
- **Requests HTTP**: Contador y duración por endpoint
- Exportación dual: Prometheus + OTLP

### 🎯 Context Injection
- Telemetría inyectada en el contexto HTTP via middleware
- Funciones helper para acceso desde cualquier parte del código
- No requiere modificar constructores de casos de uso

## Configuración

### Variables de Entorno

```bash
OTLP_ENDPOINT=http://localhost:4318  # Endpoint para traces y metrics
```

### Endpoints Expuestos

- `/metrics` - Métricas de Prometheus
- `/health` - Health check

## Uso en Código

### Iniciar Span
```go
ctx, span := telemetry.StartSpan(ctx, "operation_name",
    trace.WithAttributes(
        attribute.String("key", "value"),
    ),
)
defer span.End()
```

### Registrar Métricas
```go
// Operación de wallet
telemetry.RecordWalletOperation(ctx, "create_movement", "success", duration)

// Publicación de evento
telemetry.RecordEventPublish(ctx, "WalletCredited", "success", duration)

// Balance de wallet
telemetry.RecordWalletBalance(ctx, walletID, balance)
```

### Middleware HTTP
El middleware se configura automáticamente en main.go y:
- Inyecta telemetría en el contexto de cada request
- Crea spans para cada endpoint HTTP
- Registra métricas de duración y status code

## Métricas Disponibles

### Contadores
- `wallet_operations_total{operation, status}`
- `events_published_total{event_type, status}`

### Histogramas
- `wallet_operation_duration_seconds{operation, status}`
- `event_publish_duration_seconds{event_type, status}`

### Gauges
- `wallet_balance{wallet_id}`
- `active_wallets_total`

## Integración con Observabilidad

### Jaeger/OTLP
Las trazas se envían al endpoint OTLP configurado, compatible con:
- Jaeger
- OpenTelemetry Collector
- Grafana Tempo
- Otros backends OTLP

### Prometheus
Las métricas están disponibles en `/metrics` para ser scrapeadas por Prometheus.

### Grafana
Combina métricas de Prometheus con trazas de Jaeger para observabilidad completa.

## Ventajas de esta Implementación

✅ **Sin dependency injection**: No necesitas pasar telemetría como parámetro
✅ **Context-aware**: Automáticamente disponible donde tengas contexto
✅ **HTTP tracing**: Cada request se traza automáticamente
✅ **Graceful degradation**: Si falla la telemetría, la app sigue funcionando
✅ **Dual export**: Métricas van tanto a Prometheus como a OTLP
✅ **Production ready**: Configuración robusta con timeouts y shutdown

## Ejemplo Completo

```go
func (uc *CreateMovement) Execute(ctx context.Context, cmd *Command) error {
    // Inicia automáticamente span y métricas
    start := time.Now()
    ctx, span := telemetry.StartSpan(ctx, "create_movement")
    defer span.End()

    var status string
    defer func() {
        duration := time.Since(start)
        telemetry.RecordWalletOperation(ctx, "create_movement", status, duration)
    }()

    // Tu lógica de negocio aquí...

    // Registra balance actualizado
    telemetry.RecordWalletBalance(ctx, wallet.ID, balance)

    status = "success"
    return nil
}
```