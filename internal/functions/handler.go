package functions

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

// Handler handles HTTP requests for the functions API.
type Handler struct {
	service   *Service
	proxy     *FunctionsProxy
	jwtSecret []byte
}

// NewHandler creates a new functions handler.
func NewHandler(service *Service, jwtSecret string) *Handler {
	return &Handler{
		service:   service,
		proxy:     NewFunctionsProxy(service.RuntimePort()),
		jwtSecret: []byte(jwtSecret),
	}
}

// RegisterRoutes registers the functions API routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Invocation routes - all methods supported
	r.HandleFunc("/{name}", h.handleInvoke)
	r.HandleFunc("/{name}/*", h.handleInvoke)

	// CORS preflight
	r.Options("/{name}", h.handleOptions)
	r.Options("/{name}/*", h.handleOptions)
}

// handleInvoke handles function invocation requests.
func (h *Handler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		WriteError(w, "FunctionsHttpError", "Function name is required", http.StatusBadRequest)
		return
	}

	// Check if runtime is healthy
	if !h.service.IsRunning() {
		WriteError(w, "FunctionsRelayError", "Edge runtime is not running", http.StatusBadGateway)
		return
	}

	// Check if function exists
	if !h.service.FunctionExists(name) {
		WriteError(w, "FunctionsHttpError", "Function not found", http.StatusNotFound)
		return
	}

	// Get function metadata to check JWT verification settings
	meta, err := h.service.GetMetadata(name)
	if err != nil {
		// On error, default to requiring JWT
		meta = &FunctionMetadata{Name: name, VerifyJWT: true}
	}

	// Check JWT if required (configurable per-function, default: true)
	if meta.VerifyJWT {
		if err := h.validateJWT(r); err != nil {
			WriteError(w, "FunctionsHttpError", "Invalid authorization: "+err.Error(), http.StatusUnauthorized)
			return
		}
	}

	// Proxy to edge runtime
	h.proxy.ServeHTTP(w, r)
}

// handleOptions handles CORS preflight requests.
func (h *Handler) handleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.WriteHeader(http.StatusNoContent)
}

// validateJWT validates the JWT token in the Authorization header.
// Returns an error if the token is invalid or missing.
func (h *Handler) validateJWT(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return jwt.ErrTokenMalformed // Token required when JWT verification is enabled
	}

	// Extract token from "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return jwt.ErrTokenMalformed
	}

	token := parts[1]

	// Parse and validate token
	_, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.jwtSecret, nil
	})

	return err
}

// DashboardHandler handles dashboard API requests for functions management.
type DashboardHandler struct {
	service *Service
}

// NewDashboardHandler creates a new dashboard handler for functions.
func NewDashboardHandler(service *Service) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// RegisterRoutes registers the dashboard API routes.
func (h *DashboardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/functions", h.handleListFunctions)
	r.Get("/functions/{name}", h.handleGetFunction)
	r.Post("/functions/{name}", h.handleCreateFunction)
	r.Delete("/functions/{name}", h.handleDeleteFunction)
	r.Get("/functions/status", h.handleGetStatus)
}

// handleListFunctions returns a list of all functions.
func (h *DashboardHandler) handleListFunctions(w http.ResponseWriter, r *http.Request) {
	functions, err := h.service.ListFunctions()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Add status and metadata to each function
	for i := range functions {
		if h.service.IsRunning() {
			functions[i].Status = "ready"
		} else {
			functions[i].Status = "unavailable"
		}

		// Get per-function metadata
		meta, err := h.service.GetMetadata(functions[i].Name)
		if err == nil {
			functions[i].VerifyJWT = meta.VerifyJWT
		} else {
			functions[i].VerifyJWT = true // Default to true
		}
	}

	json.NewEncoder(w).Encode(functions)
}

// handleGetFunction returns details about a specific function.
func (h *DashboardHandler) handleGetFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	fn, err := h.service.GetFunction(name)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	if h.service.IsRunning() {
		fn.Status = "ready"
	} else {
		fn.Status = "unavailable"
	}

	json.NewEncoder(w).Encode(fn)
}

// handleCreateFunction creates a new function from template.
func (h *DashboardHandler) handleCreateFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Parse request body for template type
	var req struct {
		Template string `json:"template"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Template = "default"
	}
	if req.Template == "" {
		req.Template = "default"
	}

	if err := h.service.CreateFunction(name, req.Template); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	fn, _ := h.service.GetFunction(name)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fn)
}

// handleDeleteFunction deletes a function.
func (h *DashboardHandler) handleDeleteFunction(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := h.service.DeleteFunction(name); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetStatus returns the status of the edge runtime.
func (h *DashboardHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := "stopped"
	if h.service.IsRunning() {
		status = "running"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        status,
		"runtime_port":  h.service.RuntimePort(),
		"functions_dir": h.service.FunctionsDir(),
	})
}
