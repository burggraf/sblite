package pgtranslate

import (
	"fmt"
	"strings"
)

// FunctionMapper handles function name and signature translation between PostgreSQL and SQLite.
type FunctionMapper struct {
	pgToSQLite map[string]FunctionTransformer
	sqliteToPG map[string]FunctionTransformer
}

// FunctionTransformer transforms a function call to the target dialect.
type FunctionTransformer func(call *FunctionCall, gen *Generator) (string, bool)

// NewFunctionMapper creates a new function mapper with default mappings.
func NewFunctionMapper() *FunctionMapper {
	m := &FunctionMapper{
		pgToSQLite: make(map[string]FunctionTransformer),
		sqliteToPG: make(map[string]FunctionTransformer),
	}
	m.registerDefaults()
	return m
}

// MapToSQLite maps a PostgreSQL function call to SQLite.
func (m *FunctionMapper) MapToSQLite(call *FunctionCall) (string, bool) {
	name := strings.ToUpper(call.Name)
	if transformer, ok := m.pgToSQLite[name]; ok {
		return transformer(call, nil)
	}
	return "", false
}

// MapToPostgreSQL maps a SQLite function call to PostgreSQL.
func (m *FunctionMapper) MapToPostgreSQL(call *FunctionCall) (string, bool) {
	name := strings.ToUpper(call.Name)
	if transformer, ok := m.sqliteToPG[name]; ok {
		return transformer(call, nil)
	}
	return "", false
}

func (m *FunctionMapper) registerDefaults() {
	// Date/Time functions: PostgreSQL -> SQLite
	// Use strftime for PostgreSQL-compatible timestamptz format with milliseconds and UTC offset
	m.pgToSQLite["NOW"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return "strftime('%Y-%m-%d %H:%M:%f+00', 'now')", true
	}
	m.pgToSQLite["CURRENT_TIMESTAMP"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return "strftime('%Y-%m-%d %H:%M:%f+00', 'now')", true
	}
	m.pgToSQLite["CURRENT_DATE"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return "date('now')", true
	}
	m.pgToSQLite["CURRENT_TIME"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return "time('now')", true
	}

	// UUID generation
	m.pgToSQLite["GEN_RANDOM_UUID"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		// RFC 4122 compliant UUID v4
		return `(SELECT lower(
    substr(h, 1, 8) || '-' ||
    substr(h, 9, 4) || '-' ||
    '4' || substr(h, 14, 3) || '-' ||
    substr('89ab', (abs(random()) % 4) + 1, 1) || substr(h, 18, 3) || '-' ||
    substr(h, 21, 12)
  ) FROM (SELECT hex(randomblob(16)) as h))`, true
	}

	// String functions: PostgreSQL -> SQLite
	m.pgToSQLite["LEFT"] = transformLeft
	m.pgToSQLite["RIGHT"] = transformRight
	m.pgToSQLite["POSITION"] = transformPosition
	m.pgToSQLite["STRING_AGG"] = transformStringAgg
	m.pgToSQLite["CONCAT"] = transformConcat
	m.pgToSQLite["CONCAT_WS"] = transformConcatWS

	// Comparison functions
	m.pgToSQLite["GREATEST"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return transformFuncRename(call, "MAX")
	}
	m.pgToSQLite["LEAST"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		return transformFuncRename(call, "MIN")
	}

	// Age function
	m.pgToSQLite["AGE"] = transformAge

	// Date/Time functions: SQLite -> PostgreSQL (reverse)
	m.sqliteToPG["DATETIME"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		if len(call.Args) == 1 {
			if lit, ok := call.Args[0].(*Literal); ok && lit.Type == LitString && lit.Value == "now" {
				return "NOW()", true
			}
		}
		return "", false
	}
	m.sqliteToPG["DATE"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		if len(call.Args) == 1 {
			if lit, ok := call.Args[0].(*Literal); ok && lit.Type == LitString && lit.Value == "now" {
				return "CURRENT_DATE", true
			}
		}
		return "", false
	}
	m.sqliteToPG["TIME"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		if len(call.Args) == 1 {
			if lit, ok := call.Args[0].(*Literal); ok && lit.Type == LitString && lit.Value == "now" {
				return "CURRENT_TIME", true
			}
		}
		return "", false
	}

	// String functions: SQLite -> PostgreSQL
	m.sqliteToPG["SUBSTR"] = transformSubstrToPG
	m.sqliteToPG["GROUP_CONCAT"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		if len(call.Args) < 1 {
			return "", false
		}
		args := make([]string, len(call.Args))
		for i, arg := range call.Args {
			args[i] = exprToString(arg)
		}
		if len(args) == 1 {
			return fmt.Sprintf("STRING_AGG(%s, ',')", args[0]), true
		}
		return fmt.Sprintf("STRING_AGG(%s, %s)", args[0], args[1]), true
	}
	m.sqliteToPG["INSTR"] = func(call *FunctionCall, _ *Generator) (string, bool) {
		if len(call.Args) != 2 {
			return "", false
		}
		// INSTR(string, substring) -> POSITION(substring IN string)
		return fmt.Sprintf("POSITION(%s IN %s)", exprToString(call.Args[1]), exprToString(call.Args[0])), true
	}
}

