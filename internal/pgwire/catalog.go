package pgwire

import (
	"context"
	"regexp"
	"strings"

	wire "github.com/jeroenrinzema/psql-wire"
	"github.com/jackc/pgx/v5/pgtype"
)

// catalogHandler returns a handler for PostgreSQL catalog queries.
// These queries are sent by tools like psql to get database metadata.
func (s *Server) catalogHandler(query string) wire.PreparedStatements {
	upper := strings.ToUpper(strings.TrimSpace(query))

	// version()
	if matched, _ := regexp.MatchString(`^\s*SELECT\s+VERSION\s*\(\s*\)`, upper); matched {
		return s.versionQuery()
	}

	// current_database()
	if matched, _ := regexp.MatchString(`CURRENT_DATABASE\s*\(\s*\)`, upper); matched {
		return s.currentDatabaseQuery()
	}

	// current_user / current_schema
	if matched, _ := regexp.MatchString(`CURRENT_USER|CURRENT_SCHEMA`, upper); matched {
		return s.currentUserQuery()
	}

	// pg_catalog queries - return empty results
	if strings.Contains(upper, "PG_CATALOG") {
		return s.emptyResultQuery()
	}

	// pg_* table queries
	if matched, _ := regexp.MatchString(`FROM\s+PG_`, upper); matched {
		return s.emptyResultQuery()
	}

	// information_schema queries
	if strings.Contains(upper, "INFORMATION_SCHEMA") {
		return s.informationSchemaQuery(query)
	}

	// SET statements - acknowledge but ignore
	if strings.HasPrefix(upper, "SET ") {
		return s.setQuery()
	}

	// SHOW statements
	if strings.HasPrefix(upper, "SHOW ") {
		return s.showQuery(query)
	}

	return nil
}

// versionQuery returns the sblite version in PostgreSQL format.
func (s *Server) versionQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				return writer.Row([]interface{}{"sblite 1.0.0, compatible with PostgreSQL 15.0"})
			},
			wire.WithColumns(wire.Columns{
				{Name: "version", Oid: pgtype.TextOID},
			}),
		),
	)
}

// currentDatabaseQuery returns the current database name.
func (s *Server) currentDatabaseQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				return writer.Row([]interface{}{"sblite"})
			},
			wire.WithColumns(wire.Columns{
				{Name: "current_database", Oid: pgtype.TextOID},
			}),
		),
	)
}

// currentUserQuery returns the current user.
func (s *Server) currentUserQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				return writer.Row([]interface{}{"sblite"})
			},
			wire.WithColumns(wire.Columns{
				{Name: "current_user", Oid: pgtype.TextOID},
			}),
		),
	)
}

// emptyResultQuery returns an empty result set.
func (s *Server) emptyResultQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				// Return no rows
				return nil
			},
		),
	)
}

// setQuery handles SET statements.
func (s *Server) setQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				// SET statements are acknowledged but don't return results
				return nil
			},
		),
	)
}

// showQuery handles SHOW statements.
func (s *Server) showQuery(query string) wire.PreparedStatements {
	upper := strings.ToUpper(query)

	// Common SHOW queries
	value := "unknown"
	name := "setting"

	if strings.Contains(upper, "SERVER_VERSION") {
		name = "server_version"
		value = "15.0"
	} else if strings.Contains(upper, "SERVER_ENCODING") {
		name = "server_encoding"
		value = "UTF8"
	} else if strings.Contains(upper, "CLIENT_ENCODING") {
		name = "client_encoding"
		value = "UTF8"
	} else if strings.Contains(upper, "STANDARD_CONFORMING_STRINGS") {
		name = "standard_conforming_strings"
		value = "on"
	} else if strings.Contains(upper, "DATESTYLE") {
		name = "DateStyle"
		value = "ISO, MDY"
	} else if strings.Contains(upper, "TIMEZONE") {
		name = "TimeZone"
		value = "UTC"
	} else if strings.Contains(upper, "TRANSACTION ISOLATION LEVEL") {
		name = "transaction_isolation"
		value = "serializable"
	}

	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				return writer.Row([]interface{}{value})
			},
			wire.WithColumns(wire.Columns{
				{Name: name, Oid: pgtype.TextOID},
			}),
		),
	)
}

// informationSchemaQuery handles information_schema queries.
func (s *Server) informationSchemaQuery(query string) wire.PreparedStatements {
	upper := strings.ToUpper(query)

	// information_schema.tables - map to sqlite_master
	if strings.Contains(upper, "INFORMATION_SCHEMA.TABLES") {
		return s.tablesQuery()
	}

	// Return empty for other information_schema queries
	return s.emptyResultQuery()
}

// tablesQuery returns table information from sqlite_master.
func (s *Server) tablesQuery() wire.PreparedStatements {
	return wire.Prepared(
		wire.NewStatement(
			func(ctx context.Context, writer wire.DataWriter, params []wire.Parameter) error {
				rows, err := s.db.QueryContext(ctx, `
					SELECT
						'public' as table_schema,
						name as table_name,
						CASE type WHEN 'table' THEN 'BASE TABLE' ELSE 'VIEW' END as table_type
					FROM sqlite_master
					WHERE type IN ('table', 'view')
					AND name NOT LIKE 'sqlite_%'
					AND name NOT LIKE '_%'
					ORDER BY name
				`)
				if err != nil {
					return err
				}
				defer rows.Close()

				for rows.Next() {
					var schema, name, tableType string
					if err := rows.Scan(&schema, &name, &tableType); err != nil {
						return err
					}
					if err := writer.Row([]interface{}{schema, name, tableType}); err != nil {
						return err
					}
				}
				return rows.Err()
			},
			wire.WithColumns(wire.Columns{
				{Name: "table_schema", Oid: pgtype.TextOID},
				{Name: "table_name", Oid: pgtype.TextOID},
				{Name: "table_type", Oid: pgtype.TextOID},
			}),
		),
	)
}
