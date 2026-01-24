// internal/rpc/interceptor.go
package rpc

import (
	"fmt"
	"strings"

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
	// Check for CREATE FUNCTION using pgtranslate helpers
	if pgtranslate.IsCreateFunctionSQL(sql) {
		return i.handleCreateFunction(sql, postgresMode)
	}

	// Check for DROP FUNCTION
	if pgtranslate.IsDropFunctionSQL(sql) {
		return i.handleDropFunction(sql)
	}

	return "", false, nil
}

func (i *Interceptor) handleCreateFunction(sql string, postgresMode bool) (string, bool, error) {
	// Parse the CREATE FUNCTION statement using AST-based parser
	stmt, err := pgtranslate.ParseCreateFunctionSQL(sql)
	if err != nil {
		return "", true, fmt.Errorf("parse CREATE FUNCTION: %w", err)
	}

	// Convert AST to ParsedFunction
	parsed := convertCreateFunctionStmt(stmt)

	// Validate language
	if parsed.Language != "sql" {
		return "", true, fmt.Errorf("only LANGUAGE sql is supported, got %q", parsed.Language)
	}

	// Translate the body from PostgreSQL to SQLite using AST-based translation
	var translatedBody string
	if postgresMode {
		translatedBody = pgtranslate.TranslateToSQLite(parsed.Body)
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
	// Parse DROP FUNCTION using AST-based parser
	stmt, err := pgtranslate.ParseDropFunctionSQL(sql)
	if err != nil {
		return "", true, fmt.Errorf("parse DROP FUNCTION: %w", err)
	}

	name := stmt.Name
	ifExists := stmt.IfExists

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

// convertCreateFunctionStmt converts a pgtranslate.CreateFunctionStmt to ParsedFunction.
func convertCreateFunctionStmt(stmt *pgtranslate.CreateFunctionStmt) *ParsedFunction {
	parsed := &ParsedFunction{
		Name:       stmt.Name,
		Language:   strings.ToLower(stmt.Language),
		Volatility: stmt.Volatility,
		Security:   stmt.Security,
		Body:       stmt.Body,
		OrReplace:  stmt.OrReplace,
	}

	// Set defaults if empty
	if parsed.Language == "" {
		parsed.Language = "sql"
	}
	if parsed.Volatility == "" {
		parsed.Volatility = "VOLATILE"
	}
	if parsed.Security == "" {
		parsed.Security = "INVOKER"
	}

	// Convert return type
	if stmt.Returns != nil {
		parsed.ReturnsSet = stmt.Returns.IsSetOf || stmt.Returns.IsTable
		if stmt.Returns.IsTable && stmt.Returns.TableCols != nil {
			// Reconstruct TABLE(col type, ...) format
			var cols []string
			for _, col := range stmt.Returns.TableCols {
				cols = append(cols, col.Name+" "+col.TypeName)
			}
			parsed.ReturnType = "TABLE(" + strings.Join(cols, ", ") + ")"
		} else {
			parsed.ReturnType = stmt.Returns.TypeName
		}
	}

	// Convert arguments
	for i, arg := range stmt.Args {
		fnArg := FunctionArg{
			Name:     arg.Name,
			Type:     arg.TypeName,
			Position: i,
		}
		if arg.Default != nil {
			// Generate the default expression as a string
			gen := pgtranslate.NewGenerator(pgtranslate.WithDialect(pgtranslate.DialectPostgreSQL))
			defStr, err := gen.Generate(arg.Default)
			if err == nil {
				fnArg.DefaultValue = &defStr
			}
		}
		parsed.Args = append(parsed.Args, fnArg)
	}

	return parsed
}
