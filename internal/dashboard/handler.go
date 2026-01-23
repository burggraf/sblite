package dashboard

import (
	"bufio"
	"bytes"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/markb/sblite/internal/fts"
	"github.com/markb/sblite/internal/functions"
	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/mail/viewer"
	"github.com/markb/sblite/internal/pgtranslate"
	"github.com/markb/sblite/internal/rpc"
	"github.com/markb/sblite/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

//go:embed static/*
var staticFS embed.FS

const sessionCookieName = "_sblite_session"

// RealtimeStatsProvider provides realtime statistics
type RealtimeStatsProvider interface {
	Stats() any
}

// Handler serves the dashboard UI and API.
type Handler struct {
	db               *sql.DB
	store            *Store
	auth             *Auth
	sessions         *SessionManager
	fts              *fts.Manager
	functionsService *functions.Service
	storageService   *storage.Service
	rpcInterceptor   *rpc.Interceptor
	catchMailer      *mail.CatchMailer
	mailViewer       *viewer.Handler
	migrationsDir    string
	startTime        time.Time
	serverConfig     *ServerConfig
	jwtSecret        string
	oauthReloadFunc   func()
	onSiteURLChange   func(string)
	onStorageReload   func(*StorageConfig) error
	onMailReload      func(*MailConfig) error
	realtimeService   RealtimeStatsProvider
}

// ServerConfig holds server configuration for display in settings.
type ServerConfig struct {
	Version string
	Host    string
	Port    int
	DBPath  string
	LogMode string
	LogFile string
	LogDB   string
}

// NewHandler creates a new Handler.
func NewHandler(db *sql.DB, migrationsDir string) *Handler {
	store := NewStore(db)
	return &Handler{
		db:            db,
		store:         store,
		auth:          NewAuth(store),
		sessions:      NewSessionManager(store),
		fts:           fts.NewManager(db),
		migrationsDir: migrationsDir,
		startTime:     time.Now(),
		serverConfig:  &ServerConfig{Version: "0.1.1"},
	}
}

// SetServerConfig sets the server configuration for display.
func (h *Handler) SetServerConfig(cfg *ServerConfig) {
	h.serverConfig = cfg
}

// SetJWTSecret sets the JWT secret for API key generation.
func (h *Handler) SetJWTSecret(secret string) {
	h.jwtSecret = secret
}

// SetOAuthReloadFunc sets the callback function to be called when OAuth settings change.
func (h *Handler) SetOAuthReloadFunc(f func()) {
	h.oauthReloadFunc = f
}

// SetFunctionsService sets the functions service for the handler.
func (h *Handler) SetFunctionsService(svc *functions.Service) {
	h.functionsService = svc
}

// GetStore returns the dashboard store for auth settings.
func (h *Handler) GetStore() *Store {
	return h.store
}

// SetStorageService sets the storage service for the handler.
func (h *Handler) SetStorageService(svc *storage.Service) {
	h.storageService = svc
}

// SetRPCInterceptor sets the RPC interceptor for SQL statement handling.
func (h *Handler) SetRPCInterceptor(i *rpc.Interceptor) {
	h.rpcInterceptor = i
}

// SetStorageReloadFunc sets the callback function for storage configuration changes.
func (h *Handler) SetStorageReloadFunc(f func(*StorageConfig) error) {
	h.onStorageReload = f
}

// SetMailReloadFunc sets the callback function for mail configuration changes.
func (h *Handler) SetMailReloadFunc(f func(*MailConfig) error) {
	h.onMailReload = f
}

// SetCatchMailer sets the catch mailer for the mail viewer.
func (h *Handler) SetCatchMailer(cm *mail.CatchMailer) {
	h.catchMailer = cm
	if cm != nil {
		h.mailViewer = viewer.NewHandler(cm)
	}
}

// SetRealtimeService sets the realtime service for stats
func (h *Handler) SetRealtimeService(svc RealtimeStatsProvider) {
	h.realtimeService = svc
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

		// FTS index management routes (nested under tables)
		r.Route("/tables/{name}/fts", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListFTSIndexes)
			r.Post("/", h.handleCreateFTSIndex)
			r.Post("/test", h.handleTestFTSSearch)
			r.Get("/{index}", h.handleGetFTSIndex)
			r.Delete("/{index}", h.handleDeleteFTSIndex)
			r.Post("/{index}/rebuild", h.handleRebuildFTSIndex)
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
			// OAuth settings routes
			r.Get("/oauth", h.handleGetOAuthSettings)
			r.Patch("/oauth", h.handleUpdateOAuthSettings)
			r.Get("/oauth/redirect-urls", h.handleGetRedirectURLs)
			r.Post("/oauth/redirect-urls", h.handleAddRedirectURL)
			r.Delete("/oauth/redirect-urls", h.handleDeleteRedirectURL)
			// Auth configuration settings routes
			r.Get("/auth-config", h.handleGetAuthConfig)
			r.Patch("/auth-config", h.handlePatchAuthConfig)
			// Storage settings routes
			r.Get("/storage", h.handleGetStorageSettings)
			r.Patch("/storage", h.handleUpdateStorageSettings)
			r.Post("/storage/test", h.handleTestStorageConnection)
			// Mail settings routes
			r.Get("/mail", h.handleGetMailSettings)
			r.Patch("/mail", h.handleUpdateMailSettings)
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
			r.Get("/buffer", h.handleBufferLogs)
		})

		// SQL Browser route (require auth)
		r.Route("/sql", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Post("/", h.handleExecuteSQL)
		})

		// API Keys route (require auth)
		r.Route("/apikeys", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleGetAPIKeys)
		})

		// Functions management routes (require auth)
		r.Route("/functions", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListFunctions)
			r.Get("/status", h.handleGetFunctionsStatus)
			r.Post("/{name}", h.handleCreateFunction)
			r.Get("/{name}", h.handleGetFunction)
			r.Delete("/{name}", h.handleDeleteFunction)
			r.Get("/{name}/config", h.handleGetFunctionConfig)
			r.Patch("/{name}/config", h.handleUpdateFunctionConfig)

			// File operations (rename must come before wildcard routes)
			r.Get("/{name}/files", h.handleListFunctionFiles)
			r.Post("/{name}/files/rename", h.handleRenameFunctionFile)
			r.Get("/{name}/files/*", h.handleReadFunctionFile)
			r.Put("/{name}/files/*", h.handleWriteFunctionFile)
			r.Delete("/{name}/files/*", h.handleDeleteFunctionFile)

			// Runtime operations
			r.Post("/{name}/restart", h.handleRestartFunctions)
		})

		// Secrets management routes (require auth)
		r.Route("/secrets", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/", h.handleListSecrets)
			r.Post("/", h.handleSetSecret)
			r.Delete("/{name}", h.handleDeleteSecret)
		})

		// Storage management routes (require auth)
		r.Route("/storage", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/buckets", h.handleListBuckets)
			r.Post("/buckets", h.handleCreateBucket)
			r.Get("/buckets/{id}", h.handleGetBucket)
			r.Put("/buckets/{id}", h.handleUpdateBucket)
			r.Delete("/buckets/{id}", h.handleDeleteBucket)
			r.Post("/buckets/{id}/empty", h.handleEmptyBucket)
			// Object routes
			r.Post("/objects/list", h.handleListObjects)
			r.Post("/objects/upload", h.handleUploadObject)
			r.Get("/objects/download", h.handleDownloadObject)
			r.Delete("/objects", h.handleDeleteObjects)
		})

		// API Docs routes (require auth)
		r.Route("/apidocs", func(r chi.Router) {
			r.Use(h.requireAuth)
			// Tables documentation
			r.Get("/tables", h.handleAPIDocsListTables)
			r.Get("/tables/{name}", h.handleAPIDocsGetTable)
			r.Patch("/tables/{name}/description", h.handleAPIDocsUpdateTableDescription)
			r.Patch("/tables/{name}/columns/{column}/description", h.handleAPIDocsUpdateColumnDescription)
			// RPC functions documentation
			r.Get("/functions", h.handleAPIDocsListFunctions)
			r.Get("/functions/{name}", h.handleAPIDocsGetFunction)
			r.Patch("/functions/{name}/description", h.handleAPIDocsUpdateFunctionDescription)
		})

		// Realtime stats route (require auth)
		r.Route("/realtime", func(r chi.Router) {
			r.Use(h.requireAuth)
			r.Get("/stats", h.handleRealtimeStats)
		})
	})

	// Mail viewer routes (require auth, only when catch mailer is configured)
	if h.mailViewer != nil {
		r.Route("/mail", func(r chi.Router) {
			r.Use(h.requireAuth)
			h.mailViewer.RegisterRoutes(r)
		})
	}

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
	// Query sqlite_master for all user tables, filtering out internal tables
	rows, err := h.db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table'
		AND name NOT LIKE '\_%' ESCAPE '\'
		AND name NOT LIKE 'auth\_%' ESCAPE '\'
		AND name NOT LIKE 'storage\_%' ESCAPE '\'
		AND name != 'sqlite_sequence'
		ORDER BY name
	`)
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

	// Auto-register columns if table exists but isn't in _columns
	if err := h.ensureTableRegistered(tableName); err != nil {
		// Log but don't fail - table might not exist yet
	}

	// First, get actual columns from SQLite table schema using PRAGMA
	pragmaRows, err := h.db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get table info"})
		return
	}

	// Build a map of actual columns from PRAGMA with their order
	type pragmaColumn struct {
		cid        int
		name       string
		sqliteType string
		notnull    bool
		dfltValue  sql.NullString
		pk         bool
	}
	var pragmaCols []pragmaColumn
	for pragmaRows.Next() {
		var col pragmaColumn
		var pkInt int
		var notnullInt int
		if err := pragmaRows.Scan(&col.cid, &col.name, &col.sqliteType, &notnullInt, &col.dfltValue, &pkInt); err != nil {
			continue
		}
		col.notnull = notnullInt != 0
		col.pk = pkInt != 0
		pragmaCols = append(pragmaCols, col)
	}
	pragmaRows.Close()

	if len(pragmaCols) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	// Get metadata from _columns table (may not have all columns)
	metaRows, err := h.db.Query(`SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns WHERE table_name = ?`, tableName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get schema metadata"})
		return
	}
	defer metaRows.Close()

	// Build a map of _columns metadata by column name
	metaMap := make(map[string]map[string]interface{})
	for metaRows.Next() {
		var name, pgType string
		var nullable, primary bool
		var defaultVal sql.NullString
		if err := metaRows.Scan(&name, &pgType, &nullable, &defaultVal, &primary); err != nil {
			continue
		}
		meta := map[string]interface{}{
			"type":     pgType,
			"nullable": nullable,
			"primary":  primary,
		}
		if defaultVal.Valid {
			meta["default"] = defaultVal.String
		}
		metaMap[name] = meta
	}

	// Merge: use PRAGMA for column order and existence, _columns for type metadata
	var columns []map[string]interface{}
	for _, pc := range pragmaCols {
		col := map[string]interface{}{
			"name":     pc.name,
			"nullable": !pc.notnull,
			"primary":  pc.pk,
		}

		// Use _columns metadata if available, otherwise infer from SQLite type
		if meta, ok := metaMap[pc.name]; ok {
			col["type"] = meta["type"]
			col["nullable"] = meta["nullable"]
			col["primary"] = meta["primary"]
			if dflt, ok := meta["default"]; ok {
				col["default"] = dflt
			}
		} else {
			// Infer PostgreSQL type from SQLite type
			col["type"] = sqliteTypeToPgType(pc.sqliteType)
		}

		if pc.dfltValue.Valid && col["default"] == nil {
			col["default"] = pc.dfltValue.String
		}

		columns = append(columns, col)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    tableName,
		"columns": columns,
	})
}

// sqliteTypeToPgType converts SQLite type affinity to a reasonable PostgreSQL type
func sqliteTypeToPgType(sqliteType string) string {
	sqliteType = strings.ToUpper(strings.TrimSpace(sqliteType))
	switch {
	case strings.Contains(sqliteType, "INT"):
		return "integer"
	case strings.Contains(sqliteType, "CHAR"), strings.Contains(sqliteType, "CLOB"), strings.Contains(sqliteType, "TEXT"):
		return "text"
	case strings.Contains(sqliteType, "BLOB"):
		return "bytea"
	case strings.Contains(sqliteType, "REAL"), strings.Contains(sqliteType, "FLOA"), strings.Contains(sqliteType, "DOUB"):
		return "numeric"
	case sqliteType == "BOOLEAN" || sqliteType == "BOOL":
		return "boolean"
	default:
		return "text"
	}
}

// ensureTableRegistered checks if a table has column metadata in _columns,
// and if not, auto-registers columns by inferring types from SQLite schema.
// This allows tables created via migrations or SQL browser to appear in the dashboard.
func (h *Handler) ensureTableRegistered(tableName string) error {
	// Get existing columns for this table from _columns
	existingCols := make(map[string]bool)
	rows, err := h.db.Query(`SELECT column_name FROM _columns WHERE table_name = ?`, tableName)
	if err != nil {
		return fmt.Errorf("failed to query _columns: %w", err)
	}
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan column name: %w", err)
		}
		existingCols[colName] = true
	}
	rows.Close()

	// Get actual columns from SQLite PRAGMA
	pragmaRows, err := h.db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer pragmaRows.Close()

	// Prepare insert statement for missing columns
	insertStmt, err := h.db.Prepare(`
		INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary, description)
		VALUES (?, ?, ?, ?, ?, ?, '')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer insertStmt.Close()

	// Register any columns not already in _columns
	for pragmaRows.Next() {
		var cid int
		var name, sqliteType string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := pragmaRows.Scan(&cid, &name, &sqliteType, &notnull, &dfltValue, &pk); err != nil {
			continue
		}

		// Skip if already registered
		if existingCols[name] {
			continue
		}

		// Infer PostgreSQL type
		pgType := sqliteTypeToPgType(sqliteType)

		// Insert into _columns
		var defaultVal interface{}
		if dfltValue.Valid {
			defaultVal = dfltValue.String
		}
		_, err = insertStmt.Exec(tableName, name, pgType, notnull == 0, defaultVal, pk != 0)
		if err != nil {
			// Log but don't fail - column might have been added by another request
			continue
		}
	}

	return nil
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

	// Auto-register columns if table exists but isn't in _columns
	if err := h.ensureTableRegistered(tableName); err != nil {
		// Log but don't fail - table might not exist yet
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

	// Build WHERE clause based on filter
	filter := r.URL.Query().Get("filter")
	whereClause := ""
	switch filter {
	case "regular":
		whereClause = "WHERE is_anonymous = 0"
	case "anonymous":
		whereClause = "WHERE is_anonymous = 1"
		// "all" or empty = no filter
	}

	// Get total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM auth_users %s", whereClause)
	err := h.db.QueryRow(countQuery).Scan(&total)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to count users"})
		return
	}

	// Get users
	usersQuery := fmt.Sprintf(`
		SELECT id, email, email_confirmed_at, last_sign_in_at,
		       raw_app_meta_data, raw_user_meta_data, created_at, updated_at, is_anonymous
		FROM auth_users
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, whereClause)
	rows, err := h.db.Query(usersQuery, limit, offset)
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
		var isAnonymous int
		if err := rows.Scan(&id, &email, &emailConfirmedAt, &lastSignInAt, &appMeta, &userMeta, &createdAt, &updatedAt, &isAnonymous); err != nil {
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
			"is_anonymous":       isAnonymous == 1,
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
		cfg = &ServerConfig{Version: "0.1.1"}
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
		"jwt_secret_masked":    maskedSecret,
		"jwt_secret_source":    secretSource,
		"access_token_expiry":  "1 hour",
		"refresh_token_expiry": "1 week",
		"can_regenerate":       secretSource != "environment",
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
		"lines":     lines[start:],
		"total":     len(lines),
		"showing":   len(lines) - start,
		"file_path": cfg.LogFile,
	})
}

func (h *Handler) handleBufferLogs(w http.ResponseWriter, r *http.Request) {
	linesStr := r.URL.Query().Get("lines")
	numLines := 100
	if n, err := strconv.Atoi(linesStr); err == nil && n > 0 {
		numLines = n
	}

	lines := log.GetBufferedLogs(numLines)
	total, capacity, ok := log.GetBufferStats()

	w.Header().Set("Content-Type", "application/json")

	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lines":       []string{},
			"total":       0,
			"showing":     0,
			"buffer_size": 0,
			"enabled":     false,
			"message":     "Log buffer is disabled. Start server with --log-buffer-lines=500",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"lines":       lines,
		"total":       total,
		"showing":     len(lines),
		"buffer_size": capacity,
		"enabled":     true,
	})
}

// SQL Browser handlers

type SQLRequest struct {
	Query        string        `json:"query"`
	Params       []interface{} `json:"params"`
	PostgresMode bool          `json:"postgres_mode,omitempty"`
}

type SQLResponse struct {
	Columns         []string        `json:"columns"`
	Rows            [][]interface{} `json:"rows"`
	RowCount        int             `json:"row_count"`
	AffectedRows    int64           `json:"affected_rows,omitempty"`
	ExecutionTimeMs int64           `json:"execution_time_ms"`
	Type            string          `json:"type"`
	Error           string          `json:"error,omitempty"`
	TranslatedQuery string          `json:"translated_query,omitempty"`
	WasTranslated   bool            `json:"was_translated,omitempty"`
}

func (h *Handler) handleExecuteSQL(w http.ResponseWriter, r *http.Request) {
	var req SQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		http.Error(w, `{"error": "Query cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// Detect query type
	queryType := detectQueryType(req.Query)

	// Initialize response
	var response SQLResponse
	response.Type = queryType

	// Translate PostgreSQL syntax to SQLite if requested
	queryToExecute := req.Query
	if req.PostgresMode {
		translated, wasTranslated := pgtranslate.TranslateWithFallback(req.Query)
		queryToExecute = translated
		response.TranslatedQuery = translated
		response.WasTranslated = wasTranslated
	}

	startTime := time.Now()

	// Intercept CREATE/DROP FUNCTION statements
	if h.rpcInterceptor != nil {
		result, handled, err := h.rpcInterceptor.ProcessSQL(queryToExecute, req.PostgresMode)
		if handled {
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			if err != nil {
				response.Error = err.Error()
			} else {
				// Determine type from result message (e.g., "CREATE FUNCTION" or "DROP FUNCTION")
				if strings.HasPrefix(result, "DROP") {
					response.Type = "DROP"
				} else {
					response.Type = "CREATE"
				}
				response.AffectedRows = 1
				response.RowCount = 1
				// Store result message in a way the UI can display
				response.Rows = [][]interface{}{{result}}
				response.Columns = []string{"result"}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	if queryType == "SELECT" || queryType == "PRAGMA" {
		// For SELECT queries, return rows
		rows, err := h.db.Query(queryToExecute)
		if err != nil {
			response.Error = err.Error()
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK) // Return 200 with error in body for SQL errors
			json.NewEncoder(w).Encode(response)
			return
		}
		defer rows.Close()

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			response.Error = err.Error()
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		response.Columns = columns

		// Fetch all rows
		var resultRows [][]interface{}
		for rows.Next() {
			// Create slice of interface{} pointers for scanning
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				response.Error = err.Error()
				break
			}

			// Convert values to JSON-friendly types
			row := make([]interface{}, len(columns))
			for i, v := range values {
				switch val := v.(type) {
				case []byte:
					row[i] = string(val)
				case nil:
					row[i] = nil
				default:
					row[i] = val
				}
			}
			resultRows = append(resultRows, row)
		}

		response.Rows = resultRows
		response.RowCount = len(resultRows)
	} else {
		// For non-SELECT queries (INSERT, UPDATE, DELETE, etc.)
		result, err := h.db.Exec(queryToExecute)
		if err != nil {
			response.Error = err.Error()
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		affected, _ := result.RowsAffected()
		response.AffectedRows = affected
		response.RowCount = int(affected)
	}

	response.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func detectQueryType(query string) string {
	// Normalize query: trim whitespace and convert to uppercase for detection
	normalized := strings.ToUpper(strings.TrimSpace(query))

	// Check for common query types
	prefixes := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE",
		"CREATE", "DROP", "ALTER", "TRUNCATE",
		"PRAGMA", "EXPLAIN", "VACUUM", "REINDEX",
		"BEGIN", "COMMIT", "ROLLBACK",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return prefix
		}
	}

	return "UNKNOWN"
}

// ============================================================================
// API Keys Handler
// ============================================================================

func (h *Handler) handleGetAPIKeys(w http.ResponseWriter, r *http.Request) {
	if h.jwtSecret == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "JWT secret not configured"})
		return
	}

	anonKey, err := h.generateAPIKey("anon")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate anon key"})
		return
	}

	serviceKey, err := h.generateAPIKey("service_role")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate service_role key"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"anon_key":         anonKey,
		"service_role_key": serviceKey,
	})
}

