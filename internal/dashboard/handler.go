package dashboard

import (
	"bufio"
	"database/sql"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

//go:embed static/*
var staticFS embed.FS

const sessionCookieName = "_sblite_session"

// Handler serves the dashboard UI and API.
type Handler struct {
	db            *sql.DB
	store         *Store
	auth          *Auth
	sessions      *SessionManager
	migrationsDir string
	startTime     time.Time
	serverConfig  *ServerConfig
}

// ServerConfig holds server configuration for display in settings.
type ServerConfig struct {
	Version  string
	Host     string
	Port     int
	DBPath   string
	LogMode  string
	LogFile  string
	LogDB    string
}

// NewHandler creates a new Handler.
func NewHandler(db *sql.DB, migrationsDir string) *Handler {
	store := NewStore(db)
	return &Handler{
		db:            db,
		store:         store,
		auth:          NewAuth(store),
		sessions:      NewSessionManager(store),
		migrationsDir: migrationsDir,
		startTime:     time.Now(),
		serverConfig:  &ServerConfig{Version: "0.1.0"},
	}
}

// SetServerConfig sets the server configuration for display.
func (h *Handler) SetServerConfig(cfg *ServerConfig) {
	h.serverConfig = cfg
}

// RegisterRoutes registers the dashboard routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/auth/status", h.handleAuthStatus)
		r.Post("/auth/setup", h.handleSetup)
		r.Post("/auth/login", h.handleLogin)
		r.Post("/auth/logout", h.handleLogout)

		// Table management API routes (require auth)
		r.Route("/tables", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListTables)
			r.Post("/", h.handleCreateTable)
			r.Get("/{name}", h.handleGetTableSchema)
			r.Delete("/{name}", h.handleDeleteTable)
			r.Post("/{name}/columns", h.handleAddColumn)
			r.Patch("/{name}/columns/{column}", h.handleRenameColumn)
			r.Delete("/{name}/columns/{column}", h.handleDropColumn)
		})

		// Data API routes (require auth)
		r.Route("/data", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/{table}", h.handleSelectData)
			r.Post("/{table}", h.handleInsertData)
			r.Patch("/{table}", h.handleUpdateData)
			r.Delete("/{table}", h.handleDeleteData)
		})

		// Users API routes (require auth)
		r.Route("/users", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListUsers)
			r.Post("/", h.handleCreateUser)
			r.Post("/invite", h.handleInviteUser)
			r.Get("/{id}", h.handleGetUser)
			r.Patch("/{id}", h.handleUpdateUser)
			r.Delete("/{id}", h.handleDeleteUser)
		})

		// RLS Policies API routes (require auth)
		r.Route("/policies", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListPolicies)
			r.Post("/", h.handleCreatePolicy)
			r.Post("/test", h.handleTestPolicy)
			r.Get("/{id}", h.handleGetPolicy)
			r.Patch("/{id}", h.handleUpdatePolicy)
			r.Delete("/{id}", h.handleDeletePolicy)
		})

		// RLS table state routes (nested under tables)
		r.Route("/tables/{name}/rls", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleGetTableRLS)
			r.Patch("/", h.handleSetTableRLS)
		})

		// Settings API routes (require auth)
		r.Route("/settings", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/server", h.handleGetServerInfo)
			r.Get("/auth", h.handleGetAuthSettings)
			r.Post("/auth/regenerate-secret", h.handleRegenerateSecret)
			r.Get("/templates", h.handleListTemplates)
			r.Patch("/templates/{type}", h.handleUpdateTemplate)
			r.Post("/templates/{type}/reset", h.handleResetTemplate)
		})

		// Export API routes (require auth)
		r.Route("/export", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/schema", h.handleExportSchema)
			r.Get("/data", h.handleExportData)
			r.Get("/backup", h.handleExportBackup)
		})

		// Logs API routes (require auth)
		r.Route("/logs", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleQueryLogs)
			r.Get("/config", h.handleGetLogConfig)
			r.Get("/tail", h.handleTailLogs)
		})
	})

	// Static files - use Route group to ensure priority
	r.Route("/static", func(r chi.Router) {
		r.Get("/*", h.handleStatic)
	})

	// SPA - serve index.html for root and use NotFound for other routes
	r.Get("/", h.handleIndex)
	r.NotFound(h.handleIndex)
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func (h *Handler) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Get the file path from chi wildcard parameter
	path := chi.URLParam(r, "*")

	content, err := staticFS.ReadFile("static/" + path)
	if err != nil {
		if _, ok := err.(*fs.PathError); ok {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set content type based on extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false

	// Check session cookie
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		authenticated = h.sessions.Validate(cookie.Value)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"needs_setup":   h.auth.NeedsSetup(),
		"authenticated": authenticated,
	})
}

func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if err := h.auth.SetupPassword(req.Password); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Create session
	token, err := h.sessions.Create()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if !h.auth.VerifyPassword(req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
		return
	}

	// Create session
	token, err := h.sessions.Create()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Destroy()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/_/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // Delete cookie
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// requireAuth middleware checks for valid session cookie
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" || !h.sessions.Validate(cookie.Value) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleListTables(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT DISTINCT table_name FROM _columns ORDER BY table_name`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list tables"})
		return
	}
	defer rows.Close()

	var tables []map[string]interface{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, map[string]interface{}{"name": name})
	}

	if tables == nil {
		tables = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func (h *Handler) handleGetTableSchema(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	rows, err := h.db.Query(`SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns WHERE table_name = ? ORDER BY column_name`, tableName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get schema"})
		return
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var name, pgType string
		var nullable, primary bool
		var defaultVal sql.NullString
		if err := rows.Scan(&name, &pgType, &nullable, &defaultVal, &primary); err != nil {
			continue
		}
		col := map[string]interface{}{
			"name":     name,
			"type":     pgType,
			"nullable": nullable,
			"primary":  primary,
		}
		if defaultVal.Valid {
			col["default"] = defaultVal.String
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    tableName,
		"columns": columns,
	})
}

// CreateTableRequest defines the request body for creating a table
type CreateTableRequest struct {
	Name    string `json:"name"`
	Columns []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default,omitempty"`
		Primary  bool   `json:"primary"`
	} `json:"columns"`
}

