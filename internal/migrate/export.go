// Package migrate provides migration export functionality for sblite.
// It enables exporting schema metadata to PostgreSQL DDL for migration to Supabase.
package migrate

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/markb/sblite/internal/fts"
	"github.com/markb/sblite/internal/schema"
)

// Exporter generates PostgreSQL DDL from sblite schema metadata.
type Exporter struct {
	schema *schema.Schema
	fts    *fts.Manager
}

// New creates a new Exporter with the given schema.
func New(schema *schema.Schema) *Exporter {
	return &Exporter{schema: schema}
}

// NewWithFTS creates a new Exporter with schema and FTS support.
func NewWithFTS(schema *schema.Schema, db *sql.DB) *Exporter {
	return &Exporter{
		schema: schema,
		fts:    fts.NewManager(db),
	}
}

// ExportDDL generates PostgreSQL DDL for all user tables registered in the schema.
// Returns formatted DDL with a header comment.
func (e *Exporter) ExportDDL() (string, error) {
	tables, err := e.schema.ListTables()
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}

	var sb strings.Builder

	// Write header comment
	sb.WriteString("-- PostgreSQL DDL exported from sblite\n")
	sb.WriteString("-- Generated for migration to Supabase/PostgreSQL\n")
	sb.WriteString("--\n")
	sb.WriteString("-- Review and adjust this DDL before executing:\n")
	sb.WriteString("-- - Add foreign key constraints if needed\n")
	sb.WriteString("-- - Add indexes for performance\n")
	sb.WriteString("-- - Review default values and constraints\n")
	sb.WriteString("\n")

	// Export each table
	for i, tableName := range tables {
		tableDDL, err := e.exportTable(tableName)
		if err != nil {
			return "", fmt.Errorf("failed to export table %s: %w", tableName, err)
		}
		sb.WriteString(tableDDL)
		if i < len(tables)-1 {
			sb.WriteString("\n")
		}
	}

	// Export FTS indexes if FTS manager is available
	if e.fts != nil {
		ftsDDL, err := e.exportFTSIndexes(tables)
		if err != nil {
			return "", fmt.Errorf("failed to export FTS indexes: %w", err)
		}
		if ftsDDL != "" {
			sb.WriteString("\n")
			sb.WriteString(ftsDDL)
		}
	}

	return sb.String(), nil
}

// exportTable generates CREATE TABLE DDL for a single table.
func (e *Exporter) exportTable(tableName string) (string, error) {
	columns, err := e.schema.GetColumns(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))

	// Sort columns for consistent output (primary keys first, then alphabetically)
	sortedCols := sortColumns(columns)

	// Track primary key columns
	var primaryKeys []string

	for i, col := range sortedCols {
		sb.WriteString("    ")
		sb.WriteString(col.ColumnName)
		sb.WriteString(" ")
		sb.WriteString(pgTypeToUpper(col.PgType))

		// NOT NULL
		if !col.IsNullable {
			sb.WriteString(" NOT NULL")
		}

		// DEFAULT
		if col.DefaultValue != "" {
			mappedDefault := mapDefaultToPostgres(col.DefaultValue)
			if mappedDefault != "" {
				sb.WriteString(" DEFAULT ")
				sb.WriteString(mappedDefault)
			}
		}

		// Track primary keys
		if col.IsPrimary {
			primaryKeys = append(primaryKeys, col.ColumnName)
		}

		// Add comma if not the last column or if there's a primary key clause
		if i < len(sortedCols)-1 || len(primaryKeys) > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	// Add PRIMARY KEY constraint
	if len(primaryKeys) > 0 {
		sb.WriteString("    PRIMARY KEY (")
		sb.WriteString(strings.Join(primaryKeys, ", "))
		sb.WriteString(")\n")
	}

	sb.WriteString(");\n")

	return sb.String(), nil
}

// sortColumns returns columns sorted with primary keys first, then alphabetically.
func sortColumns(columns map[string]schema.Column) []schema.Column {
	result := make([]schema.Column, 0, len(columns))
	for _, col := range columns {
		result = append(result, col)
	}

	sort.Slice(result, func(i, j int) bool {
		// Primary keys come first
		if result[i].IsPrimary != result[j].IsPrimary {
			return result[i].IsPrimary
		}
		// Then sort alphabetically
		return result[i].ColumnName < result[j].ColumnName
	})

	return result
}