func (h *Handler) generateAPIKey(role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"role": role,
		"iss":  "sblite",
		"iat":  now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// ============================================================================
// FTS Index Management Handlers
// ============================================================================

func (h *Handler) handleListFTSIndexes(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	indexes, err := h.fts.ListIndexes(tableName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"indexes": indexes})
}

func (h *Handler) handleCreateFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	var req struct {
		Name      string   `json:"name"`
		Columns   []string `json:"columns"`
		Tokenizer string   `json:"tokenizer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Index name is required"})
		return
	}

	if len(req.Columns) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "At least one column is required"})
		return
	}

	err := h.fts.CreateIndex(tableName, req.Name, req.Columns, req.Tokenizer)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Fetch the created index to return
	index, err := h.fts.GetIndex(tableName, req.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Index created but failed to fetch details"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(index)
}

func (h *Handler) handleGetFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name and index name required"})
		return
	}

	index, err := h.fts.GetIndex(tableName, indexName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(index)
}

func (h *Handler) handleDeleteFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name and index name required"})
		return
	}

	err := h.fts.DropIndex(tableName, indexName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRebuildFTSIndex(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	indexName := chi.URLParam(r, "index")

	if tableName == "" || indexName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name and index name required"})
		return
	}

	err := h.fts.RebuildIndex(tableName, indexName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Index rebuilt successfully",
	})
}

func (h *Handler) handleTestFTSSearch(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	var req struct {
		IndexName string `json:"index_name"`
		Query     string `json:"query"`
		QueryType string `json:"query_type"` // plain, phrase, websearch, fts
		Limit     int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Query is required"})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Get the index to find the FTS table name
	index, err := h.fts.GetIndex(tableName, req.IndexName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Convert query based on type
	ftsQuery := req.Query
	if req.QueryType == "" {
		req.QueryType = "plain"
	}
	ftsQuery, err = fts.ConvertQuery(req.Query, req.QueryType)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid query: " + err.Error()})
		return
	}

	// Build and execute the search query
	ftsTableName := fts.GetFTSTableName(tableName, req.IndexName)

	// Get primary key column for the table
	var pkColumn string
	err = h.db.QueryRow(`SELECT name FROM pragma_table_info(?) WHERE pk = 1`, tableName).Scan(&pkColumn)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get primary key"})
		return
	}

	// Query joining source table with FTS for ranking
	query := fmt.Sprintf(`
		SELECT t.*, fts.rank
		FROM %q t
		JOIN %q fts ON t.%q = fts.rowid
		WHERE %q MATCH ?
		ORDER BY fts.rank
		LIMIT ?
	`, tableName, ftsTableName, pkColumn, ftsTableName)

	rows, err := h.db.Query(query, ftsQuery, req.Limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"error":        err.Error(),
			"executed_sql": query,
			"fts_query":    ftsQuery,
		})
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

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"results":    results,
		"count":      len(results),
		"index":      index,
		"fts_query":  ftsQuery,
		"query_type": req.QueryType,
	})
}

// ============================================================
// Functions Management Handlers
// ============================================================

// handleListFunctions returns a list of all edge functions.
func (h *Handler) handleListFunctions(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"functions": []interface{}{},
			"enabled":   false,
			"message":   "Edge functions are not enabled. Start the server with --functions flag.",
		})
		return
	}

	funcs, err := h.functionsService.ListFunctions()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Enrich with metadata
	for i := range funcs {
		meta, err := h.functionsService.GetMetadata(funcs[i].Name)
		if err == nil {
			funcs[i].VerifyJWT = meta.VerifyJWT
		} else {
			funcs[i].VerifyJWT = true // Default
		}

		if h.functionsService.IsRunning() {
			funcs[i].Status = "ready"
		} else {
			funcs[i].Status = "unavailable"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"functions": funcs,
		"enabled":   true,
	})
}

// handleGetFunctionsStatus returns the status of the edge runtime.
func (h *Handler) handleGetFunctionsStatus(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":       false,
			"status":        "disabled",
			"runtime_port":  0,
			"functions_dir": "",
		})
		return
	}

	status := "stopped"
	if h.functionsService.IsRunning() {
		status = "running"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":       true,
		"status":        status,
		"runtime_port":  h.functionsService.RuntimePort(),
		"functions_dir": h.functionsService.FunctionsDir(),
	})
}

// handleGetFunction returns details about a specific function.
func (h *Handler) handleGetFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	fn, err := h.functionsService.GetFunction(name)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Add metadata
	meta, err := h.functionsService.GetMetadata(name)
	if err == nil {
		fn.VerifyJWT = meta.VerifyJWT
	} else {
		fn.VerifyJWT = true
	}

	if h.functionsService.IsRunning() {
		fn.Status = "ready"
	} else {
		fn.Status = "unavailable"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fn)
}

// handleCreateFunction creates a new edge function from template.
func (h *Handler) handleCreateFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	// Validate function name
	if err := functions.ValidateFunctionName(name); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Parse request body for template type
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to "default" template if no body
		req.Template = "default"
	}
	if req.Template == "" {
		req.Template = "default"
	}

	if err := h.functionsService.CreateFunction(name, req.Template); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	fn, _ := h.functionsService.GetFunction(name)
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fn)
}

// handleDeleteFunction deletes an edge function.
func (h *Handler) handleDeleteFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	if err := h.functionsService.DeleteFunction(name); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetFunctionConfig returns the configuration for a specific function.
func (h *Handler) handleGetFunctionConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	meta, err := h.functionsService.GetMetadata(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// handleUpdateFunctionConfig updates the configuration for a specific function.
func (h *Handler) handleUpdateFunctionConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	var req struct {
		VerifyJWT *bool             `json:"verify_jwt"`
		MemoryMB  *int              `json:"memory_mb"`
		TimeoutMS *int              `json:"timeout_ms"`
		ImportMap *string           `json:"import_map"`
		EnvVars   map[string]string `json:"env_vars"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}

	// Get existing metadata
	meta, err := h.functionsService.GetMetadata(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.VerifyJWT != nil {
		meta.VerifyJWT = *req.VerifyJWT
	}
	if req.MemoryMB != nil {
		meta.MemoryMB = *req.MemoryMB
	}
	if req.TimeoutMS != nil {
		meta.TimeoutMS = *req.TimeoutMS
	}
	if req.ImportMap != nil {
		meta.ImportMap = *req.ImportMap
	}
	if req.EnvVars != nil {
		meta.EnvVars = req.EnvVars
	}

	if err := h.functionsService.SetMetadata(meta); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// ============================================================
// Function Files Handlers
// ============================================================

// FileNode represents a file or directory in the function's file tree.
type FileNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "dir"
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// handleListFunctionFiles returns a recursive tree of files in a function directory.
func (h *Handler) handleListFunctionFiles(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	// Build the function directory path
	fnDir := filepath.Join(h.functionsService.FunctionsDir(), name)

	// Check if the directory exists
	info, err := os.Stat(fnDir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Function %q not found", name),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to access function directory: %v", err),
		})
		return
	}

	if !info.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Function %q is not a directory", name),
		})
		return
	}

	// Build the file tree
	tree, err := buildFileTree(fnDir, name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to build file tree: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

// buildFileTree recursively builds a FileNode tree from a directory.
func buildFileTree(dir, name string) (*FileNode, error) {
	node := &FileNode{
		Name:     name,
		Type:     "dir",
		Children: []*FileNode{},
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		entryName := entry.Name()

		// Skip hidden files and directories (starting with .)
		if strings.HasPrefix(entryName, ".") {
			continue
		}

		entryPath := filepath.Join(dir, entryName)

		if entry.IsDir() {
			// Recursively build tree for subdirectories
			childNode, err := buildFileTree(entryPath, entryName)
			if err != nil {
				// Skip directories we can't read
				continue
			}
			node.Children = append(node.Children, childNode)
		} else {
			// Check if file extension is allowed
			ext := filepath.Ext(entryName)
			if !IsAllowedExtension(ext) {
				continue
			}

			// Get file info for size
			info, err := entry.Info()
			if err != nil {
				continue
			}

			node.Children = append(node.Children, &FileNode{
				Name: entryName,
				Type: "file",
				Size: info.Size(),
			})
		}
	}

	return node, nil
}

// handleReadFunctionFile returns the content of a specific file in a function directory.
func (h *Handler) handleReadFunctionFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	// Build base path and sanitize the file path
	basePath := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(basePath, filePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid file path: %v", err),
		})
		return
	}

	// Check if file exists and get info
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("File %q not found", filePath),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to access file: %v", err),
		})
		return
	}

	// Check if it's a directory
	if info.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Cannot read directory as file",
		})
		return
	}

	// Check file size
	if info.Size() > MaxFileSize {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("File too large (%d bytes). Maximum allowed size is %d bytes", info.Size(), MaxFileSize),
		})
		return
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to read file: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    filePath,
		"content": string(content),
		"size":    info.Size(),
	})
}

