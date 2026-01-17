package dashboard

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed static/*
var staticFS embed.FS

const sessionCookieName = "_sblite_session"

// Handler serves the dashboard UI and API.
type Handler struct {
	db       *sql.DB
	store    *Store
	auth     *Auth
	sessions *SessionManager
}

// NewHandler creates a new Handler.
func NewHandler(db *sql.DB) *Handler {
	store := NewStore(db)
	return &Handler{
		db:       db,
		store:    store,
		auth:     NewAuth(store),
		sessions: NewSessionManager(store),
	}
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
			r.Get("/{id}", h.handleGetUser)
			r.Patch("/{id}", h.handleUpdateUser)
			r.Delete("/{id}", h.handleDeleteUser)
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
