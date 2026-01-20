// internal/rpc/interceptor.go
package rpc

import (
	"fmt"

	"github.com/markb/sblite/internal/pgtranslate"
)

// Interceptor intercepts CREATE/DROP FUNCTION statements.
type Interceptor struct {
	store *Store
}

// NewInterceptor creates a new Interceptor.
func NewInterceptor(store *Store) *Interceptor {
	return &Interceptor{store: store}
}

// ProcessSQL processes a SQL statement, handling CREATE/DROP FUNCTION.
// Returns (result message, handled, error).
// If handled is true, the caller should not execute the SQL normally.
func (i *Interceptor) ProcessSQL(sql string, postgresMode bool) (string, bool, error) {
	// Check for CREATE FUNCTION
	if IsCreateFunction(sql) {
		return i.handleCreateFunction(sql, postgresMode)
	}

	// Check for DROP FUNCTION
	if IsDropFunction(sql) {
		return i.handleDropFunction(sql)
	}

	return "", false, nil
}

func (i *Interceptor) handleCreateFunction(sql string, postgresMode bool) (string, bool, error) {
	// Parse the CREATE FUNCTION statement
	parsed, err := ParseCreateFunction(sql)
	if err != nil {
		return "", true, fmt.Errorf("parse CREATE FUNCTION: %w", err)
	}

	// Translate the body from PostgreSQL to SQLite
	var translatedBody string
	if postgresMode {
		translatedBody = pgtranslate.Translate(parsed.Body)
	} else {
		translatedBody = parsed.Body
	}

	// Prepare the SQLite source with parameter placeholders
	sqliteSource := PrepareSource(translatedBody, parsed.Args)

	// Create the function definition
	def := &FunctionDef{
		Name:         parsed.Name,
		Language:     parsed.Language,
		ReturnType:   parsed.ReturnType,
		ReturnsSet:   parsed.ReturnsSet,
		Volatility:   parsed.Volatility,
		Security:     parsed.Security,
		SourcePG:     parsed.Body,
		SourceSQLite: sqliteSource,
		Args:         parsed.Args,
	}

	// Store the function
	var storeErr error
	if parsed.OrReplace {
		storeErr = i.store.CreateOrReplace(def)
	} else {
		if i.store.Exists(parsed.Name) {
			return "", true, fmt.Errorf("function %q already exists", parsed.Name)
		}
		storeErr = i.store.Create(def)
	}

	if storeErr != nil {
		return "", true, fmt.Errorf("store function: %w", storeErr)
	}

	return fmt.Sprintf("CREATE FUNCTION %s", parsed.Name), true, nil
}

func (i *Interceptor) handleDropFunction(sql string) (string, bool, error) {
	name, ifExists, err := ParseDropFunction(sql)
	if err != nil {
		return "", true, err
	}

	err = i.store.Delete(name)
	if err != nil {
		if ifExists {
			// IF EXISTS means don't error if not found
			return fmt.Sprintf("DROP FUNCTION %s (not found)", name), true, nil
		}
		return "", true, fmt.Errorf("drop function: %w", err)
	}

	return fmt.Sprintf("DROP FUNCTION %s", name), true, nil
}
