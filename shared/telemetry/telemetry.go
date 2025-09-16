package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	metricSDK "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSDK "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds telemetry configuration for a service
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
}

type Telemetry struct {
	tracer trace.Tracer
	meter  metric.Meter
	config Config
}

// NewTelemetry creates a new telemetry instance with the given config
func NewTelemetry(config Config) *Telemetry {
	return &Telemetry{
		config: config,
		tracer: otel.Tracer(config.ServiceName),
		meter:  otel.Meter(config.ServiceName),
	}
}

// InitTelemetry initializes OpenTelemetry with OTLP and Prometheus exporters
func InitTelemetry(ctx context.Context, config Config) (*Telemetry, func(), error) {
	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// Set up tracing
	traceProvider, traceShutdown, err := setupTracing(ctx, res, config.OTLPEndpoint)
	if err != nil {
		return nil, nil, err
	}

	// Set up metrics
	meterProvider, metricShutdown, err := setupMetrics(ctx, res, config.OTLPEndpoint)
	if err != nil {
		traceShutdown()
		return nil, nil, err
	}

	// Set global providers
	otel.SetTracerProvider(traceProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create telemetry instance
	tel := &Telemetry{
		config: config,
		tracer: otel.Tracer(config.ServiceName),
		meter:  otel.Meter(config.ServiceName),
	}

	// Combined shutdown function
	shutdown := func() {
		traceShutdown()
		metricShutdown()
	}

	return tel, shutdown, nil
}

func setupTracing(ctx context.Context, res *resource.Resource, otlpEndpoint string) (trace.TracerProvider, func(), error) {
	// Create OTLP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create trace provider
	traceProvider := traceSDK.NewTracerProvider(
		traceSDK.WithBatcher(traceExporter),
		traceSDK.WithResource(res),
		traceSDK.WithSampler(traceSDK.AlwaysSample()),
	)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		traceProvider.Shutdown(ctx)
	}

	return traceProvider, shutdown, nil
}

func setupMetrics(ctx context.Context, res *resource.Resource, otlpEndpoint string) (metric.MeterProvider, func(), error) {
	// Create Prometheus exporter
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, err
	}

	// Create OTLP metric exporter
	otlpExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Create meter provider with both exporters
	meterProvider := metricSDK.NewMeterProvider(
		metricSDK.WithResource(res),
		metricSDK.WithReader(prometheusExporter),
		metricSDK.WithReader(metricSDK.NewPeriodicReader(otlpExporter,
			metricSDK.WithInterval(30*time.Second),
		)),
	)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		meterProvider.Shutdown(ctx)
	}

	return meterProvider, shutdown, nil
}

// StartSpan starts a new trace span (method on Telemetry)
func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// GetMeter returns the meter instance for creating custom metrics
func (t *Telemetry) GetMeter() metric.Meter {
	return t.meter
}

// GetServiceName returns the service name
func (t *Telemetry) GetServiceName() string {
	return t.config.ServiceName
}

// Context key for telemetry
type contextKey string

const telemetryKey contextKey = "telemetry"

// WithTelemetry injects telemetry into context
func WithTelemetry(ctx context.Context, tel *Telemetry) context.Context {
	return context.WithValue(ctx, telemetryKey, tel)
}

// FromContext extracts telemetry from context
func FromContext(ctx context.Context) *Telemetry {
	if tel, ok := ctx.Value(telemetryKey).(*Telemetry); ok {
		return tel
	}
	return nil
}

// StartSpan starts a new trace span using telemetry from context
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tel := FromContext(ctx); tel != nil {
		return tel.StartSpan(ctx, name, opts...)
	}
	// Fallback to global tracer if no telemetry in context
	return otel.Tracer("fallback").Start(ctx, name, opts...)
}

// GetMeter returns meter from context for creating custom metrics
func GetMeter(ctx context.Context) metric.Meter {
	if tel := FromContext(ctx); tel != nil {
		return tel.GetMeter()
	}
	// Fallback to global meter if no telemetry in context
	return otel.Meter("fallback")
}

// GetServiceName returns service name from context
func GetServiceName(ctx context.Context) string {
	if tel := FromContext(ctx); tel != nil {
		return tel.GetServiceName()
	}
	return "unknown"
}

// RecordCounter records a counter metric
func RecordCounter(ctx context.Context, name, description string, value int64, attrs ...attribute.KeyValue) {
	meter := GetMeter(ctx)
	counter, err := meter.Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		return
	}

	// Add service name to attributes if not already present
	serviceName := GetServiceName(ctx)
	attrs = append(attrs, attribute.String("service", serviceName))

	counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

// RecordHistogram records a histogram metric
func RecordHistogram(ctx context.Context, name, description string, value float64, attrs ...attribute.KeyValue) {
	meter := GetMeter(ctx)
	histogram, err := meter.Float64Histogram(name, metric.WithDescription(description))
	if err != nil {
		return
	}

	// Add service name to attributes if not already present
	serviceName := GetServiceName(ctx)
	attrs = append(attrs, attribute.String("service", serviceName))

	histogram.Record(ctx, value, metric.WithAttributes(attrs...))
}

// RecordGauge records a gauge metric
func RecordGauge(ctx context.Context, name, description string, value float64, attrs ...attribute.KeyValue) {
	meter := GetMeter(ctx)
	gauge, err := meter.Float64Gauge(name, metric.WithDescription(description))
	if err != nil {
		return
	}

	// Add service name to attributes if not already present
	serviceName := GetServiceName(ctx)
	attrs = append(attrs, attribute.String("service", serviceName))

	gauge.Record(ctx, value, metric.WithAttributes(attrs...))
}