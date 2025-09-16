package telemetry

// Predefined service configurations
var (
	// WalletServiceConfig is the telemetry configuration for the wallet service
	WalletServiceConfig = Config{
		ServiceName:    "wallet-service",
		ServiceVersion: "1.0.0",
	}

	// PaymentServiceConfig is the telemetry configuration for the payment service
	PaymentServiceConfig = Config{
		ServiceName:    "payment-service",
		ServiceVersion: "1.0.0",
	}

	// DefaultConfig is the default telemetry configuration
	DefaultConfig = Config{
		ServiceName:    "unknown-service",
		ServiceVersion: "1.0.0",
	}
)

// NewConfigForService creates a new telemetry config for a custom service
func NewConfigForService(serviceName, version, otlpEndpoint string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: version,
		OTLPEndpoint:   otlpEndpoint,
	}
}

// WithOTLPEndpoint sets the OTLP endpoint for a config
func (c Config) WithOTLPEndpoint(endpoint string) Config {
	c.OTLPEndpoint = endpoint
	return c
}

// WithVersion sets the service version for a config
func (c Config) WithVersion(version string) Config {
	c.ServiceVersion = version
	return c
}