// handleWriteFunctionFile creates or updates a file in a function directory.
func (h *Handler) handleWriteFunctionFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	// Build base path and sanitize the file path
	basePath := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(basePath, filePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid file path: %v", err),
		})
		return
	}

	// Parse request body
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to create directory: %v", err),
		})
		return
	}

	// Write file content
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to write file: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"path":   filePath,
	})
}

// handleDeleteFunctionFile deletes a file or directory in a function directory.
func (h *Handler) handleDeleteFunctionFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	// Build base path and sanitize the file path
	basePath := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(basePath, filePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid file path: %v", err),
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("File %q not found", filePath),
		})
		return
	}

	// Delete the file or directory
	if err := os.RemoveAll(fullPath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to delete: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleRenameFunctionFile renames/moves a file within a function directory.
func (h *Handler) handleRenameFunctionFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	// Parse request body
	var req struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	if req.OldPath == "" || req.NewPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Both oldPath and newPath are required",
		})
		return
	}

	// Build base path and sanitize both paths
	basePath := filepath.Join(h.functionsService.FunctionsDir(), name)
	oldFullPath, err := SanitizePath(basePath, req.OldPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid old path: %v", err),
		})
		return
	}

	newFullPath, err := SanitizePath(basePath, req.NewPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid new path: %v", err),
		})
		return
	}

	// Check if source exists
	if _, err := os.Stat(oldFullPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("File %q not found", req.OldPath),
		})
		return
	}

	// Create parent directories for new path if needed
	newDir := filepath.Dir(newFullPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to create directory: %v", err),
		})
		return
	}

	// Rename the file
	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to rename: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleRestartFunctions restarts the edge runtime.
