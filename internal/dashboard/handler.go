package dashboard

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
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
