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

	// Create a prepared statement that will execute the translated query
	stmt := wire.NewStatement(func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
		return s.executeQuery(ctx, translated, writer, params)
	})

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

	// First, define columns if the writer supports it
	// The columns are already defined by the prepared statement

	// Prepare values slice
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Write rows
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
	}

	return rows.Err()
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
