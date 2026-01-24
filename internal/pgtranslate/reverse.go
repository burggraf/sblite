package pgtranslate

import (
	"fmt"
	"regexp"
	"strings"
)

// ReverseTranslator translates SQLite SQL to PostgreSQL.
type ReverseTranslator struct {
	funcMapper *FunctionMapper
	typeMapper *TypeMapper
}

// NewReverseTranslator creates a new reverse translator.
func NewReverseTranslator() *ReverseTranslator {
	return &ReverseTranslator{
		funcMapper: NewFunctionMapper(),
		typeMapper: NewTypeMapper(),
	}
}

// Translate converts SQLite SQL to PostgreSQL.
func (r *ReverseTranslator) Translate(sql string) string {
	result := sql

	// Apply reverse function translations
	result = r.translateFunctions(result)

	// Apply reverse type translations (for DDL)
	result = r.translateTypes(result)

	// Translate SQLite-specific syntax
	result = r.translateSyntax(result)

	return result
}

// translateFunctions converts SQLite functions to PostgreSQL equivalents.
func (r *ReverseTranslator) translateFunctions(sql string) string {
	result := sql

	// strftime('%Y-%m-%d %H:%M:%f+00', 'now') -> NOW() (PostgreSQL-compatible timestamp format)
	result = regexp.MustCompile(`(?i)strftime\s*\(\s*'[^']+'\s*,\s*'now'\s*\)`).ReplaceAllString(result, "NOW()")

	// datetime('now') -> NOW()
	result = regexp.MustCompile(`(?i)datetime\s*\(\s*'now'\s*\)`).ReplaceAllString(result, "NOW()")

	// date('now') -> CURRENT_DATE
	result = regexp.MustCompile(`(?i)date\s*\(\s*'now'\s*\)`).ReplaceAllString(result, "CURRENT_DATE")

	// time('now') -> CURRENT_TIME
	result = regexp.MustCompile(`(?i)time\s*\(\s*'now'\s*\)`).ReplaceAllString(result, "CURRENT_TIME")

	// SUBSTR(str, 1, n) -> LEFT(str, n)
	result = regexp.MustCompile(`(?i)SUBSTR\s*\(\s*([^,]+)\s*,\s*1\s*,\s*(\d+)\s*\)`).ReplaceAllString(result, "LEFT($1, $2)")

	// SUBSTR(str, -n) -> RIGHT(str, n)
	result = regexp.MustCompile(`(?i)SUBSTR\s*\(\s*([^,]+)\s*,\s*-(\d+)\s*\)`).ReplaceAllString(result, "RIGHT($1, $2)")

	// INSTR(str, substr) -> POSITION(substr IN str)
	result = regexp.MustCompile(`(?i)INSTR\s*\(\s*([^,]+)\s*,\s*([^)]+)\s*\)`).ReplaceAllString(result, "POSITION($2 IN $1)")

	// GROUP_CONCAT(expr, sep) -> STRING_AGG(expr, sep)
	result = regexp.MustCompile(`(?i)GROUP_CONCAT\s*\(\s*([^,]+)\s*,\s*([^)]+)\s*\)`).ReplaceAllString(result, "STRING_AGG($1, $2)")

	// GROUP_CONCAT(expr) -> STRING_AGG(expr, ',')
	result = regexp.MustCompile(`(?i)GROUP_CONCAT\s*\(\s*([^,)]+)\s*\)`).ReplaceAllString(result, "STRING_AGG($1, ',')")

	// json_extract(col, '$.key') -> col->>'key'
	result = regexp.MustCompile(`(?i)json_extract\s*\(\s*(\w+)\s*,\s*'\$\.([^']+)'\s*\)`).ReplaceAllString(result, "$1->>'$2'")

	// julianday('now') - julianday(x) -> AGE(x)  (approximate)
	result = regexp.MustCompile(`(?i)\(julianday\s*\(\s*'now'\s*\)\s*-\s*julianday\s*\(\s*([^)]+)\s*\)\)`).ReplaceAllString(result, "AGE($1)")

	// Boolean values: 1 -> TRUE, 0 -> FALSE (in boolean context only - be careful)
	// This is context-dependent, so we're conservative

	return result
}