func (h *Handler) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" || len(req.Columns) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and columns required"})
		return
	}

	// Build CREATE TABLE SQL
	var colDefs []string
	var primaryKeys []string
	for _, col := range req.Columns {
		sqlType := pgTypeToSQLite(col.Type)
		def := fmt.Sprintf(`"%s" %s`, col.Name, sqlType)
		if !col.Nullable {
			def += " NOT NULL"
		}
		if col.Default != "" {
			def += " DEFAULT " + col.Default
		}
		colDefs = append(colDefs, def)
		if col.Primary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, col.Name))
		}
	}
	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}

	createSQL := fmt.Sprintf(`CREATE TABLE "%s" (%s)`, req.Name, strings.Join(colDefs, ", "))

	tx, err := h.db.Begin()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(createSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Register columns in metadata
	for _, col := range req.Columns {
		_, err := tx.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary) VALUES (?, ?, ?, ?, ?, ?)`,
			req.Name, col.Name, col.Type, col.Nullable, col.Default, col.Primary)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register column"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	// Write migration file
	migrationName := fmt.Sprintf("create_%s_table", req.Name)
	if err := h.writeMigration(migrationName, createSQL+";"); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table created but failed to write migration: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"name": req.Name, "columns": req.Columns})
}

func pgTypeToSQLite(pgType string) string {
	switch pgType {
	case "integer", "boolean":
		return "INTEGER"
	case "bytea":
		return "BLOB"
	default:
		return "TEXT"
	}
}

// writeMigration creates a migration file and records it in _schema_migrations.
func (h *Handler) writeMigration(name string, sql string) error {
	// Ensure migrations directory exists (auto-create if needed)
	if err := os.MkdirAll(h.migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate version timestamp
	version := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.sql", version, name)

	// Write migration file
	path := filepath.Join(h.migrationsDir, filename)
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	// Record in _schema_migrations
	_, err := h.db.Exec(`INSERT INTO _schema_migrations (version, name) VALUES (?, ?)`, version, name)
	if err != nil {
		// Clean up the file if we can't record the migration
		os.Remove(path)
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

func (h *Handler) handleDeleteTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Drop the table
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Remove metadata
	if _, err := tx.Exec(`DELETE FROM _columns WHERE table_name = ?`, tableName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to remove metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	// Write migration file
	dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS "%s";`, tableName)
	migrationName := fmt.Sprintf("drop_%s_table", tableName)
	if err := h.writeMigration(migrationName, dropSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table dropped but failed to write migration: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleSelectData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	limit := 25
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Parse filters
	whereClause, whereValues := h.parseSelectFilter(r.URL.Query())

	// Parse order
	orderClause := ""
	if order := r.URL.Query().Get("order"); order != "" {
		parts := strings.Split(order, ".")
		if len(parts) >= 1 {
			col := parts[0]
			dir := "ASC"
			if len(parts) >= 2 && strings.ToLower(parts[1]) == "desc" {
				dir = "DESC"
			}
			orderClause = fmt.Sprintf(` ORDER BY "%s" %s`, col, dir)
		}
	}

	// Get total count with filters
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" %s`, tableName, whereClause)
	err := h.db.QueryRow(countQuery, whereValues...).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	// Get rows with filters and order
	query := fmt.Sprintf(`SELECT * FROM "%s" %s%s LIMIT %d OFFSET %d`, tableName, whereClause, orderClause, limit, offset)
	rows, err := h.db.Query(query, whereValues...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rows":   results,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) handleInsertData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	var columns []string
	var placeholders []string
	var values []interface{}
	for col, val := range data {
		columns = append(columns, fmt.Sprintf(`"%s"`, col))
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	query := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	if _, err := h.db.Exec(query, values...); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleUpdateData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Build SET clause
	var setClauses []string
	var values []interface{}
	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf(`"%s" = ?`, col))
		values = append(values, val)
	}

	// Parse filter from query string (simple eq filter)
	whereClause, whereValues := h.parseSimpleFilter(r.URL.Query())
	values = append(values, whereValues...)

	query := fmt.Sprintf(`UPDATE "%s" SET %s %s`, tableName, strings.Join(setClauses, ", "), whereClause)

	result, err := h.db.Exec(query, values...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"updated": affected})
}