func (h *Handler) handleRestartFunctions(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Edge functions not enabled. Start the server with --functions flag.",
		})
		return
	}

	if err := h.functionsService.Restart(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to restart runtime: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Runtime restarted",
	})
}

// ============================================================
// Secrets Management Handlers
// ============================================================

// handleListSecrets returns a list of all secrets (names only, no values).
func (h *Handler) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"secrets": []interface{}{},
			"enabled": false,
		})
		return
	}

	secrets, err := h.functionsService.ListSecrets()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"secrets": secrets,
		"enabled": true,
	})
}

// handleSetSecret creates or updates a secret.
func (h *Handler) handleSetSecret(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}

	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Secret name is required"})
		return
	}

	if req.Value == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Secret value is required"})
		return
	}

	if err := h.functionsService.SetSecret(req.Name, req.Value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"name":    req.Name,
		"message": "Secret set. Restart edge runtime for changes to take effect.",
	})
}

// handleDeleteSecret deletes a secret.
func (h *Handler) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.functionsService == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	if err := h.functionsService.DeleteSecret(name); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"name":    name,
		"message": "Secret deleted. Restart edge runtime for changes to take effect.",
	})
}

// Storage bucket handlers

// handleListBuckets returns a list of all storage buckets.
func (h *Handler) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	// Parse query parameters
	limit := 100
	offset := 0
	search := r.URL.Query().Get("search")

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	buckets, err := h.storageService.ListBuckets(limit, offset, search)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

