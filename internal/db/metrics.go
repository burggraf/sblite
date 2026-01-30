package db

import (
	"database/sql"
)

// CreateMetricsTables creates the metrics tracking tables.
func CreateMetricsTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS _observability_metrics (
			timestamp INTEGER NOT NULL,
			metric_name TEXT NOT NULL,
			value REAL,
			tags TEXT,
			PRIMARY KEY (timestamp, metric_name, tags)
		)`,
		`CREATE INDEX IF NOT EXISTS _observability_metrics_ts_idx ON _observability_metrics(timestamp)`,
		`CREATE INDEX IF NOT EXISTS _observability_metrics_name_idx ON _observability_metrics(metric_name)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
