// Package migrate provides migration export functionality for sblite.
// It enables exporting schema metadata to PostgreSQL DDL for migration to Supabase.
package migrate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/markb/sblite/internal/schema"
)

// Exporter generates PostgreSQL DDL from sblite schema metadata.
type Exporter struct {
	schema *schema.Schema
}

// New creates a new Exporter with the given schema.
func New(schema *schema.Schema) *Exporter {
	return &Exporter{schema: schema}
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