// handleCreateBucket creates a new storage bucket.
func (h *Handler) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	var req storage.CreateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "message": "Invalid request body"})
		return
	}

	bucket, err := h.storageService.CreateBucket(req, "")
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bucket)
}

// handleGetBucket returns a specific bucket by ID.
func (h *Handler) handleGetBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_id", "message": "Bucket ID is required"})
		return
	}

	bucket, err := h.storageService.GetBucket(id)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bucket)
}

// handleUpdateBucket updates a bucket's configuration.
func (h *Handler) handleUpdateBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_id", "message": "Bucket ID is required"})
		return
	}

	var req storage.UpdateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "message": "Invalid request body"})
		return
	}

	bucket, err := h.storageService.UpdateBucket(id, req)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bucket)
}

// handleDeleteBucket deletes a bucket.
func (h *Handler) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_id", "message": "Bucket ID is required"})
		return
	}

	// Check for force parameter
	force := r.URL.Query().Get("force") == "true"

	if err := h.storageService.DeleteBucket(id, force); err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleEmptyBucket removes all objects from a bucket.
func (h *Handler) handleEmptyBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_id", "message": "Bucket ID is required"})
		return
	}

	if err := h.storageService.EmptyBucket(id); err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Bucket emptied successfully"})
}

// handleStorageError handles storage service errors and returns appropriate HTTP responses.
func (h *Handler) handleStorageError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	if storageErr, ok := err.(*storage.StorageError); ok {
		w.WriteHeader(storageErr.StatusCode)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   storageErr.ErrorCode,
			"message": storageErr.Message,
		})
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": "internal_error", "message": err.Error()})
}

