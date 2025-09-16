package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Example demonstrates how to use the shared telemetry system
func Example(ctx context.Context) {
	// Example 1: Using telemetry from context (when middleware is active)
	ctx, span := StartSpan(ctx, "example_operation",
		// You can add attributes to spans
	)
	defer span.End()

	// Add attributes to the span
	span.SetAttributes(
		attribute.String("operation.type", "example"),
		attribute.String("user.id", "123"),
	)

	// Example 2: Recording metrics
	// Counter - for counting events
	RecordCounter(ctx, "operations_total", "Total operations performed", 1,
		attribute.String("operation", "example"),
		attribute.String("status", "success"),
	)

	// Histogram - for recording distributions (latency, request size, etc.)
	duration := time.Second * 2
	RecordHistogram(ctx, "operation_duration_seconds", "Operation duration", duration.Seconds(),
		attribute.String("operation", "example"),
	)

	// Gauge - for recording values that can go up and down (memory usage, queue size, etc.)
	RecordGauge(ctx, "active_connections", "Number of active connections", 42,
		attribute.String("service", GetServiceName(ctx)),
	)
}

// ExampleDirectTelemetry shows how to use telemetry directly without context
func ExampleDirectTelemetry() {
	// Create telemetry configuration
	config := NewConfigForService("my-service", "1.0.0", "http://localhost:4318")

	// Initialize telemetry
	ctx := context.Background()
	tel, shutdown, err := InitTelemetry(ctx, config)
	if err != nil {
		panic(err)
	}
	defer shutdown()

	// Use telemetry directly
	ctx = WithTelemetry(ctx, tel)

	// Now you can use all telemetry functions
	Example(ctx)
}