// translateTypes converts SQLite types to PostgreSQL types.
func (r *ReverseTranslator) translateTypes(sql string) string {
	// These are applied in CREATE TABLE context
	// The migrate/export.go handles this more precisely using _columns metadata
	return sql
}

// translateSyntax converts SQLite-specific syntax to PostgreSQL.
func (r *ReverseTranslator) translateSyntax(sql string) string {
	result := sql

	// INSERT OR IGNORE -> INSERT ... ON CONFLICT DO NOTHING
	// This is complex - need to restructure the statement
	// For now, just note this limitation

	// AUTOINCREMENT -> SERIAL (handled by type mapping)

	// || for concat is the same in both

	return result
}

// TranslateDefault converts a SQLite default value to PostgreSQL.
// This uses column metadata to make accurate conversions.
func (r *ReverseTranslator) TranslateDefault(sqliteDefault string, pgType string) string {
	if sqliteDefault == "" {
		return ""
	}

	// Handle special SQLite functions
	lowerDefault := strings.ToLower(sqliteDefault)
	switch lowerDefault {
	case "datetime('now')", "(datetime('now'))":
		return "NOW()"
	case "date('now')", "(date('now'))":
		return "CURRENT_DATE"
	case "time('now')", "(time('now'))":
		return "CURRENT_TIME"
	}

	// Handle PostgreSQL-compatible strftime timestamp format
	if strings.Contains(lowerDefault, "strftime") && strings.Contains(lowerDefault, "'now'") {
		return "NOW()"
	}

	// Handle gen_uuid() which is our internal UUID function
	if strings.ToLower(sqliteDefault) == "gen_uuid()" {
		return "gen_random_uuid()"
	}

	// Handle the complex UUID generation subquery we use
	if strings.Contains(strings.ToLower(sqliteDefault), "select lower") &&
		strings.Contains(strings.ToLower(sqliteDefault), "randomblob") {
		return "gen_random_uuid()"
	}

	// Handle boolean defaults based on type
	upperType := strings.ToUpper(pgType)
	if upperType == "BOOLEAN" || upperType == "BOOL" {
		switch sqliteDefault {
		case "1", "(1)":
			return "TRUE"
		case "0", "(0)":
			return "FALSE"
		}
	}

	// Strip parentheses if present
	if strings.HasPrefix(sqliteDefault, "(") && strings.HasSuffix(sqliteDefault, ")") {
		return sqliteDefault[1 : len(sqliteDefault)-1]
	}

	return sqliteDefault
}

// TranslateType converts a SQLite storage type to PostgreSQL based on metadata.
// If pgType is provided from _columns, use it directly; otherwise infer.
func (r *ReverseTranslator) TranslateType(sqliteType string, pgType string) string {
	// If we have PostgreSQL type from metadata, use it
	if pgType != "" {
		return pgType
	}

	// Otherwise, infer from SQLite type
	return r.InferPostgreSQLType(sqliteType)
}

// InferPostgreSQLType infers a PostgreSQL type from SQLite storage type.
func (r *ReverseTranslator) InferPostgreSQLType(sqliteType string) string {
	upper := strings.ToUpper(sqliteType)

	switch {
	case strings.Contains(upper, "INT"):
		return "INTEGER"
	case strings.Contains(upper, "CHAR"), strings.Contains(upper, "TEXT"), strings.Contains(upper, "CLOB"):
		return "TEXT"
	case strings.Contains(upper, "BLOB"):
		return "BYTEA"
	case strings.Contains(upper, "REAL"), strings.Contains(upper, "FLOA"), strings.Contains(upper, "DOUB"):
		return "DOUBLE PRECISION"
	case strings.Contains(upper, "BOOL"):
		return "BOOLEAN"
	case strings.Contains(upper, "NUMERIC"), strings.Contains(upper, "DECIMAL"):
		return "NUMERIC"
	default:
		return "TEXT"
	}
}