// Storage object handlers

// handleListObjects lists objects in a bucket with optional prefix filtering.
func (h *Handler) handleListObjects(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	var req struct {
		Bucket string `json:"bucket"`
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "message": "Invalid request body"})
		return
	}

	if req.Bucket == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_bucket", "message": "Bucket name is required"})
		return
	}

	// Default limit to 100
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	listReq := storage.ListObjectsRequest{
		Prefix: req.Prefix,
		Limit:  limit,
		Offset: req.Offset,
	}

	objects, err := h.storageService.ListObjects(req.Bucket, listReq)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(objects)
}

// handleUploadObject uploads a file to a bucket via multipart form.
func (h *Handler) handleUploadObject(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	// Parse multipart form with 32MB max memory
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "message": "Failed to parse multipart form: " + err.Error()})
		return
	}

	bucket := r.FormValue("bucket")
	path := r.FormValue("path")

	if bucket == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_bucket", "message": "Bucket name is required"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_file", "message": "File is required"})
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "read_error", "message": "Failed to read file content"})
		return
	}

	// Construct full path: path + filename
	fullPath := header.Filename
	if path != "" {
		path = strings.TrimSuffix(path, "/")
		fullPath = path + "/" + header.Filename
	}

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}

	// Upload the file (upsert = true to allow overwriting)
	resp, err := h.storageService.UploadObject(bucket, fullPath, io.NopCloser(bytes.NewReader(content)), int64(len(content)), contentType, "", true)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// handleDownloadObject downloads a file from a bucket.
