// Package pgwire provides a PostgreSQL wire protocol server for sblite.
// This allows PostgreSQL clients (psql, pgAdmin, DBeaver) to connect to sblite.
package pgwire

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	wire "github.com/jeroenrinzema/psql-wire"

	"github.com/markb/sblite/internal/pgtranslate"
)

// Config holds the pgwire server configuration.
type Config struct {
	Address  string // TCP address to listen on (e.g., ":5432")
	Password string // Password for authentication (empty = no auth)
	NoAuth   bool   // Disable authentication entirely
	Logger   *slog.Logger
}

// Server implements a PostgreSQL wire protocol server.
type Server struct {
	db         *sql.DB
	config     Config
	server     *wire.Server
	translator *pgtranslate.ASTTranslator
}

// NewServer creates a new PostgreSQL wire protocol server.
func NewServer(db *sql.DB, cfg Config) (*Server, error) {
	s := &Server{
		db:         db,
		config:     cfg,
		translator: pgtranslate.NewASTTranslator(pgtranslate.DialectSQLite),
	}

	// Build server options
	opts := []wire.OptionFn{
		wire.Version("sblite 1.0.0 (PostgreSQL compatible)"),
		wire.GlobalParameters(wire.Parameters{
			wire.ParamServerEncoding: "UTF8",
			wire.ParamServerVersion:  "15.0",
			"DateStyle":              "ISO, MDY",
			"TimeZone":               "UTC",
		}),
	}

	if cfg.Logger != nil {
		opts = append(opts, wire.Logger(cfg.Logger))
	}

	// Configure authentication
	if !cfg.NoAuth && cfg.Password != "" {
		opts = append(opts, wire.SessionAuthStrategy(
			wire.ClearTextPassword(s.passwordAuth),
		))
	}

	// Create the wire server
	server, err := wire.NewServer(s.handleQuery, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgwire server: %w", err)
	}
	s.server = server

	return s, nil
}

// passwordAuth validates the provided password.
func (s *Server) passwordAuth(ctx context.Context, database, username, password string) (context.Context, bool, error) {
	return ctx, password == s.config.Password, nil
}

// ListenAndServe starts the server and listens for connections.
func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}

	if s.config.Logger != nil {
		s.config.Logger.Info("pgwire server listening", "address", s.config.Address)
	}

	return s.server.Serve(listener)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Close()
}

// handleQuery parses and executes a SQL query.
func (s *Server) handleQuery(ctx context.Context, query string) (wire.PreparedStatements, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return wire.Prepared(), nil
	}

	// Handle special PostgreSQL catalog queries
	if handler := s.catalogHandler(query); handler != nil {
		return handler, nil
	}

	upperQuery := strings.ToUpper(strings.TrimSpace(query))

	// Handle CREATE TABLE specially to track UUID columns and register metadata
	if strings.HasPrefix(upperQuery, "CREATE TABLE") || strings.HasPrefix(upperQuery, "CREATE TEMP") {
		return s.handleCreateTable(ctx, query)
	}

	// Handle INSERT specially to add UUID generation
	if strings.HasPrefix(upperQuery, "INSERT") {
		return s.handleInsert(ctx, query)
	}

	// Translate PostgreSQL to SQLite
	translated := pgtranslate.TranslateToSQLite(query)

	upperTranslated := strings.ToUpper(strings.TrimSpace(translated))

	// For SELECT queries, we need to determine columns first
	if strings.HasPrefix(upperTranslated, "SELECT") ||
		strings.HasPrefix(upperTranslated, "WITH") ||
		strings.Contains(upperTranslated, "RETURNING") {
		return s.prepareSelectStatement(ctx, translated)
	}

	// For non-SELECT queries (UPDATE, DELETE, etc.)
	stmt := wire.NewStatement(func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
		return s.executeExec(ctx, translated, params)
	})

	return wire.Prepared(stmt), nil
}

// handleCreateTable handles CREATE TABLE with UUID column tracking and metadata registration.
func (s *Server) handleCreateTable(ctx context.Context, query string) (wire.PreparedStatements, error) {
	// Extract table name and UUID columns before translation
	tableName := pgtranslate.GetTableName(query)
	uuidColumns := pgtranslate.GetUUIDColumns(query)

	// Translate PostgreSQL to SQLite
	translated := pgtranslate.TranslateToSQLite(query)

	stmt := wire.NewStatement(func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
		// Execute the CREATE TABLE
		if err := s.executeExec(ctx, translated, params); err != nil {
			return err
		}

		// Register the table in _columns metadata
		if tableName != "" {
			if err := s.registerTableMetadata(ctx, tableName, uuidColumns); err != nil {
				// Log but don't fail - table was created successfully
				if s.config.Logger != nil {
					s.config.Logger.Warn("failed to register table metadata", "table", tableName, "error", err)
				}
			}
		}

		return nil
	})

	return wire.Prepared(stmt), nil
}