// pgTypeToUpper converts a PostgreSQL type to uppercase for DDL output.
func pgTypeToUpper(pgType string) string {
	return strings.ToUpper(pgType)
}

// mapDefaultToPostgres converts SQLite/sblite default values to PostgreSQL equivalents.
func mapDefaultToPostgres(defaultVal string) string {
	if defaultVal == "" {
		return ""
	}

	// Map gen_uuid() to gen_random_uuid()
	if defaultVal == "gen_uuid()" {
		return "gen_random_uuid()"
	}

	// Keep now() as-is
	if defaultVal == "now()" {
		return "now()"
	}

	// Keep boolean literals as-is
	if defaultVal == "true" || defaultVal == "false" {
		return defaultVal
	}

	// Keep numeric literals as-is
	if isNumeric(defaultVal) {
		return defaultVal
	}

	// Quote string literals
	return "'" + defaultVal + "'"
}

// isNumeric returns true if the string represents a numeric value.
var numericRegex = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

func isNumeric(s string) bool {
	return numericRegex.MatchString(s)
}

// exportFTSIndexes generates PostgreSQL DDL for FTS indexes.
// Creates GIN indexes on to_tsvector() expressions.
func (e *Exporter) exportFTSIndexes(tables []string) (string, error) {
	var sb strings.Builder
	hasIndexes := false

	for _, tableName := range tables {
		indexes, err := e.fts.ListIndexes(tableName)
		if err != nil {
			continue // Skip if error (table might not have FTS)
		}

		for _, idx := range indexes {
			if !hasIndexes {
				sb.WriteString("-- Full-Text Search Indexes\n")
				sb.WriteString("-- Note: PostgreSQL uses tsvector columns and GIN indexes for FTS\n")
				sb.WriteString("-- Adjust the text search configuration ('english') as needed\n")
				sb.WriteString("\n")
				hasIndexes = true
			}

			// Map sblite tokenizer to PostgreSQL text search configuration
			tsConfig := mapTokenizerToTSConfig(idx.Tokenizer)

			// Build the to_tsvector expression for multiple columns
			// PostgreSQL uses: to_tsvector('english', coalesce(col1, '') || ' ' || coalesce(col2, ''))
			var tsvectorExpr string
			if len(idx.Columns) == 1 {
				tsvectorExpr = fmt.Sprintf("to_tsvector('%s', coalesce(%s, ''))",
					tsConfig, idx.Columns[0])
			} else {
				parts := make([]string, len(idx.Columns))
				for i, col := range idx.Columns {
					parts[i] = fmt.Sprintf("coalesce(%s, '')", col)
				}
				tsvectorExpr = fmt.Sprintf("to_tsvector('%s', %s)",
					tsConfig, strings.Join(parts, " || ' ' || "))
			}

			// Generate index name: {table}_{index_name}_fts_idx
			indexName := fmt.Sprintf("%s_%s_fts_idx", tableName, idx.IndexName)

			// Write the CREATE INDEX statement
			sb.WriteString(fmt.Sprintf("CREATE INDEX %s ON %s USING GIN (%s);\n",
				indexName, tableName, tsvectorExpr))
		}
	}

	return sb.String(), nil
}

// mapTokenizerToTSConfig maps sblite tokenizer names to PostgreSQL text search configurations.
func mapTokenizerToTSConfig(tokenizer string) string {
	switch tokenizer {
	case "porter":
		// Porter stemming maps to 'english' which includes stemming
		return "english"
	case "unicode61":
		// Unicode-aware maps to 'simple' (no stemming, just tokenization)
		return "simple"
	case "ascii":
		// ASCII maps to 'simple'
		return "simple"
	case "trigram":
		// Trigram requires pg_trgm extension - use 'simple' and note in comment
		// The user will need to create a GIN index with gin_trgm_ops instead
		return "simple"
	default:
		return "simple"
	}
}