func (h *Handler) handleDownloadObject(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	bucket := r.URL.Query().Get("bucket")
	path := r.URL.Query().Get("path")

	if bucket == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_bucket", "message": "Bucket name is required"})
		return
	}
	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_path", "message": "Object path is required"})
		return
	}

	reader, contentType, size, err := h.storageService.GetObject(bucket, path)
	if err != nil {
		h.handleStorageError(w, err)
		return
	}
	defer reader.Close()

	// Extract filename from path for Content-Disposition
	filename := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		filename = path[idx+1:]
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

	io.Copy(w, reader)
}

// handleDeleteObjects deletes multiple files from a bucket.
func (h *Handler) handleDeleteObjects(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "service_unavailable",
			"message": "Storage service not configured",
		})
		return
	}

	var req struct {
		Bucket string   `json:"bucket"`
		Paths  []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "message": "Invalid request body"})
		return
	}

	if req.Bucket == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_bucket", "message": "Bucket name is required"})
		return
	}

	if len(req.Paths) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing_paths", "message": "At least one path is required"})
		return
	}

	// Delete each path and collect errors
	errors := h.storageService.DeleteObjects(req.Bucket, req.Paths)

	// Check if any errors occurred
	hasErrors := false
	for _, err := range errors {
		if err != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		// Return 207 Multi-Status with details
		type deleteResult struct {
			Path  string `json:"path"`
			Error string `json:"error,omitempty"`
		}
		results := make([]deleteResult, len(req.Paths))
		for i, path := range req.Paths {
			result := deleteResult{Path: path}
			if errors[i] != nil {
				result.Error = errors[i].Error()
			}
			results[i] = result
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		json.NewEncoder(w).Encode(results)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// =============================================================================
// API Docs Handlers
// =============================================================================

// APIDocsTableInfo represents a table with its columns for API documentation.
type APIDocsTableInfo struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Columns     []APIDocsColumnInfo `json:"columns"`
}

// APIDocsColumnInfo represents a column for API documentation.
type APIDocsColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`   // JavaScript type (string, number, boolean, etc.)
	Format      string `json:"format"` // PostgreSQL type (uuid, text, integer, etc.)
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// APIDocsFunctionInfo represents an RPC function for API documentation.
type APIDocsFunctionInfo struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	ReturnType  string                   `json:"return_type"`
	ReturnsSet  bool                     `json:"returns_set"`
	Arguments   []APIDocsFunctionArgInfo `json:"arguments"`
}

// APIDocsFunctionArgInfo represents a function argument for API documentation.
type APIDocsFunctionArgInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`   // JavaScript type
	Format   string `json:"format"` // PostgreSQL type
	Required bool   `json:"required"`
	Position int    `json:"position"`
}

// handleAPIDocsListTables returns all user tables with their columns for API documentation.
func (h *Handler) handleAPIDocsListTables(w http.ResponseWriter, r *http.Request) {
	// Get list of user tables (exclude internal tables)
	rows, err := h.db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table'
		AND name NOT LIKE 'auth_%'
		AND name NOT LIKE '_rls%'
		AND name NOT LIKE '_dashboard%'
		AND name NOT LIKE '_columns%'
		AND name NOT LIKE '_schema%'
		AND name NOT LIKE '_fts%'
		AND name NOT LIKE '_functions%'
		AND name NOT LIKE '_rpc%'
		AND name NOT LIKE '_table_descriptions%'
		AND name NOT LIKE '_function_descriptions%'
		AND name NOT LIKE 'storage_%'
		AND name NOT LIKE 'sqlite_%'
		AND name NOT LIKE '%_fts%'
		ORDER BY name
	`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list tables"})
		return
	}
	defer rows.Close()

	var tables []APIDocsTableInfo
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		tableInfo, err := h.getAPIDocsTableInfo(tableName)
		if err != nil {
			continue
		}
		tables = append(tables, *tableInfo)
	}

	if tables == nil {
		tables = []APIDocsTableInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

// handleAPIDocsGetTable returns detailed information about a specific table.
func (h *Handler) handleAPIDocsGetTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	tableInfo, err := h.getAPIDocsTableInfo(tableName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tableInfo)
}

// getAPIDocsTableInfo retrieves table information including columns and descriptions.
func (h *Handler) getAPIDocsTableInfo(tableName string) (*APIDocsTableInfo, error) {
	// Get table description
	var description string
	err := h.db.QueryRow(`SELECT description FROM _table_descriptions WHERE table_name = ?`, tableName).Scan(&description)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get columns from _columns metadata table with descriptions
	rows, err := h.db.Query(`
		SELECT column_name, pg_type, is_nullable, description
		FROM _columns
		WHERE table_name = ?
		ORDER BY created_at, column_name
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []APIDocsColumnInfo
	hasMetadata := false
	for rows.Next() {
		hasMetadata = true
		var colName, pgType, colDesc string
		var isNullable int
		if err := rows.Scan(&colName, &pgType, &isNullable, &colDesc); err != nil {
			continue
		}

		columns = append(columns, APIDocsColumnInfo{
			Name:        colName,
			Type:        pgTypeToJSType(pgType),
			Format:      pgType,
			Required:    isNullable == 0,
			Description: colDesc,
		})
	}

	// If no metadata, fall back to pragma table_info
	if !hasMetadata {
		pragmaRows, err := h.db.Query(fmt.Sprintf("PRAGMA table_info('%s')", tableName))
		if err != nil {
			return nil, err
		}
		defer pragmaRows.Close()

		for pragmaRows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			if err := pragmaRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				continue
			}

			// Convert SQLite type to approximate PG type
			pgType := sqliteTypeToPGType(colType)
			columns = append(columns, APIDocsColumnInfo{
				Name:        name,
				Type:        pgTypeToJSType(pgType),
				Format:      pgType,
				Required:    notNull == 1 || pk == 1,
				Description: "",
			})
		}
	}

	return &APIDocsTableInfo{
		Name:        tableName,
		Description: description,
		Columns:     columns,
	}, nil
}