func (h *Handler) handleDeleteData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	whereClause, whereValues := h.parseSimpleFilter(r.URL.Query())
	if whereClause == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Filter required for delete"})
		return
	}

	query := fmt.Sprintf(`DELETE FROM "%s" %s`, tableName, whereClause)

	if _, err := h.db.Exec(query, whereValues...); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) parseSimpleFilter(query url.Values) (string, []interface{}) {
	var conditions []string
	var values []interface{}

	for key, vals := range query {
		if key == "limit" || key == "offset" || key == "order" {
			continue
		}
		if len(vals) > 0 {
			val := vals[0]
			if strings.HasPrefix(val, "eq.") {
				conditions = append(conditions, fmt.Sprintf(`"%s" = ?`, key))
				values = append(values, strings.TrimPrefix(val, "eq."))
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), values
}

func (h *Handler) parseSelectFilter(query url.Values) (string, []interface{}) {
	var conditions []string
	var values []interface{}

	for key, vals := range query {
		if key == "limit" || key == "offset" || key == "order" {
			continue
		}
		// Process ALL filter values for this key (supports multiple filters on same column)
		for _, val := range vals {
			switch {
			case strings.HasPrefix(val, "eq."):
				conditions = append(conditions, fmt.Sprintf(`"%s" = ?`, key))
				values = append(values, strings.TrimPrefix(val, "eq."))
			case strings.HasPrefix(val, "neq."):
				conditions = append(conditions, fmt.Sprintf(`"%s" != ?`, key))
				values = append(values, strings.TrimPrefix(val, "neq."))
			case strings.HasPrefix(val, "gt."):
				conditions = append(conditions, fmt.Sprintf(`"%s" > ?`, key))
				values = append(values, strings.TrimPrefix(val, "gt."))
			case strings.HasPrefix(val, "gte."):
				conditions = append(conditions, fmt.Sprintf(`"%s" >= ?`, key))
				values = append(values, strings.TrimPrefix(val, "gte."))
			case strings.HasPrefix(val, "lt."):
				conditions = append(conditions, fmt.Sprintf(`"%s" < ?`, key))
				values = append(values, strings.TrimPrefix(val, "lt."))
			case strings.HasPrefix(val, "lte."):
				conditions = append(conditions, fmt.Sprintf(`"%s" <= ?`, key))
				values = append(values, strings.TrimPrefix(val, "lte."))
			case strings.HasPrefix(val, "like."):
				pattern := strings.TrimPrefix(val, "like.")
				pattern = strings.ReplaceAll(pattern, "*", "%")
				conditions = append(conditions, fmt.Sprintf(`"%s" LIKE ?`, key))
				values = append(values, pattern)
			case strings.HasPrefix(val, "ilike."):
				pattern := strings.TrimPrefix(val, "ilike.")
				pattern = strings.ReplaceAll(pattern, "*", "%")
				conditions = append(conditions, fmt.Sprintf(`"%s" LIKE ? COLLATE NOCASE`, key))
				values = append(values, pattern)
			case strings.HasPrefix(val, "is."):
				v := strings.TrimPrefix(val, "is.")
				switch v {
				case "null":
					conditions = append(conditions, fmt.Sprintf(`"%s" IS NULL`, key))
				case "true":
					conditions = append(conditions, fmt.Sprintf(`"%s" = 1`, key))
				case "false":
					conditions = append(conditions, fmt.Sprintf(`"%s" = 0`, key))
				}
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), values
}

func (h *Handler) handleAddColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")

	var col struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&col); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	sqlType := pgTypeToSQLite(col.Type)
	alterSQL := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, tableName, col.Name, sqlType)
	if col.Default != "" {
		alterSQL += " DEFAULT " + col.Default
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(alterSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_, err = tx.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary) VALUES (?, ?, ?, ?, ?, ?)`,
		tableName, col.Name, col.Type, col.Nullable, col.Default, false)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register column"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	// Write migration file
	migrationName := fmt.Sprintf("add_%s_column_to_%s", col.Name, tableName)
	if err := h.writeMigration(migrationName, alterSQL+";"); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Column added but failed to write migration: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(col)
}

func (h *Handler) handleRenameColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	oldName := chi.URLParam(r, "column")

	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "new_name required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	alterSQL := fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`, tableName, oldName, req.NewName)
	if _, err := tx.Exec(alterSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(`UPDATE _columns SET column_name = ? WHERE table_name = ? AND column_name = ?`,
		req.NewName, tableName, oldName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	// Write migration file
	migrationName := fmt.Sprintf("rename_column_%s_to_%s_in_%s", oldName, req.NewName, tableName)
	if err := h.writeMigration(migrationName, alterSQL+";"); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Column renamed but failed to write migration: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"name": req.NewName})
}

func (h *Handler) handleDropColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	columnName := chi.URLParam(r, "column")

	// Get remaining columns
	rows, err := h.db.Query(`SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns WHERE table_name = ? AND column_name != ? ORDER BY column_name`, tableName, columnName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get columns"})
		return
	}
	defer rows.Close()

	type colInfo struct {
		name, pgType      string
		nullable, primary bool
		defaultVal        sql.NullString
	}
	var remainingCols []colInfo
	var colNames []string

	for rows.Next() {
		var c colInfo
		rows.Scan(&c.name, &c.pgType, &c.nullable, &c.defaultVal, &c.primary)
		remainingCols = append(remainingCols, c)
		colNames = append(colNames, fmt.Sprintf(`"%s"`, c.name))
	}

	if len(remainingCols) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot drop last column"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Create new table without the column
	var colDefs []string
	var primaryKeys []string
	for _, c := range remainingCols {
		def := fmt.Sprintf(`"%s" %s`, c.name, pgTypeToSQLite(c.pgType))
		if !c.nullable {
			def += " NOT NULL"
		}
		if c.defaultVal.Valid {
			def += " DEFAULT " + c.defaultVal.String
		}
		colDefs = append(colDefs, def)
		if c.primary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, c.name))
		}
	}
	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}

	newTableSQL := fmt.Sprintf(`CREATE TABLE "%s_new" (%s)`, tableName, strings.Join(colDefs, ", "))
	if _, err := tx.Exec(newTableSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Copy data
	copySQL := fmt.Sprintf(`INSERT INTO "%s_new" SELECT %s FROM "%s"`, tableName, strings.Join(colNames, ", "), tableName)
	if _, err := tx.Exec(copySQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Drop old, rename new
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE "%s"`, tableName)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE "%s_new" RENAME TO "%s"`, tableName, tableName)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update metadata
	if _, err := tx.Exec(`DELETE FROM _columns WHERE table_name = ? AND column_name = ?`, tableName, columnName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	// Write migration file (use PostgreSQL-compatible syntax for Supabase migration)
	dropColumnSQL := fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN "%s";`, tableName, columnName)
	migrationName := fmt.Sprintf("drop_column_%s_from_%s", columnName, tableName)
	if err := h.writeMigration(migrationName, dropColumnSQL); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Column dropped but failed to write migration: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// User management handlers

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	limit := 25
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get total count
	var total int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM auth_users`).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to count users"})
		return
	}

	// Get users
	rows, err := h.db.Query(`
		SELECT id, email, email_confirmed_at, last_sign_in_at,
		       raw_app_meta_data, raw_user_meta_data, created_at, updated_at
		FROM auth_users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list users"})
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, email string
		var emailConfirmedAt, lastSignInAt, appMeta, userMeta, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&id, &email, &emailConfirmedAt, &lastSignInAt, &appMeta, &userMeta, &createdAt, &updatedAt); err != nil {
			continue
		}
		user := map[string]interface{}{
			"id":                 id,
			"email":              email,
			"email_confirmed_at": nullStringToInterface(emailConfirmedAt),
			"last_sign_in_at":    nullStringToInterface(lastSignInAt),
			"raw_app_meta_data":  nullStringToInterface(appMeta),
			"raw_user_meta_data": nullStringToInterface(userMeta),
			"created_at":         nullStringToInterface(createdAt),
			"updated_at":         nullStringToInterface(updatedAt),
		}
		users = append(users, user)
	}

	if users == nil {
		users = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func nullStringToInterface(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		AutoConfirm bool   `json:"auto_confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Validate email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Please enter a valid email address"})
		return
	}

	// Validate password
	if len(req.Password) < 6 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 6 characters"})
		return
	}

	// Check if user already exists
	var existingID string
	err := h.db.QueryRow("SELECT id FROM auth_users WHERE email = ?", req.Email).Scan(&existingID)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "A user with this email already exists"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	// Create user
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	var emailConfirmedAt interface{} = nil
	if req.AutoConfirm {
		emailConfirmedAt = now
	}

	_, err = h.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, ?, ?, ?, '{"provider":"email","providers":["email"]}', '{}', ?, ?)
	`, id, req.Email, string(hash), emailConfirmedAt, now, now)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                 id,
		"email":              req.Email,
		"created_at":         now,
		"email_confirmed_at": emailConfirmedAt,
	})
}

func (h *Handler) handleInviteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Validate email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Please enter a valid email address"})
		return
	}

	// Check if user already exists with confirmed email
	var existingID string
	var emailConfirmedAt sql.NullString
	err := h.db.QueryRow("SELECT id, email_confirmed_at FROM auth_users WHERE email = ?", req.Email).Scan(&existingID, &emailConfirmedAt)
	if err == nil && emailConfirmedAt.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "A user with this email already exists"})
		return
	}

	// Create invite token
	token := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	// If user exists but unconfirmed (previously invited), update the token
	// Otherwise create a new user with no password
	var userID string
	if err == nil {
		// User exists, reuse their ID
		userID = existingID
	} else {
		// No user exists, create one with no password
		userID = uuid.New().String()
		_, err = h.db.Exec(`
			INSERT INTO auth_users (id, email, encrypted_password, email_confirmed_at, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
			VALUES (?, ?, '', NULL, '{"provider":"email","providers":["email"]}', '{}', ?, ?)
		`, userID, req.Email, now.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
			return
		}
	}

	// Create invite token
	_, err = h.db.Exec(`
		INSERT INTO auth_verification_tokens (id, user_id, type, email, expires_at, created_at)
		VALUES (?, ?, 'invite', ?, ?, ?)
	`, token, userID, req.Email, expiresAt.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create invitation"})
		return
	}

	// Build invite link - get base URL from request
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	inviteLink := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=invite", baseURL, url.QueryEscape(token))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"invite_link": inviteLink,
		"email":       req.Email,
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) handleGetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID required"})
		return
	}

	var id, email string
	var emailConfirmedAt, lastSignInAt, appMeta, userMeta, createdAt, updatedAt sql.NullString
	err := h.db.QueryRow(`
		SELECT id, email, email_confirmed_at, last_sign_in_at,
		       raw_app_meta_data, raw_user_meta_data, created_at, updated_at
		FROM auth_users WHERE id = ?`, userID).Scan(
		&id, &email, &emailConfirmedAt, &lastSignInAt, &appMeta, &userMeta, &createdAt, &updatedAt)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                 id,
		"email":              email,
		"email_confirmed_at": nullStringToInterface(emailConfirmedAt),
		"last_sign_in_at":    nullStringToInterface(lastSignInAt),
		"raw_app_meta_data":  nullStringToInterface(appMeta),
		"raw_user_meta_data": nullStringToInterface(userMeta),
		"created_at":         nullStringToInterface(createdAt),
		"updated_at":         nullStringToInterface(updatedAt),
	})
}

func (h *Handler) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID required"})
		return
	}

	var req struct {
		Email          *string `json:"email,omitempty"`
		AppMetadata    *string `json:"raw_app_meta_data,omitempty"`
		UserMetadata   *string `json:"raw_user_meta_data,omitempty"`
		EmailConfirmed *bool   `json:"email_confirmed,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Build update query
	var setClauses []string
	var values []interface{}

	if req.Email != nil {
		setClauses = append(setClauses, "email = ?")
		values = append(values, *req.Email)
	}
	if req.AppMetadata != nil {
		setClauses = append(setClauses, "raw_app_meta_data = ?")
		values = append(values, *req.AppMetadata)
	}
	if req.UserMetadata != nil {
		setClauses = append(setClauses, "raw_user_meta_data = ?")
		values = append(values, *req.UserMetadata)
	}
	if req.EmailConfirmed != nil {
		if *req.EmailConfirmed {
			setClauses = append(setClauses, "email_confirmed_at = datetime('now')")
		} else {
			setClauses = append(setClauses, "email_confirmed_at = NULL")
		}
	}

	if len(setClauses) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No fields to update"})
		return
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	values = append(values, userID)

	query := fmt.Sprintf(`UPDATE auth_users SET %s WHERE id = ?`, strings.Join(setClauses, ", "))
	result, err := h.db.Exec(query, values...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *Handler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "User ID required"})
		return
	}

	result, err := h.db.Exec(`DELETE FROM auth_users WHERE id = ?`, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// RLS Policy Handlers
// ============================================================================

func (h *Handler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")

	var rows *sql.Rows
	var err error
	if tableName != "" {
		rows, err = h.db.Query(`
			SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
			FROM _rls_policies WHERE table_name = ? ORDER BY policy_name
		`, tableName)
	} else {
		rows, err = h.db.Query(`
			SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
			FROM _rls_policies ORDER BY table_name, policy_name
		`)
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Policy struct {
		ID         int64  `json:"id"`
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
		Command    string `json:"command"`
		UsingExpr  string `json:"using_expr,omitempty"`
		CheckExpr  string `json:"check_expr,omitempty"`
		Enabled    bool   `json:"enabled"`
		CreatedAt  string `json:"created_at"`
	}

	policies := []Policy{}
	for rows.Next() {
		var p Policy
		var usingExpr, checkExpr sql.NullString
		var enabled int
		if err := rows.Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &enabled, &p.CreatedAt); err != nil {
			continue
		}
		p.UsingExpr = usingExpr.String
		p.CheckExpr = checkExpr.String
		p.Enabled = enabled == 1
		policies = append(policies, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"policies": policies})
}

func (h *Handler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
		Command    string `json:"command"`
		UsingExpr  string `json:"using_expr"`
		CheckExpr  string `json:"check_expr"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Validate required fields
	if req.TableName == "" || req.PolicyName == "" || req.Command == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "table_name, policy_name, and command are required"})
		return
	}

	// Validate command
	validCommands := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "ALL": true}
	if !validCommands[req.Command] {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "command must be SELECT, INSERT, UPDATE, DELETE, or ALL"})
		return
	}

	enabled := 1
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}

	result, err := h.db.Exec(`
		INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, req.TableName, req.PolicyName, req.Command, req.UsingExpr, req.CheckExpr, enabled)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "A policy with this name already exists for this table"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()

	// Fetch and return the created policy
	var p struct {
		ID         int64  `json:"id"`
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
		Command    string `json:"command"`
		UsingExpr  string `json:"using_expr,omitempty"`
		CheckExpr  string `json:"check_expr,omitempty"`
		Enabled    bool   `json:"enabled"`
		CreatedAt  string `json:"created_at"`
	}
	var usingExpr, checkExpr sql.NullString
	var enabledInt int
	h.db.QueryRow(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE id = ?
	`, id).Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &enabledInt, &p.CreatedAt)
	p.UsingExpr = usingExpr.String
	p.CheckExpr = checkExpr.String
	p.Enabled = enabledInt == 1

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid policy ID"})
		return
	}

	var p struct {
		ID         int64  `json:"id"`
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
		Command    string `json:"command"`
		UsingExpr  string `json:"using_expr,omitempty"`
		CheckExpr  string `json:"check_expr,omitempty"`
		Enabled    bool   `json:"enabled"`
		CreatedAt  string `json:"created_at"`
	}
	var usingExpr, checkExpr sql.NullString
	var enabled int
	err = h.db.QueryRow(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE id = ?
	`, id).Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &enabled, &p.CreatedAt)
	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Policy not found"})
		return
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	p.UsingExpr = usingExpr.String
	p.CheckExpr = checkExpr.String
	p.Enabled = enabled == 1

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid policy ID"})
		return
	}

	var req struct {
		PolicyName *string `json:"policy_name"`
		Command    *string `json:"command"`
		UsingExpr  *string `json:"using_expr"`
		CheckExpr  *string `json:"check_expr"`
		Enabled    *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Build update query dynamically
	var updates []string
	var args []interface{}
	if req.PolicyName != nil {
		updates = append(updates, "policy_name = ?")
		args = append(args, *req.PolicyName)
	}
	if req.Command != nil {
		validCommands := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "ALL": true}
		if !validCommands[*req.Command] {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "command must be SELECT, INSERT, UPDATE, DELETE, or ALL"})
			return
		}
		updates = append(updates, "command = ?")
		args = append(args, *req.Command)
	}
	if req.UsingExpr != nil {
		updates = append(updates, "using_expr = ?")
		args = append(args, *req.UsingExpr)
	}
	if req.CheckExpr != nil {
		updates = append(updates, "check_expr = ?")
		args = append(args, *req.CheckExpr)
	}
	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled {
			enabled = 1
		}
		updates = append(updates, "enabled = ?")
		args = append(args, enabled)
	}

	if len(updates) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No fields to update"})
		return
	}

	args = append(args, id)
	query := "UPDATE _rls_policies SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	result, err := h.db.Exec(query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "A policy with this name already exists for this table"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Policy not found"})
		return
	}

	// Fetch and return the updated policy
	var p struct {
		ID         int64  `json:"id"`
		TableName  string `json:"table_name"`
		PolicyName string `json:"policy_name"`
		Command    string `json:"command"`
		UsingExpr  string `json:"using_expr,omitempty"`
		CheckExpr  string `json:"check_expr,omitempty"`
		Enabled    bool   `json:"enabled"`
		CreatedAt  string `json:"created_at"`
	}
	var usingExpr, checkExpr sql.NullString
	var enabled int
	h.db.QueryRow(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE id = ?
	`, id).Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &enabled, &p.CreatedAt)
	p.UsingExpr = usingExpr.String
	p.CheckExpr = checkExpr.String
	p.Enabled = enabled == 1

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid policy ID"})
		return
	}

	result, err := h.db.Exec("DELETE FROM _rls_policies WHERE id = ?", id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Policy not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// RLS Table State Handlers
// ============================================================================

func (h *Handler) handleGetTableRLS(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	var enabled int
	err := h.db.QueryRow("SELECT enabled FROM _rls_tables WHERE table_name = ?", tableName).Scan(&enabled)
	if err == sql.ErrNoRows {
		// Default to disabled if not set
		enabled = 0
	} else if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Also get policy count for the table
	var policyCount int
	h.db.QueryRow("SELECT COUNT(*) FROM _rls_policies WHERE table_name = ?", tableName).Scan(&policyCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table_name":   tableName,
		"rls_enabled":  enabled == 1,
		"policy_count": policyCount,
	})
}

func (h *Handler) handleSetTableRLS(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	enabled := 0
	if req.Enabled {
		enabled = 1
	}

	_, err := h.db.Exec(`
		INSERT INTO _rls_tables (table_name, enabled) VALUES (?, ?)
		ON CONFLICT(table_name) DO UPDATE SET enabled = excluded.enabled
	`, tableName, enabled)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Get policy count
	var policyCount int
	h.db.QueryRow("SELECT COUNT(*) FROM _rls_policies WHERE table_name = ?", tableName).Scan(&policyCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table_name":   tableName,
		"rls_enabled":  req.Enabled,
		"policy_count": policyCount,
	})
}

// ============================================================================
// Policy Test Handler
// ============================================================================

func (h *Handler) handleTestPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Table     string `json:"table"`
		UsingExpr string `json:"using_expr"`
		CheckExpr string `json:"check_expr"`
		UserID    string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if req.Table == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "table is required"})
		return
	}

	// Get user details if user_id provided
	var userEmail, userRole string
	if req.UserID != "" {
		h.db.QueryRow("SELECT email, role FROM auth_users WHERE id = ?", req.UserID).Scan(&userEmail, &userRole)
		if userRole == "" {
			userRole = "authenticated"
		}
	}

	// Substitute auth functions in the expression
	testExpr := req.UsingExpr
	if testExpr == "" {
		testExpr = req.CheckExpr
	}
	if testExpr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "using_expr or check_expr is required"})
		return
	}

	// Replace auth functions with actual values
	substitutedExpr := testExpr
	if req.UserID != "" {
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.uid()", "'"+escapeSQLString(req.UserID)+"'")
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.email()", "'"+escapeSQLString(userEmail)+"'")
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.role()", "'"+escapeSQLString(userRole)+"'")
	} else {
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.uid()", "NULL")
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.email()", "NULL")
		substitutedExpr = strings.ReplaceAll(substitutedExpr, "auth.role()", "'anon'")
	}

	// Execute test query
	testSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", req.Table, substitutedExpr)
	var count int
	err := h.db.QueryRow(testSQL).Scan(&count)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"error":        err.Error(),
			"executed_sql": testSQL,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"row_count":    count,
		"executed_sql": testSQL,
	})
}

// escapeSQLString escapes single quotes in SQL strings
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// ============================================================================
// Settings Handlers
// ============================================================================

func (h *Handler) handleGetServerInfo(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(h.startTime)

	cfg := h.serverConfig
	if cfg == nil {
		cfg = &ServerConfig{Version: "0.1.0"}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":        cfg.Version,
		"host":           cfg.Host,
		"port":           cfg.Port,
		"db_path":        cfg.DBPath,
		"log_mode":       cfg.LogMode,
		"uptime_seconds": int(uptime.Seconds()),
		"uptime_human":   formatDuration(uptime),
		"memory_mb":      memStats.Alloc / 1024 / 1024,
		"memory_sys_mb":  memStats.Sys / 1024 / 1024,
		"goroutines":     runtime.NumGoroutine(),
		"go_version":     runtime.Version(),
	})
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (h *Handler) handleGetAuthSettings(w http.ResponseWriter, r *http.Request) {
	// Get JWT secret from _dashboard table or env
	var maskedSecret string
	var secretSource string

	// Check env first
	if secret := os.Getenv("SBLITE_JWT_SECRET"); secret != "" {
		secretSource = "environment"
		if len(secret) > 6 {
			maskedSecret = "***..." + secret[len(secret)-6:]
		} else {
			maskedSecret = "***"
		}
	} else {
		// Check _dashboard table
		var secret string
		err := h.db.QueryRow("SELECT value FROM _dashboard WHERE key = 'jwt_secret'").Scan(&secret)
		if err == nil && secret != "" {
			secretSource = "database"
			if len(secret) > 6 {
				maskedSecret = "***..." + secret[len(secret)-6:]
			} else {
				maskedSecret = "***"
			}
		} else {
			secretSource = "default (insecure)"
			maskedSecret = "using default secret"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jwt_secret_masked":     maskedSecret,
		"jwt_secret_source":     secretSource,
		"access_token_expiry":   "1 hour",
		"refresh_token_expiry":  "1 week",
		"can_regenerate":        secretSource != "environment",
	})
}

func (h *Handler) handleRegenerateSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	if req.Confirmation != "REGENERATE" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Please type REGENERATE to confirm"})
		return
	}

	// Check if secret is from environment (can't change)
	if os.Getenv("SBLITE_JWT_SECRET") != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot regenerate: JWT secret is set via environment variable"})
		return
	}

	// Generate new secret
	newSecret := uuid.New().String() + "-" + uuid.New().String()

	// Store in _dashboard table
	_, err := h.db.Exec(`
		INSERT INTO _dashboard (key, value, updated_at) VALUES ('jwt_secret', ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')
	`, newSecret, newSecret)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save new secret"})
		return
	}

	// Invalidate all refresh tokens
	_, err = h.db.Exec("UPDATE auth_refresh_tokens SET revoked = 1")
	if err != nil {
		// Log but don't fail - secret is already changed
	}

	// Delete all sessions
	_, err = h.db.Exec("DELETE FROM auth_sessions")
	if err != nil {
		// Log but don't fail
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"message":           "JWT secret regenerated. All user sessions have been invalidated.",
		"new_secret_masked": "***..." + newSecret[len(newSecret)-6:],
	})
}

func (h *Handler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, type, subject, body_html, body_text, updated_at
		FROM auth_email_templates
		ORDER BY type
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var templates []map[string]interface{}
	for rows.Next() {
		var id, ttype, subject, bodyHTML, updatedAt string
		var bodyText sql.NullString
		if err := rows.Scan(&id, &ttype, &subject, &bodyHTML, &bodyText, &updatedAt); err != nil {
			continue
		}
		templates = append(templates, map[string]interface{}{
			"id":         id,
			"type":       ttype,
			"subject":    subject,
			"body_html":  bodyHTML,
			"body_text":  bodyText.String,
			"updated_at": updatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

func (h *Handler) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	templateType := chi.URLParam(r, "type")

	var req struct {
		Subject  string `json:"subject"`
		BodyHTML string `json:"body_html"`
		BodyText string `json:"body_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	result, err := h.db.Exec(`
		UPDATE auth_email_templates
		SET subject = ?, body_html = ?, body_text = ?, updated_at = datetime('now')
		WHERE type = ?
	`, req.Subject, req.BodyHTML, req.BodyText, templateType)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Template not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"type":    templateType,
	})
}

func (h *Handler) handleResetTemplate(w http.ResponseWriter, r *http.Request) {
	templateType := chi.URLParam(r, "type")

	// Default templates
	defaults := map[string]struct {
		subject  string
		bodyHTML string
		bodyText string
	}{
		"confirmation": {
			subject:  "Confirm your email",
			bodyHTML: `<h2>Confirm your email</h2><p>Click the link below to confirm your email address:</p><p><a href="{{.ConfirmationURL}}">Confirm Email</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
			bodyText: "Confirm your email\n\nClick the link below to confirm your email address:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.",
		},
		"recovery": {
			subject:  "Reset your password",
			bodyHTML: `<h2>Reset your password</h2><p>Click the link below to reset your password:</p><p><a href="{{.ConfirmationURL}}">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
			bodyText: "Reset your password\n\nClick the link below to reset your password:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.",
		},
		"magic_link": {
			subject:  "Your login link",
			bodyHTML: `<h2>Your login link</h2><p>Click the link below to sign in:</p><p><a href="{{.ConfirmationURL}}">Sign In</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
			bodyText: "Your login link\n\nClick the link below to sign in:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.",
		},
		"email_change": {
			subject:  "Confirm email change",
			bodyHTML: `<h2>Confirm your new email</h2><p>Click the link below to confirm your new email address:</p><p><a href="{{.ConfirmationURL}}">Confirm New Email</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
			bodyText: "Confirm your new email\n\nClick the link below to confirm your new email address:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.",
		},
		"invite": {
			subject:  "You have been invited",
			bodyHTML: `<h2>You have been invited</h2><p>Click the link below to accept your invitation and set your password:</p><p><a href="{{.ConfirmationURL}}">Accept Invitation</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
			bodyText: "You have been invited\n\nClick the link below to accept your invitation and set your password:\n{{.ConfirmationURL}}\n\nThis link expires in {{.ExpiresIn}}.",
		},
	}

	def, ok := defaults[templateType]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown template type"})
		return
	}

	_, err := h.db.Exec(`
		UPDATE auth_email_templates
		SET subject = ?, body_html = ?, body_text = ?, updated_at = datetime('now')
		WHERE type = ?
	`, def.subject, def.bodyHTML, def.bodyText, templateType)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"type":      templateType,
		"subject":   def.subject,
		"body_html": def.bodyHTML,
		"body_text": def.bodyText,
	})
}

