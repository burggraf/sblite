// internal/rpc/executor.go
package rpc

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/markb/sblite/internal/rls"
)

// Executor executes stored functions.
type Executor struct {
	db    *sql.DB
	store *Store
}

// NewExecutor creates a new Executor.
func NewExecutor(db *sql.DB, store *Store) *Executor {
	return &Executor{db: db, store: store}
}

// Execute runs a function with the given arguments.
func (e *Executor) Execute(name string, args map[string]interface{}, authCtx *rls.AuthContext) (*ExecuteResult, error) {
	// Get function definition
	fn, err := e.store.Get(name)
	if err != nil {
		return nil, fmt.Errorf("function %q not found", name)
	}

	// Validate and bind arguments
	boundArgs, err := e.bindArguments(fn, args)
	if err != nil {
		return nil, err
	}

	// Prepare SQL with parameter substitution
	sqlStr := fn.SourceSQLite
	var sqlArgs []interface{}

	// Replace named parameters with positional ones
	for _, arg := range fn.Args {
		placeholder := ":" + arg.Name
		count := strings.Count(sqlStr, placeholder)
		if count > 0 {
			sqlStr = strings.ReplaceAll(sqlStr, placeholder, "?")
			for i := 0; i < count; i++ {
				sqlArgs = append(sqlArgs, boundArgs[arg.Name])
			}
		}
	}

	// Execute the query
	rows, err := e.db.Query(sqlStr, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	// Get column info
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	// Collect results
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	// Format result based on return type
	result := &ExecuteResult{}

	if fn.ReturnsSet {
		result.IsSet = true
		result.Data = results
	} else if len(results) == 0 {
		result.Data = nil
	} else if len(columns) == 1 && !isTableReturn(fn.ReturnType) {
		// Single scalar value
		result.IsScalar = true
		result.Data = results[0][columns[0]]
	} else {
		// Single row
		result.Data = results[0]
	}

	return result, nil
}

// bindArguments validates and binds function arguments, applying defaults.
// Supports both named arguments (by parameter name) and positional arguments ($1, $2, etc.).
func (e *Executor) bindArguments(fn *FunctionDef, provided map[string]interface{}) (map[string]interface{}, error) {
	bound := make(map[string]interface{})

	for i, arg := range fn.Args {
		// Check by parameter name first
		if val, ok := provided[arg.Name]; ok {
			bound[arg.Name] = val
		} else if val, ok := provided[fmt.Sprintf("$%d", i+1)]; ok {
			// Check by positional argument ($1, $2, etc.)
			bound[arg.Name] = val
		} else if arg.DefaultValue != nil {
			bound[arg.Name] = *arg.DefaultValue
		} else {
			return nil, fmt.Errorf("missing required argument: %s", arg.Name)
		}
	}

	return bound, nil
}

// isTableReturn checks if return type is TABLE(...).
func isTableReturn(returnType string) bool {
	return strings.HasPrefix(strings.ToUpper(returnType), "TABLE")
}

// PrepareSource converts PostgreSQL function body to SQLite-ready form.
// It replaces parameter references with named placeholders.
func PrepareSource(body string, args []FunctionArg) string {
	result := body

	// Replace PostgreSQL positional parameters ($1, $2, etc.) with named placeholders
	for i, arg := range args {
		// Replace $N with :param_name
		positional := fmt.Sprintf("$%d", i+1)
		result = strings.ReplaceAll(result, positional, ":"+arg.Name)
	}

	return result
}