// Helper function transformers

func transformLeft(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) != 2 {
		return "", false
	}
	str := exprToString(call.Args[0])
	n := exprToString(call.Args[1])
	return fmt.Sprintf("SUBSTR(%s, 1, %s)", str, n), true
}

func transformRight(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) != 2 {
		return "", false
	}
	str := exprToString(call.Args[0])
	n := exprToString(call.Args[1])
	return fmt.Sprintf("SUBSTR(%s, -%s)", str, n), true
}

func transformPosition(call *FunctionCall, _ *Generator) (string, bool) {
	// POSITION expects a special syntax: POSITION(substring IN string)
	// But after parsing, we get it as POSITION(args...)
	// The parser should handle this, but for now we support both syntaxes
	if len(call.Args) != 2 {
		return "", false
	}
	substring := exprToString(call.Args[0])
	str := exprToString(call.Args[1])
	return fmt.Sprintf("INSTR(%s, %s)", str, substring), true
}

func transformStringAgg(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) < 2 {
		return "", false
	}
	expr := exprToString(call.Args[0])
	delimiter := exprToString(call.Args[1])

	result := fmt.Sprintf("GROUP_CONCAT(%s, %s)", expr, delimiter)

	// Handle ORDER BY if present
	if len(call.OrderBy) > 0 {
		// SQLite's GROUP_CONCAT doesn't support ORDER BY directly
		// We'd need to use a subquery, but that's complex
		// For now, just ignore the ORDER BY
	}

	return result, true
}

func transformConcat(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) == 0 {
		return "''", true
	}

	parts := make([]string, len(call.Args))
	for i, arg := range call.Args {
		parts[i] = exprToString(arg)
	}

	// SQLite uses || for concatenation
	return "(" + strings.Join(parts, " || ") + ")", true
}

func transformConcatWS(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) < 2 {
		return "", false
	}

	separator := exprToString(call.Args[0])
	parts := make([]string, len(call.Args)-1)
	for i, arg := range call.Args[1:] {
		parts[i] = exprToString(arg)
	}

	// Build: COALESCE(arg1, '') || sep || COALESCE(arg2, '') || ...
	coalescedParts := make([]string, len(parts))
	for i, part := range parts {
		coalescedParts[i] = fmt.Sprintf("COALESCE(%s, '')", part)
	}

	// For CONCAT_WS, we need to handle NULLs and empty strings
	// This is a simplified version - full implementation would filter out NULLs
	result := strings.Join(coalescedParts, " || "+separator+" || ")
	return "(" + result + ")", true
}

func transformAge(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) < 1 {
		return "", false
	}

	timestamp := exprToString(call.Args[0])

	// AGE(timestamp) returns interval
	// SQLite: julianday('now') - julianday(timestamp)
	if len(call.Args) == 1 {
		return fmt.Sprintf("(julianday('now') - julianday(%s))", timestamp), true
	}

	// AGE(timestamp1, timestamp2) returns interval between two timestamps
	timestamp2 := exprToString(call.Args[1])
	return fmt.Sprintf("(julianday(%s) - julianday(%s))", timestamp, timestamp2), true
}

func transformFuncRename(call *FunctionCall, newName string) (string, bool) {
	if len(call.Args) == 0 {
		return newName + "()", true
	}

	args := make([]string, len(call.Args))
	for i, arg := range call.Args {
		args[i] = exprToString(arg)
	}

	return fmt.Sprintf("%s(%s)", newName, strings.Join(args, ", ")), true
}

func transformSubstrToPG(call *FunctionCall, _ *Generator) (string, bool) {
	if len(call.Args) < 2 {
		return "", false
	}

	str := exprToString(call.Args[0])
	start := exprToString(call.Args[1])

	// Check if this looks like a LEFT or RIGHT translation
	if start == "1" && len(call.Args) == 3 {
		// SUBSTR(str, 1, n) -> LEFT(str, n)
		n := exprToString(call.Args[2])
		return fmt.Sprintf("LEFT(%s, %s)", str, n), true
	}

	// Check for negative start (RIGHT)
	if strings.HasPrefix(start, "-") && len(call.Args) == 2 {
		// SUBSTR(str, -n) -> RIGHT(str, n)
		n := strings.TrimPrefix(start, "-")
		return fmt.Sprintf("RIGHT(%s, %s)", str, n), true
	}

	// General case: keep as SUBSTR (PostgreSQL also supports it)
	if len(call.Args) == 2 {
		return fmt.Sprintf("SUBSTR(%s, %s)", str, start), true
	}
	length := exprToString(call.Args[2])
	return fmt.Sprintf("SUBSTR(%s, %s, %s)", str, start, length), true
}

