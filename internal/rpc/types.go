// internal/rpc/types.go
package rpc

import "time"

// FunctionDef represents a stored function definition.
type FunctionDef struct {
	ID           string
	Name         string
	Language     string
	ReturnType   string
	ReturnsSet   bool
	Volatility   string
	Security     string
	SourcePG     string // Original PostgreSQL body
	SourceSQLite string // Translated SQLite body
	Args         []FunctionArg
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FunctionArg represents a function parameter.
type FunctionArg struct {
	ID           string
	FunctionID   string
	Name         string
	Type         string
	Position     int
	DefaultValue *string
}

// ParsedFunction is the result of parsing a CREATE FUNCTION statement.
type ParsedFunction struct {
	Name       string
	Args       []FunctionArg
	ReturnType string
	ReturnsSet bool
	Language   string
	Volatility string
	Security   string
	Body       string
	OrReplace  bool
}

// ExecuteResult holds the result of function execution.
type ExecuteResult struct {
	Data     interface{} // Single value, row, or []row
	IsSet    bool        // True if RETURNS SETOF/TABLE
	IsScalar bool        // True if single scalar value (not row)
}