// handleInsert handles INSERT with UUID generation for columns with gen_random_uuid() defaults.
func (s *Server) handleInsert(ctx context.Context, query string) (wire.PreparedStatements, error) {
	// Translate PostgreSQL to SQLite
	translated := pgtranslate.TranslateToSQLite(query)

	// Rewrite INSERT to add UUID generation for missing UUID columns
	translated = s.rewriteInsertWithUUIDs(translated)

	// Check if this INSERT has RETURNING clause
	upperTranslated := strings.ToUpper(translated)
	if strings.Contains(upperTranslated, "RETURNING") {
		return s.prepareSelectStatement(ctx, translated)
	}

	stmt := wire.NewStatement(func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
		return s.executeExec(ctx, translated, params)
	})

	return wire.Prepared(stmt), nil
}

// registerTableMetadata registers a table's columns in the _columns metadata table.
func (s *Server) registerTableMetadata(ctx context.Context, tableName string, uuidColumns []string) error {
	// Get column info from PRAGMA
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	// Create a set of UUID columns for quick lookup
	uuidColSet := make(map[string]bool)
	for _, col := range uuidColumns {
		uuidColSet[strings.ToLower(col)] = true
	}

	// Get existing columns for this table from _columns
	existingCols := make(map[string]bool)
	existingRows, err := s.db.QueryContext(ctx, `SELECT column_name FROM _columns WHERE table_name = ?`, tableName)
	if err == nil {
		defer existingRows.Close()
		for existingRows.Next() {
			var colName string
			if err := existingRows.Scan(&colName); err == nil {
				existingCols[colName] = true
			}
		}
	}

	// Process each column from PRAGMA
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}

		// Skip if already registered
		if existingCols[name] {
			continue
		}

		// Infer PostgreSQL type from SQLite type
		pgType := inferPgType(colType)

		// Check if this is a UUID column
		defaultValue := ""
		if uuidColSet[strings.ToLower(name)] {
			pgType = "uuid"
			defaultValue = "gen_random_uuid()"
		}

		// Insert into _columns
		_, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary)
			VALUES (?, ?, ?, ?, ?, ?)
		`, tableName, name, pgType, notNull == 0, defaultValue, pk == 1)
		if err != nil {
			return fmt.Errorf("failed to insert column metadata: %w", err)
		}
	}

	return rows.Err()
}

// inferPgType infers a PostgreSQL type from a SQLite type.
func inferPgType(sqliteType string) string {
	upper := strings.ToUpper(sqliteType)
	switch {
	case strings.Contains(upper, "INT"):
		return "integer"
	case strings.Contains(upper, "CHAR"), strings.Contains(upper, "CLOB"), strings.Contains(upper, "TEXT"):
		return "text"
	case strings.Contains(upper, "BLOB"):
		return "bytea"
	case strings.Contains(upper, "REAL"), strings.Contains(upper, "FLOA"), strings.Contains(upper, "DOUB"):
		return "numeric"
	case strings.Contains(upper, "BOOL"):
		return "boolean"
	default:
		return "text"
	}
}

// rewriteInsertWithUUIDs modifies INSERT statements to add UUID generation for columns
// that have gen_random_uuid() as their default value.
func (s *Server) rewriteInsertWithUUIDs(query string) string {
	// Extract table name from INSERT statement
	insertTablePattern := regexp.MustCompile(`(?i)INSERT\s+(?:OR\s+\w+\s+)?INTO\s+"?(\w+)"?`)
	match := insertTablePattern.FindStringSubmatch(query)
	if match == nil {
		return query
	}
	tableName := match[1]

	// Get columns with gen_random_uuid() default from _columns
	rows, err := s.db.Query(`
		SELECT column_name FROM _columns
		WHERE table_name = ? AND default_value = 'gen_random_uuid()'
	`, tableName)
	if err != nil {
		return query
	}
	defer rows.Close()

	var uuidCols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err == nil {
			uuidCols = append(uuidCols, col)
		}
	}

	if len(uuidCols) == 0 {
		return query
	}

	// Check if the INSERT specifies column names
	// Pattern: INSERT INTO table (col1, col2, ...) VALUES (...)
	columnsPattern := regexp.MustCompile(`(?i)INSERT\s+(?:OR\s+\w+\s+)?INTO\s+"?\w+"?\s*\(([^)]+)\)\s*VALUES`)
	colMatch := columnsPattern.FindStringSubmatch(query)

	if colMatch != nil {
		// INSERT has explicit columns - check if UUID columns are missing
		specifiedCols := strings.ToLower(colMatch[1])
		var missingUUIDCols []string
		for _, uuidCol := range uuidCols {
			if !strings.Contains(specifiedCols, strings.ToLower(uuidCol)) {
				missingUUIDCols = append(missingUUIDCols, uuidCol)
			}
		}

		if len(missingUUIDCols) > 0 {
			// Add missing UUID columns to the INSERT
			return s.addUUIDColumnsToInsert(query, missingUUIDCols)
		}
	}

	return query
}

// addUUIDColumnsToInsert adds UUID columns to an INSERT statement.
func (s *Server) addUUIDColumnsToInsert(query string, uuidCols []string) string {
	// UUID v4 generation expression for SQLite
	uuidExpr := `(lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))))`

	// Find position of ) VALUES to add columns
	queryUpper := strings.ToUpper(query)
	valuesIdx := strings.Index(queryUpper, ") VALUES")
	if valuesIdx == -1 {
		return query
	}

	// Add UUID column names before ) VALUES
	colAddition := ""
	for _, col := range uuidCols {
		colAddition += ", " + col
	}
	query = query[:valuesIdx] + colAddition + query[valuesIdx:]

	// Update valuesIdx after modification
	valuesIdx += len(colAddition)

	// Find all VALUES tuples and add UUID expressions
	// Pattern to match each tuple: (val1, val2, ...)
	result := query[:valuesIdx+8] // Include ") VALUES"

	remaining := query[valuesIdx+8:]
	tuplePattern := regexp.MustCompile(`\(([^)]*)\)`)
	matches := tuplePattern.FindAllStringSubmatchIndex(remaining, -1)

	lastEnd := 0
	for _, match := range matches {
		// match[0] is start of full match, match[1] is end
		// match[2] is start of group, match[3] is end of group
		start := match[0]
		end := match[1]

		// Add any text before this tuple
		result += remaining[lastEnd:start]

		// Add the tuple with UUID expressions
		tupleContent := remaining[match[2]:match[3]]
		uuidAdditions := ""
		for range uuidCols {
			uuidAdditions += ", " + uuidExpr
		}
		result += "(" + tupleContent + uuidAdditions + ")"

		lastEnd = end
	}

	// Add any remaining text
	result += remaining[lastEnd:]

	return result
}

// prepareSelectStatement prepares a SELECT statement with proper column definitions.
func (s *Server) prepareSelectStatement(ctx context.Context, query string) (wire.PreparedStatements, error) {
	// Get column metadata by executing the query
	// We wrap in a subquery with LIMIT 0 to avoid fetching data
	metaQuery := fmt.Sprintf("SELECT * FROM (%s) AS _meta LIMIT 0", query)
	rows, err := s.db.QueryContext(ctx, metaQuery)
	if err != nil {
		// Fallback: try original query if subquery fails
		rows, err = s.db.QueryContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}
	rows.Close()

	// Build wire.Columns from the column types
	columns := make(wire.Columns, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = wire.Column{
			Table: 0,
			Name:  ct.Name(),
			Oid:   GetOID(ct.DatabaseTypeName()),
			Width: -1, // Variable width
		}
	}

	// Create the statement with columns
	stmt := wire.NewStatement(
		func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
			return s.executeSelect(ctx, query, writer, params)
		},
		wire.WithColumns(columns),
	)

	return wire.Prepared(stmt), nil
}

// executeQuery executes a query against SQLite and writes results.
func (s *Server) executeQuery(ctx context.Context, query string, writer wire.DataWriter, params []wire.Parameter) error {
	upperQuery := strings.ToUpper(strings.TrimSpace(query))

	// Check if this is a SELECT or other query that returns rows
	if strings.HasPrefix(upperQuery, "SELECT") ||
		strings.HasPrefix(upperQuery, "WITH") ||
		strings.Contains(upperQuery, "RETURNING") {
		return s.executeSelect(ctx, query, writer, params)
	}

	// For non-SELECT queries (INSERT, UPDATE, DELETE, CREATE, etc.)
	return s.executeExec(ctx, query, params)
}

// executeSelect executes a SELECT query and writes results.
func (s *Server) executeSelect(ctx context.Context, query string, writer wire.DataWriter, params []wire.Parameter) error {
	// Convert parameters
	args := make([]interface{}, len(params))
	for i, p := range params {
		args[i] = p.Value
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	// Prepare values slice
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Write rows
	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		row := make([]interface{}, len(cols))
		for i, v := range values {
			row[i] = v
		}

		if err := writer.Row(row); err != nil {
			return err
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Complete the result set
	return writer.Complete(fmt.Sprintf("SELECT %d", rowCount))
}

// executeExec executes a non-SELECT query.
func (s *Server) executeExec(ctx context.Context, query string, params []wire.Parameter) error {
	args := make([]interface{}, len(params))
	for i, p := range params {
		args[i] = p.Value
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}