// handleAPIDocsUpdateTableDescription updates a table's description.
func (h *Handler) handleAPIDocsUpdateTableDescription(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Upsert the description
	_, err := h.db.Exec(`
		INSERT INTO _table_descriptions (table_name, description, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(table_name) DO UPDATE SET description = excluded.description, updated_at = datetime('now')
	`, tableName, req.Description)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update description"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIDocsUpdateColumnDescription updates a column's description.
func (h *Handler) handleAPIDocsUpdateColumnDescription(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	columnName := chi.URLParam(r, "column")
	if tableName == "" || columnName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table and column names required"})
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Update the column description in _columns table
	result, err := h.db.Exec(`
		UPDATE _columns SET description = ?
		WHERE table_name = ? AND column_name = ?
	`, req.Description, tableName, columnName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update description"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Column not found in metadata"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIDocsListFunctions returns all RPC functions for API documentation.
func (h *Handler) handleAPIDocsListFunctions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT f.name, f.return_type, f.returns_set, COALESCE(d.description, '') as description
		FROM _rpc_functions f
		LEFT JOIN _function_descriptions d ON f.name = d.function_name
		ORDER BY f.name
	`)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list functions"})
		return
	}
	defer rows.Close()

	var functions []APIDocsFunctionInfo
	for rows.Next() {
		var name, returnType, description string
		var returnsSet int
		if err := rows.Scan(&name, &returnType, &returnsSet, &description); err != nil {
			continue
		}

		// Get function arguments
		args, err := h.getFunctionArguments(name)
		if err != nil {
			args = []APIDocsFunctionArgInfo{}
		}

		functions = append(functions, APIDocsFunctionInfo{
			Name:        name,
			Description: description,
			ReturnType:  returnType,
			ReturnsSet:  returnsSet == 1,
			Arguments:   args,
		})
	}

	if functions == nil {
		functions = []APIDocsFunctionInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(functions)
}

// handleAPIDocsGetFunction returns detailed information about a specific function.
func (h *Handler) handleAPIDocsGetFunction(w http.ResponseWriter, r *http.Request) {
	funcName := chi.URLParam(r, "name")
	if funcName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Function name required"})
		return
	}

	var name, returnType, description string
	var returnsSet int
	err := h.db.QueryRow(`
		SELECT f.name, f.return_type, f.returns_set, COALESCE(d.description, '') as description
		FROM _rpc_functions f
		LEFT JOIN _function_descriptions d ON f.name = d.function_name
		WHERE f.name = ?
	`, funcName).Scan(&name, &returnType, &returnsSet, &description)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Function not found"})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get function"})
		return
	}

	// Get function arguments
	args, err := h.getFunctionArguments(name)
	if err != nil {
		args = []APIDocsFunctionArgInfo{}
	}

	funcInfo := APIDocsFunctionInfo{
		Name:        name,
		Description: description,
		ReturnType:  returnType,
		ReturnsSet:  returnsSet == 1,
		Arguments:   args,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(funcInfo)
}

// getFunctionArguments retrieves the arguments for a function.
func (h *Handler) getFunctionArguments(funcName string) ([]APIDocsFunctionArgInfo, error) {
	rows, err := h.db.Query(`
		SELECT a.name, a.type, a.position, a.default_value
		FROM _rpc_function_args a
		JOIN _rpc_functions f ON a.function_id = f.id
		WHERE f.name = ?
		ORDER BY a.position
	`, funcName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var args []APIDocsFunctionArgInfo
	for rows.Next() {
		var name, pgType string
		var position int
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &pgType, &position, &defaultValue); err != nil {
			continue
		}

		args = append(args, APIDocsFunctionArgInfo{
			Name:     name,
			Type:     pgTypeToJSType(pgType),
			Format:   pgType,
			Required: !defaultValue.Valid,
			Position: position,
		})
	}

	return args, nil
}

// handleAPIDocsUpdateFunctionDescription updates a function's description.
func (h *Handler) handleAPIDocsUpdateFunctionDescription(w http.ResponseWriter, r *http.Request) {
	funcName := chi.URLParam(r, "name")
	if funcName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Function name required"})
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Verify function exists
	var exists int
	err := h.db.QueryRow(`SELECT 1 FROM _rpc_functions WHERE name = ?`, funcName).Scan(&exists)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Function not found"})
		return
	}

	// Upsert the description
	_, err = h.db.Exec(`
		INSERT INTO _function_descriptions (function_name, description, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(function_name) DO UPDATE SET description = excluded.description, updated_at = datetime('now')
	`, funcName, req.Description)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update description"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// pgTypeToJSType converts a PostgreSQL type to a JavaScript type for API docs.
func pgTypeToJSType(pgType string) string {
	switch pgType {
	case "uuid", "text", "timestamptz", "bytea":
		return "string"
	case "integer", "numeric":
		return "number"
	case "boolean":
		return "boolean"
	case "jsonb":
		return "object"
	default:
		return "string"
	}
}

// sqliteTypeToPGType converts a SQLite type to an approximate PostgreSQL type.
func sqliteTypeToPGType(sqliteType string) string {
	sqliteType = strings.ToUpper(sqliteType)
	switch {
	case strings.Contains(sqliteType, "INT"):
		return "integer"
	case strings.Contains(sqliteType, "TEXT"), strings.Contains(sqliteType, "CHAR"), strings.Contains(sqliteType, "CLOB"):
		return "text"
	case strings.Contains(sqliteType, "BLOB"):
		return "bytea"
	case strings.Contains(sqliteType, "REAL"), strings.Contains(sqliteType, "FLOA"), strings.Contains(sqliteType, "DOUB"):
		return "numeric"
	case strings.Contains(sqliteType, "BOOL"):
		return "boolean"
	default:
		return "text"
	}
}

// handleRealtimeStats returns realtime connection statistics
func (h *Handler) handleRealtimeStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.realtimeService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "realtime not enabled",
		})
		return
	}

	stats := h.realtimeService.Stats()
	json.NewEncoder(w).Encode(stats)
}
