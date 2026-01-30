package observability

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry holds OTel providers and configuration.
type Telemetry struct {
	config         *Config
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	metrics        *Metrics
	meterReader    any // Stored as any to allow type assertion for ForceFlush
	db             *sql.DB
	shutdownFunc   func(context.Context) error
	_shutdownOnce  sync.Once
	metricsMu      sync.RWMutex
	metricsBuffer  []metricData
	stopFlusher    chan struct{}
}

// metricData holds a single metric data point for storage.
type metricData struct {
	timestamp int64
	name      string
	value     float64
	tags      string
}

// Init initializes OpenTelemetry with the given configuration.
// Returns Telemetry manager, cleanup function, and error.
func Init(ctx context.Context, cfg *Config) (*Telemetry, func(), error) {
	// If disabled, return a no-op telemetry manager
	if !cfg.ShouldEnable() {
		return &Telemetry{config: cfg}, func() {}, nil
	}

	tel := &Telemetry{config: cfg}

	// Initialize tracer provider if enabled
	if cfg.TracesEnabled {
		tp, err := initTracerProvider(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		tel.tracerProvider = tp
		otel.SetTracerProvider(tp)
	}

	// Initialize meter provider if enabled
	if cfg.MetricsEnabled {
		mp, reader, err := initMeterProvider(ctx, cfg)
		if err != nil {
			// Shutdown tracer if meter init fails
			if tp, ok := tel.tracerProvider.(interface{ Shutdown(context.Context) error }); ok {
				_ = tp.Shutdown(ctx)
			}
			return nil, nil, err
		}
		tel.meterProvider = mp
		tel.meterReader = reader
		otel.SetMeterProvider(mp)

		// Initialize metric instruments
		metrics, err := InitMetrics(mp)
		if err != nil {
			// Shutdown providers on failure
			if tp, ok := tel.tracerProvider.(interface{ Shutdown(context.Context) error }); ok {
				_ = tp.Shutdown(ctx)
			}
			if reader != nil {
				// Try to force flush if it's a PeriodicReader
				if pr, ok := reader.(interface{ ForceFlush(context.Context) error }); ok {
					_ = pr.ForceFlush(ctx)
				}
			}
			if mp, ok := mp.(interface{ Shutdown(context.Context) error }); ok {
				_ = mp.Shutdown(ctx)
			}
			return nil, nil, err
		}
		tel.metrics = metrics
	}

	// Combine shutdown functions
	tel.shutdownFunc = func(ctx context.Context) error {
		var errs []error
		if tel.tracerProvider != nil {
			if tp, ok := tel.tracerProvider.(interface{ Shutdown(context.Context) error }); ok {
				if err := tp.Shutdown(ctx); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if tel.meterReader != nil {
			// Force flush metrics before shutdown (only works for PeriodicReader)
			if pr, ok := tel.meterReader.(interface{ ForceFlush(context.Context) error }); ok {
				if err := pr.ForceFlush(ctx); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if tel.meterProvider != nil {
			if mp, ok := tel.meterProvider.(interface{ Shutdown(context.Context) error }); ok {
				if err := mp.Shutdown(ctx); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if len(errs) > 0 {
			return errs[0]
		}
		return nil
	}

	// Start periodic metrics flusher
	tel.startPeriodicFlusher()

	return tel, tel.Cleanup, nil
}

// TracerProvider returns the tracer provider (or noop if disabled).
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	if t.tracerProvider != nil {
		return t.tracerProvider
	}
	return trace.NewNoopTracerProvider()
}

// MeterProvider returns the meter provider (or noop if disabled).
func (t *Telemetry) MeterProvider() metric.MeterProvider {
	if t.meterProvider != nil {
		return t.meterProvider
	}
	return otel.GetMeterProvider()
}

// Metrics returns the metric instruments (or nil if disabled).
func (t *Telemetry) Metrics() *Metrics {
	return t.metrics
}

// Shutdown flushes and closes all providers.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var err error
	t._shutdownOnce.Do(func() {
		// Stop periodic flusher
		t.stopPeriodicFlusher()

		// Final metrics flush
		_ = t.FlushMetrics()

		if t.shutdownFunc != nil {
			err = t.shutdownFunc(ctx)
		}
	})
	return err
}

// Cleanup is a convenience function for defer cleanup.
func (t *Telemetry) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = t.Shutdown(ctx)
}

// Config returns the telemetry configuration.
func (t *Telemetry) Config() *Config {
	return t.config
}

// shutdownTimeout is the maximum time to wait for shutdown.
const shutdownTimeout = 5 * time.Second

// SetDB sets the database connection for metrics storage.
func (t *Telemetry) SetDB(db *sql.DB) {
	t.metricsMu.Lock()
	defer t.metricsMu.Unlock()
	t.db = db
}

// StoreMetric stores a metric data point to the database.
// This is called asynchronously by middleware to avoid blocking requests.
func (t *Telemetry) StoreMetric(timestamp int64, name string, value float64, tags string) {
	if t.db == nil {
		return
	}

	// Buffer metric for batch storage
	t.metricsMu.Lock()
	t.metricsBuffer = append(t.metricsBuffer, metricData{
		timestamp: timestamp,
		name:      name,
		value:     value,
		tags:      tags,
	})
	// Flush if buffer is too large
	if len(t.metricsBuffer) >= 100 {
		t.flushMetricsLocked()
	}
	t.metricsMu.Unlock()
}

// FlushMetrics flushes any buffered metrics to the database.
func (t *Telemetry) FlushMetrics() error {
	t.metricsMu.Lock()
	defer t.metricsMu.Unlock()
	return t.flushMetricsLocked()
}

// flushMetricsLocked flushes buffered metrics. Caller must hold metricsMu.
func (t *Telemetry) flushMetricsLocked() error {
	if len(t.metricsBuffer) == 0 || t.db == nil {
		return nil
	}

	tx, err := t.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO _observability_metrics (timestamp, metric_name, value, tags)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, m := range t.metricsBuffer {
		if _, err := stmt.Exec(m.timestamp, m.name, m.value, m.tags); err != nil {
			return fmt.Errorf("failed to insert metric: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	t.metricsBuffer = t.metricsBuffer[:0] // Clear buffer
	return nil
}

// startPeriodicFlusher starts a background goroutine that periodically flushes metrics.
func (t *Telemetry) startPeriodicFlusher() {
	if t.db == nil {
		return
	}

	t.stopFlusher = make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = t.FlushMetrics()
			case <-t.stopFlusher:
				// Final flush before stopping
				_ = t.FlushMetrics()
				return
			}
		}
	}()
}

// stopPeriodicFlusher stops the periodic flusher goroutine.
func (t *Telemetry) stopPeriodicFlusher() {
	if t.stopFlusher != nil {
		close(t.stopFlusher)
		t.stopFlusher = nil
	}
}