// ============================================================================
// Export Handlers
// ============================================================================

func (h *Handler) handleExportSchema(w http.ResponseWriter, r *http.Request) {
	// Get all user tables
	rows, err := h.db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%'
		AND name NOT LIKE 'auth_%'
		AND name NOT LIKE '_rls_%'
		AND name NOT LIKE '_columns'
		AND name NOT LIKE '_schema_%'
		AND name NOT LIKE '_dashboard'
		ORDER BY name
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}

	var sb strings.Builder
	sb.WriteString("-- PostgreSQL Schema Export from sblite\n")
	sb.WriteString("-- Generated at: " + time.Now().Format(time.RFC3339) + "\n\n")

	for _, table := range tables {
		sb.WriteString(h.generatePostgreSQLDDL(table))
		sb.WriteString("\n")
	}

	w.Header().Set("Content-Type", "application/sql")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=schema_%s.sql", time.Now().Format("20060102_150405")))
	w.Write([]byte(sb.String()))
}

func (h *Handler) generatePostgreSQLDDL(tableName string) string {
	var sb strings.Builder

	// Get column metadata from _columns table
	rows, err := h.db.Query(`
		SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns
		WHERE table_name = ?
		ORDER BY rowid
	`, tableName)
	if err != nil {
		// Fallback to basic table definition
		sb.WriteString(fmt.Sprintf("-- Table: %s (no metadata available)\n", tableName))
		return sb.String()
	}
	defer rows.Close()

	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))

	var columns []string
	var primaryKeys []string
	first := true

	for rows.Next() {
		var colName, pgType string
		var isNullable, isPrimary int
		var defaultVal sql.NullString

		if err := rows.Scan(&colName, &pgType, &isNullable, &defaultVal, &isPrimary); err != nil {
			continue
		}

		var colDef strings.Builder
		if !first {
			colDef.WriteString(",\n")
		}
		first = false

		colDef.WriteString(fmt.Sprintf("    %s %s", colName, pgType))

		if isNullable == 0 {
			colDef.WriteString(" NOT NULL")
		}

		if defaultVal.Valid && defaultVal.String != "" {
			colDef.WriteString(fmt.Sprintf(" DEFAULT %s", defaultVal.String))
		}

		if isPrimary == 1 {
			primaryKeys = append(primaryKeys, colName)
		}

		columns = append(columns, colDef.String())
	}

	sb.WriteString(strings.Join(columns, ""))

	if len(primaryKeys) > 0 {
		sb.WriteString(fmt.Sprintf(",\n    PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	sb.WriteString("\n);\n")

	return sb.String()
}

func (h *Handler) handleExportData(w http.ResponseWriter, r *http.Request) {
	tablesParam := r.URL.Query().Get("tables")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	if tablesParam == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "tables parameter required"})
		return
	}

	tables := strings.Split(tablesParam, ",")

	switch format {
	case "json":
		h.exportDataJSON(w, tables)
	case "csv":
		h.exportDataCSV(w, tables)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "format must be json or csv"})
	}
}

