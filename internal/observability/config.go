package observability

// Config holds OpenTelemetry configuration.
type Config struct {
	// Exporter type: "none", "stdout", or "otlp"
	Exporter string

	// OTLP endpoint (for otlp exporter)
	Endpoint string

	// Service name for telemetry
	ServiceName string

	// Trace sampling rate (0.0 to 1.0)
	SampleRate float64

	// Enable metrics collection
	MetricsEnabled bool

	// Enable trace collection
	TracesEnabled bool
}

// NewConfig returns default configuration.
func NewConfig() *Config {
	return &Config{
		Exporter:       "none",
		Endpoint:       "localhost:4317",
		ServiceName:    "sblite",
		SampleRate:     0.1,
		MetricsEnabled: false,
		TracesEnabled:  false,
	}
}

// ShouldEnable returns true if OTel should be initialized.
func (c *Config) ShouldEnable() bool {
	return c.Exporter != "none"
}