// ReverseTranslate is a convenience function for quick reverse translation.
func ReverseTranslate(sql string) string {
	return NewReverseTranslator().Translate(sql)
}

// ReverseTranslateDefault translates a SQLite default to PostgreSQL.
func ReverseTranslateDefault(sqliteDefault, pgType string) string {
	return NewReverseTranslator().TranslateDefault(sqliteDefault, pgType)
}

// ----------------------------------------------------------------------------
// DDL Generation helpers for migration export
// ----------------------------------------------------------------------------

// GeneratePostgreSQLColumn generates a PostgreSQL column definition.
func GeneratePostgreSQLColumn(name, pgType string, notNull bool, defaultVal string, isPrimary bool) string {
	var parts []string
	parts = append(parts, name)
	parts = append(parts, strings.ToUpper(pgType))

	if notNull {
		parts = append(parts, "NOT NULL")
	}

	if defaultVal != "" {
		parts = append(parts, "DEFAULT")
		parts = append(parts, defaultVal)
	}

	if isPrimary {
		parts = append(parts, "PRIMARY KEY")
	}

	return strings.Join(parts, " ")
}

// GeneratePostgreSQLTable generates a CREATE TABLE statement.
func GeneratePostgreSQLTable(tableName string, columns []ColumnSpec) string {
	var sb strings.Builder

	sb.WriteString("CREATE TABLE ")
	sb.WriteString(tableName)
	sb.WriteString(" (\n")

	var primaryKeys []string

	for i, col := range columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("    ")
		sb.WriteString(col.Name)
		sb.WriteString(" ")
		sb.WriteString(strings.ToUpper(col.PgType))

		if col.NotNull {
			sb.WriteString(" NOT NULL")
		}

		if col.Default != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.Default)
		}

		if col.IsPrimary {
			primaryKeys = append(primaryKeys, col.Name)
		}
	}

	if len(primaryKeys) > 0 {
		sb.WriteString(",\n    PRIMARY KEY (")
		sb.WriteString(strings.Join(primaryKeys, ", "))
		sb.WriteString(")")
	}

	sb.WriteString("\n);")

	return sb.String()
}

// ColumnSpec describes a column for DDL generation.
type ColumnSpec struct {
	Name      string
	PgType    string
	NotNull   bool
	Default   string
	IsPrimary bool
}

// GeneratePostgreSQLFunction generates a CREATE FUNCTION statement.
func GeneratePostgreSQLFunction(fn *CreateFunctionStmt) (string, error) {
	gen := NewGenerator(WithDialect(DialectPostgreSQL))
	return gen.Generate(fn)
}

// TranslateExpressionToPostgreSQL translates a single expression.
func TranslateExpressionToPostgreSQL(expr string) (string, error) {
	parser := NewParser(expr)
	ast, err := parser.ParseExpr()
	if err != nil {
		// Fall back to simple regex-based reverse translation
		return ReverseTranslate(expr), nil
	}

	gen := NewGenerator(WithDialect(DialectPostgreSQL))
	return gen.Generate(ast)
}

// TranslateFunctionBodyToPostgreSQL translates a function body from SQLite to PostgreSQL.
func TranslateFunctionBodyToPostgreSQL(body string) string {
	return ReverseTranslate(body)
}

// ----------------------------------------------------------------------------
// Interval Handling
// ----------------------------------------------------------------------------

// TranslateInterval converts SQLite datetime modifier to PostgreSQL INTERVAL.
func TranslateInterval(sqliteInterval string) string {
	// SQLite uses '+1 day', '+2 hours', etc.
	// PostgreSQL uses INTERVAL '1 day', INTERVAL '2 hours'

	interval := strings.TrimSpace(sqliteInterval)
	interval = strings.Trim(interval, "'\"")

	// Remove leading + if present
	interval = strings.TrimPrefix(interval, "+")

	return fmt.Sprintf("INTERVAL '%s'", interval)
}