// exprToString converts an expression to its string representation.
// This is a simple conversion for use in function transformers.
func exprToString(expr Expr) string {
	switch e := expr.(type) {
	case *Identifier:
		if e.Quoted {
			return `"` + e.Name + `"`
		}
		return e.Name
	case *QualifiedRef:
		return e.Table + "." + e.Column
	case *Literal:
		switch e.Type {
		case LitString:
			return "'" + strings.ReplaceAll(e.Value, "'", "''") + "'"
		case LitNumber:
			return e.Value
		case LitBoolean:
			return e.Value
		case LitNull:
			return "NULL"
		default:
			return e.Value
		}
	case *BinaryOp:
		left := exprToString(e.Left)
		right := exprToString(e.Right)
		return fmt.Sprintf("(%s %s %s)", left, e.Op, right)
	case *UnaryOp:
		operand := exprToString(e.Operand)
		if e.Op == "NOT" {
			return fmt.Sprintf("NOT %s", operand)
		}
		return fmt.Sprintf("%s%s", e.Op, operand)
	case *FunctionCall:
		args := make([]string, len(e.Args))
		for i, arg := range e.Args {
			args[i] = exprToString(arg)
		}
		if e.Star {
			return e.Name + "(*)"
		}
		if e.Distinct {
			return fmt.Sprintf("%s(DISTINCT %s)", e.Name, strings.Join(args, ", "))
		}
		return fmt.Sprintf("%s(%s)", e.Name, strings.Join(args, ", "))
	case *ParenExpr:
		return "(" + exprToString(e.Expr) + ")"
	case *TypeCast:
		return exprToString(e.Expr) + "::" + e.TypeName
	case *StarExpr:
		if e.Table != "" {
			return e.Table + ".*"
		}
		return "*"
	default:
		return fmt.Sprintf("%v", expr)
	}
}

// TypeMapper handles type translation between PostgreSQL and SQLite.
type TypeMapper struct {
	pgToSQLite map[string]string
	sqliteToPG map[string]string
}

// NewTypeMapper creates a new type mapper with default mappings.
func NewTypeMapper() *TypeMapper {
	m := &TypeMapper{
		pgToSQLite: map[string]string{
			"UUID":        "TEXT",
			"BOOLEAN":     "INTEGER",
			"BOOL":        "INTEGER",
			"TIMESTAMPTZ": "TEXT",
			"TIMESTAMP":   "TEXT",
			"JSONB":       "TEXT",
			"JSON":        "TEXT",
			"SERIAL":      "INTEGER",
			"BIGSERIAL":   "INTEGER",
			"SMALLSERIAL": "INTEGER",
			"BIGINT":      "INTEGER",
			"SMALLINT":    "INTEGER",
			"INT":         "INTEGER",
			"INTEGER":     "INTEGER",
			"REAL":        "REAL",
			"FLOAT":       "REAL",
			"DOUBLE":      "REAL",
			"NUMERIC":     "NUMERIC",
			"DECIMAL":     "NUMERIC",
			"TEXT":        "TEXT",
			"VARCHAR":     "TEXT",
			"CHAR":        "TEXT",
			"BYTEA":       "BLOB",
			"DATE":        "TEXT",
			"TIME":        "TEXT",
			"INTERVAL":    "TEXT",
			// Vector type - special handling needed
			"VECTOR":      "TEXT",
		},
		sqliteToPG: map[string]string{
			"TEXT":    "TEXT",
			"INTEGER": "INTEGER",
			"REAL":    "DOUBLE PRECISION",
			"BLOB":    "BYTEA",
			"NUMERIC": "NUMERIC",
		},
	}
	return m
}

// MapToSQLite maps a PostgreSQL type to SQLite.
func (m *TypeMapper) MapToSQLite(pgType string) string {
	upper := strings.ToUpper(pgType)

	// Handle parameterized types like VARCHAR(255)
	if idx := strings.Index(upper, "("); idx != -1 {
		baseType := upper[:idx]
		if mapped, ok := m.pgToSQLite[baseType]; ok {
			return mapped
		}
	}

	// Handle array types
	if strings.HasSuffix(upper, "[]") {
		return "TEXT" // Store arrays as JSON in SQLite
	}

	// Handle vector type with dimension
	if strings.HasPrefix(upper, "VECTOR") {
		return "TEXT"
	}

	if mapped, ok := m.pgToSQLite[upper]; ok {
		return mapped
	}

	// Types that can be removed entirely (for type casts)
	removeTypes := map[string]bool{
		"UUID": true, "TEXT": true, "TIMESTAMPTZ": true,
		"TIMESTAMP": true, "INTEGER": true, "INT": true,
	}
	if removeTypes[upper] {
		return ""
	}

	return "TEXT" // Default to TEXT
}

// MapToPostgreSQL maps a SQLite type to PostgreSQL.
func (m *TypeMapper) MapToPostgreSQL(sqliteType string) string {
	upper := strings.ToUpper(sqliteType)
	if mapped, ok := m.sqliteToPG[upper]; ok {
		return mapped
	}
	return "TEXT"
}