func (h *Handler) exportDataJSON(w http.ResponseWriter, tables []string) {
	result := make(map[string][]map[string]interface{})

	for _, table := range tables {
		table = strings.TrimSpace(table)
		// Validate table name (prevent SQL injection)
		if !isValidIdentifier(table) {
			continue
		}

		rows, err := h.db.Query(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			continue
		}

		columns, _ := rows.Columns()
		var tableData []map[string]interface{}

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				continue
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			tableData = append(tableData, row)
		}
		rows.Close()

		result[table] = tableData
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=data_%s.json", time.Now().Format("20060102_150405")))
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) exportDataCSV(w http.ResponseWriter, tables []string) {
	// For CSV, we only export the first table
	if len(tables) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No tables specified"})
		return
	}

	table := strings.TrimSpace(tables[0])
	if !isValidIdentifier(table) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid table name"})
		return
	}

	rows, err := h.db.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.csv", table, time.Now().Format("20060102_150405")))

	csvWriter := csv.NewWriter(w)
	csvWriter.Write(columns)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		record := make([]string, len(columns))
		for i, v := range values {
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		csvWriter.Write(record)
	}

	csvWriter.Flush()
}

func (h *Handler) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	dbPath := h.serverConfig.DBPath
	if dbPath == "" {
		dbPath = "./data.db"
	}

	file, err := os.Open(dbPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot open database file"})
		return
	}
	defer file.Close()

	stat, _ := file.Stat()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=backup_%s.db", time.Now().Format("20060102_150405")))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	io.Copy(w, file)
}

