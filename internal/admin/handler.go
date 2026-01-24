// Package admin provides HTTP handlers for administrative table management.
// It allows creating, listing, retrieving, and deleting tables with typed schemas.
package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/fts"
	"github.com/markb/sblite/internal/schema"
	"github.com/markb/sblite/internal/types"
)

// reservedTableNames is a set of table names that cannot be created by users.
var reservedTableNames = map[string]bool{
	"auth_users":          true,
	"auth_sessions":       true,
	"auth_refresh_tokens": true,
	"_columns":            true,
	"_rls_policies":       true,
}

// reservedTablePrefixes are prefixes that cannot be used for table names.
var reservedTablePrefixes = []string{"auth_", "_", "sqlite_"}

// validColumnNameRegex matches valid column names: starts with letter or underscore,
// contains only alphanumeric and underscore characters.
var validColumnNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ColumnDef defines a column in a table creation request.
type ColumnDef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

// CreateTableRequest is the request body for creating a table.
type CreateTableRequest struct {
	Name    string      `json:"name"`
	Columns []ColumnDef `json:"columns"`
}

// TableInfo represents table information in responses.
type TableInfo struct {
	Name    string      `json:"name"`
	Columns []ColumnDef `json:"columns"`
}

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CreateFTSIndexRequest is the request body for creating an FTS index.
type CreateFTSIndexRequest struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Tokenizer string   `json:"tokenizer,omitempty"`
}

