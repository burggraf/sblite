// Package pgwire provides a PostgreSQL wire protocol server for sblite.
// This allows PostgreSQL clients (psql, pgAdmin, DBeaver) to connect to sblite.
package pgwire

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
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

	// Translate PostgreSQL to SQLite
	translated := pgtranslate.TranslateToSQLite(query)

	upperQuery := strings.ToUpper(strings.TrimSpace(translated))

	// For SELECT queries, we need to determine columns first
	if strings.HasPrefix(upperQuery, "SELECT") ||
		strings.HasPrefix(upperQuery, "WITH") ||
		strings.Contains(upperQuery, "RETURNING") {
		return s.prepareSelectStatement(ctx, translated)
	}

	// For non-SELECT queries (INSERT, UPDATE, DELETE, CREATE, etc.)
	stmt := wire.NewStatement(func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
		return s.executeExec(ctx, translated, params)
	})

	return wire.Prepared(stmt), nil
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