func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 && c >= '0' && c <= '9' {
			return false
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// ============================================================================
// Logs Handlers
// ============================================================================

func (h *Handler) handleGetLogConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.serverConfig
	if cfg == nil {
		cfg = &ServerConfig{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":      cfg.LogMode,
		"file_path": cfg.LogFile,
		"db_path":   cfg.LogDB,
	})
}

func (h *Handler) handleQueryLogs(w http.ResponseWriter, r *http.Request) {
	cfg := h.serverConfig
	if cfg == nil || cfg.LogMode != "database" || cfg.LogDB == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":    []interface{}{},
			"total":   0,
			"message": "Database logging is not enabled. Start server with --log-mode=database",
		})
		return
	}

	// Open log database
	logDB, err := sql.Open("sqlite", cfg.LogDB+"?mode=ro")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot open log database"})
		return
	}
	defer logDB.Close()

	// Parse query params
	level := r.URL.Query().Get("level")
	since := r.URL.Query().Get("since")
	until := r.URL.Query().Get("until")
	search := r.URL.Query().Get("search")
	userID := r.URL.Query().Get("user_id")
	requestID := r.URL.Query().Get("request_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	// Build query
	var conditions []string
	var args []interface{}

	if level != "" && level != "all" {
		conditions = append(conditions, "level = ?")
		args = append(args, strings.ToUpper(level))
	}
	if since != "" {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, since)
	}
	if until != "" {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, until)
	}
	if search != "" {
		conditions = append(conditions, "message LIKE ?")
		args = append(args, "%"+search+"%")
	}
	if userID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, userID)
	}
	if requestID != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, requestID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM logs %s", whereClause)
	logDB.QueryRow(countQuery, args...).Scan(&total)

	// Fetch logs
	query := fmt.Sprintf(`
		SELECT id, timestamp, level, message, source, request_id, user_id, extra
		FROM logs %s
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)
	rows, err := logDB.Query(query, args...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id int64
		var timestamp, level, message string
		var source, reqID, uID, extra sql.NullString

		if err := rows.Scan(&id, &timestamp, &level, &message, &source, &reqID, &uID, &extra); err != nil {
			continue
		}

		log := map[string]interface{}{
			"id":         id,
			"timestamp":  timestamp,
			"level":      level,
			"message":    message,
			"source":     source.String,
			"request_id": reqID.String,
			"user_id":    uID.String,
		}

		if extra.Valid && extra.String != "" {
			var extraData interface{}
			if json.Unmarshal([]byte(extra.String), &extraData) == nil {
				log["extra"] = extraData
			}
		}

		logs = append(logs, log)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":     logs,
		"total":    total,
		"has_more": offset+len(logs) < total,
	})
}

func (h *Handler) handleTailLogs(w http.ResponseWriter, r *http.Request) {
	cfg := h.serverConfig
	if cfg == nil || cfg.LogMode != "file" || cfg.LogFile == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lines":   []string{},
			"message": "File logging is not enabled or no log file configured",
		})
		return
	}

	linesStr := r.URL.Query().Get("lines")
	numLines := 100
	if n, err := strconv.Atoi(linesStr); err == nil && n > 0 && n <= 1000 {
		numLines = n
	}

	file, err := os.Open(cfg.LogFile)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot open log file: " + err.Error()})
		return
	}
	defer file.Close()

	// Read all lines (simple approach for small files)
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Return last N lines
	start := 0
	if len(lines) > numLines {
		start = len(lines) - numLines
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"lines":      lines[start:],
		"total":      len(lines),
		"showing":    len(lines) - start,
		"file_path":  cfg.LogFile,
	})
}