// FTSIndexInfo represents FTS index information in responses.
type FTSIndexInfo struct {
	TableName string   `json:"table_name"`
	IndexName string   `json:"index_name"`
	Columns   []string `json:"columns"`
	Tokenizer string   `json:"tokenizer"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// Handler handles admin API requests.
type Handler struct {
	db     *db.DB
	schema *schema.Schema
	fts    *fts.Manager
}

// NewHandler creates a new admin Handler.
func NewHandler(database *db.DB, sch *schema.Schema) *Handler {
	return &Handler{
		db:     database,
		schema: sch,
		fts:    fts.NewManager(database.DB),
	}
}

// GetFTSManager returns the FTS manager for use by other handlers.
func (h *Handler) GetFTSManager() *fts.Manager {
	return h.fts
}

// isReservedTableName checks if a table name is reserved or uses a reserved prefix.
func isReservedTableName(name string) (bool, string) {
	// Check exact reserved names
	if reservedTableNames[name] {
		return true, fmt.Sprintf("Table name '%s' is reserved", name)
	}

	// Check reserved prefixes
	for _, prefix := range reservedTablePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true, fmt.Sprintf("Table names starting with '%s' are reserved", prefix)
		}
	}

	return false, ""
}

// validateColumnName checks if a column name is valid.
func validateColumnName(name string) (bool, string) {
	// Must not be empty
	if name == "" {
		return false, "Column name is required"
	}

	// Must not be SQLite reserved
	if strings.EqualFold(name, "rowid") {
		return false, "Column name 'rowid' is reserved by SQLite"
	}

	// Must match valid identifier pattern (starts with letter/underscore, alphanumeric+underscore)
	if !validColumnNameRegex.MatchString(name) {
		return false, fmt.Sprintf("Column name '%s' is invalid: must start with a letter or underscore and contain only alphanumeric characters and underscores", name)
	}

	return true, ""
}

// CreateTable handles POST /admin/v1/tables.
// It creates a new SQLite table and registers column metadata.
func (h *Handler) CreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate table name
	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name is required")
		return
	}

	// Check for reserved table names
	if reserved, msg := isReservedTableName(req.Name); reserved {
		h.writeError(w, http.StatusBadRequest, "validation_failed", msg)
		return
	}

	// Validate columns
	if len(req.Columns) == 0 {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "At least one column is required")
		return
	}

	// Validate all column names and types before doing anything
	for _, col := range req.Columns {
		if valid, msg := validateColumnName(col.Name); !valid {
			h.writeError(w, http.StatusBadRequest, "validation_failed", msg)
			return
		}
		if !types.IsValidType(col.Type) {
			h.writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("Invalid column type: %s", col.Type))
			return
		}
	}

	// Build CREATE TABLE SQL
	createSQL := h.buildCreateTableSQL(req)

	// Execute in transaction
	tx, err := h.db.Begin()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Create the table
	if _, err := tx.Exec(createSQL); err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", fmt.Sprintf("Failed to create table: %v", err))
		return
	}

	// Register column metadata within the same transaction
	for _, col := range req.Columns {
		schemCol := schema.Column{
			TableName:    req.Name,
			ColumnName:   col.Name,
			PgType:       col.Type,
			IsNullable:   col.Nullable,
			DefaultValue: col.Default,
			IsPrimary:    col.Primary,
		}
		if err := h.schema.RegisterColumnTx(tx, schemCol); err != nil {
			h.writeError(w, http.StatusInternalServerError, "server_error", fmt.Sprintf("Failed to register column metadata: %v", err))
			return
		}
	}

	// Commit the transaction (both table creation and metadata registration)
	if err := tx.Commit(); err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to commit transaction")
		return
	}

	// Build response
	response := TableInfo{
		Name:    req.Name,
		Columns: req.Columns,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ListTables handles GET /admin/v1/tables.
// It returns all tables with their column definitions.
func (h *Handler) ListTables(w http.ResponseWriter, r *http.Request) {
	// Get all table names from schema
	tableNames, err := h.schema.ListTables()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to list tables")
		return
	}

	tables := make([]TableInfo, 0, len(tableNames))

	for _, tableName := range tableNames {
		columns, err := h.schema.GetColumns(tableName)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "server_error", fmt.Sprintf("Failed to get columns for table %s", tableName))
			return
		}

		colDefs := make([]ColumnDef, 0, len(columns))
		for _, col := range columns {
			colDefs = append(colDefs, ColumnDef{
				Name:     col.ColumnName,
				Type:     col.PgType,
				Nullable: col.IsNullable,
				Default:  col.DefaultValue,
				Primary:  col.IsPrimary,
			})
		}

		tables = append(tables, TableInfo{
			Name:    tableName,
			Columns: colDefs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

// GetTable handles GET /admin/v1/tables/{name}.
// It returns a single table with its column definitions.
func (h *Handler) GetTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name is required")
		return
	}

	columns, err := h.schema.GetColumns(tableName)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to get columns")
		return
	}

	if len(columns) == 0 {
		h.writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Table %s not found", tableName))
		return
	}

	colDefs := make([]ColumnDef, 0, len(columns))
	for _, col := range columns {
		colDefs = append(colDefs, ColumnDef{
			Name:     col.ColumnName,
			Type:     col.PgType,
			Nullable: col.IsNullable,
			Default:  col.DefaultValue,
			Primary:  col.IsPrimary,
		})
	}

	response := TableInfo{
		Name:    tableName,
		Columns: colDefs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteTable handles DELETE /admin/v1/tables/{name}.
// It drops the SQLite table and removes column metadata.
func (h *Handler) DeleteTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name is required")
		return
	}

	// Execute in transaction
	tx, err := h.db.Begin()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Drop the table
	dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", sanitizeIdentifier(tableName))
	if _, err := tx.Exec(dropSQL); err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", fmt.Sprintf("Failed to drop table: %v", err))
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to commit transaction")
		return
	}

	// Delete metadata
	if err := h.schema.DeleteTableColumns(tableName); err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Failed to delete column metadata")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// buildCreateTableSQL builds the CREATE TABLE SQL statement.
func (h *Handler) buildCreateTableSQL(req CreateTableRequest) string {
	var columns []string
	var primaryKeys []string

	for _, col := range req.Columns {
		colSQL := fmt.Sprintf("%s %s", sanitizeIdentifier(col.Name), pgTypeToSQLite(col.Type))

		if !col.Nullable {
			colSQL += " NOT NULL"
		}

		if col.Default != "" {
			colSQL += fmt.Sprintf(" DEFAULT %s", mapDefaultValue(col.Default, col.Type))
		}

		columns = append(columns, colSQL)

		if col.Primary {
			primaryKeys = append(primaryKeys, sanitizeIdentifier(col.Name))
		}
	}

	// Add PRIMARY KEY constraint if there are primary keys
	if len(primaryKeys) > 0 {
		columns = append(columns, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	return fmt.Sprintf("CREATE TABLE %s (\n\t%s\n)", sanitizeIdentifier(req.Name), strings.Join(columns, ",\n\t"))
}

// writeError writes a JSON error response.
func (h *Handler) writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

// pgTypeToSQLite maps PostgreSQL types to SQLite types.
func pgTypeToSQLite(pgType string) string {
	switch pgType {
	case "uuid", "text", "numeric", "timestamptz", "jsonb":
		return "TEXT"
	case "integer", "boolean":
		return "INTEGER"
	case "bytea":
		return "BLOB"
	default:
		// Handle vector types (e.g., "vector(1536)")
		if types.IsVectorType(pgType) {
			return "TEXT" // Vectors stored as JSON arrays in TEXT
		}
		return "TEXT"
	}
}

// mapDefaultValue maps PostgreSQL default values to SQLite equivalents.
func mapDefaultValue(defaultVal, pgType string) string {
	// Handle common PostgreSQL functions
	switch defaultVal {
	case "gen_random_uuid()":
		// SQLite UUID generation expression that produces valid UUID v4 format
		return "(lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))"
	case "now()":
		// PostgreSQL-compatible timestamptz format with milliseconds and UTC offset
		return "(strftime('%Y-%m-%d %H:%M:%f+00', 'now'))"
	}

	// Handle boolean literals
	if pgType == "boolean" {
		switch strings.ToLower(defaultVal) {
		case "true":
			return "1"
		case "false":
			return "0"
		}
	}

	// Return as-is for other values
	return defaultVal
}

// sanitizeIdentifier wraps an identifier in double quotes to prevent SQL injection.
// This is a simple implementation - in production you might want more robust handling.
func sanitizeIdentifier(name string) string {
	// Remove any existing quotes and double-quote for safety
	name = strings.ReplaceAll(name, "\"", "")
	return "\"" + name + "\""
}

// CreateFTSIndex handles POST /admin/v1/tables/{name}/fts.
// It creates a new FTS index on the specified columns.
func (h *Handler) CreateFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name is required")
		return
	}

	var req CreateFTSIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate index name
	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Index name is required")
		return
	}

	// Validate columns
	if len(req.Columns) == 0 {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "At least one column is required")
		return
	}

	// Create the FTS index
	if err := h.fts.CreateIndex(tableName, req.Name, req.Columns, req.Tokenizer); err != nil {
		h.writeError(w, http.StatusBadRequest, "fts_error", err.Error())
		return
	}

	// Get the created index for response
	idx, err := h.fts.GetIndex(tableName, req.Name)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Index created but failed to retrieve details")
		return
	}

	response := FTSIndexInfo{
		TableName: idx.TableName,
		IndexName: idx.IndexName,
		Columns:   idx.Columns,
		Tokenizer: idx.Tokenizer,
		CreatedAt: idx.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ListFTSIndexes handles GET /admin/v1/tables/{name}/fts.
// It returns all FTS indexes for a table.
func (h *Handler) ListFTSIndexes(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name is required")
		return
	}

	indexes, err := h.fts.ListIndexes(tableName)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	response := make([]FTSIndexInfo, len(indexes))
	for i, idx := range indexes {
		response[i] = FTSIndexInfo{
			TableName: idx.TableName,
			IndexName: idx.IndexName,
			Columns:   idx.Columns,
			Tokenizer: idx.Tokenizer,
			CreatedAt: idx.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetFTSIndex handles GET /admin/v1/tables/{name}/fts/{index}.
// It returns a single FTS index.
func (h *Handler) GetFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name and index name are required")
		return
	}

	idx, err := h.fts.GetIndex(tableName, indexName)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	response := FTSIndexInfo{
		TableName: idx.TableName,
		IndexName: idx.IndexName,
		Columns:   idx.Columns,
		Tokenizer: idx.Tokenizer,
		CreatedAt: idx.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteFTSIndex handles DELETE /admin/v1/tables/{name}/fts/{index}.
// It removes an FTS index and its triggers.
func (h *Handler) DeleteFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name and index name are required")
		return
	}

	if err := h.fts.DropIndex(tableName, indexName); err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RebuildFTSIndex handles POST /admin/v1/tables/{name}/fts/{index}/rebuild.
// It rebuilds an FTS index from scratch.
func (h *Handler) RebuildFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		h.writeError(w, http.StatusBadRequest, "validation_failed", "Table name and index name are required")
		return
	}

	if err := h.fts.RebuildIndex(tableName, indexName); err != nil {
		h.writeError(w, http.StatusInternalServerError, "rebuild_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rebuilt"})
}
