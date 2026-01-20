// internal/rpc/handler.go
package rpc

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/rls"
)

// Handler handles RPC HTTP requests.
type Handler struct {
	executor *Executor
	store    *Store
}

// NewHandler creates a new RPC handler.
func NewHandler(executor *Executor, store *Store) *Handler {
	return &Handler{
		executor: executor,
		store:    store,
	}
}

// HandleRPC handles POST /rest/v1/rpc/{name}.
func (h *Handler) HandleRPC(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "PGRST000", "Function name required")
		return
	}

	// Parse request body for arguments
	var args map[string]interface{}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			h.writeError(w, http.StatusBadRequest, "PGRST000", "Invalid JSON body")
			return
		}
	}

	// Get auth context from request
	authCtx := getAuthContextFromRequest(r)

	// Execute the function
	result, err := h.executor.Execute(name, args, authCtx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, "PGRST202", "Function not found: "+name)
			return
		}
		if strings.Contains(err.Error(), "missing required argument") {
			h.writeError(w, http.StatusBadRequest, "42883", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "PGRST500", err.Error())
		return
	}

	// Check Accept header for response format
	accept := r.Header.Get("Accept")
	wantSingle := strings.Contains(accept, "application/vnd.pgrst.object+json")

	// Check Prefer header for minimal return
	prefer := r.Header.Get("Prefer")
	if strings.Contains(prefer, "return=minimal") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Handle single object request
	if wantSingle && result.IsSet {
		rows, ok := result.Data.([]map[string]interface{})
		if !ok || len(rows) != 1 {
			h.writeError(w, http.StatusNotAcceptable, "PGRST116", "JSON object requested, multiple (or no) rows returned")
			return
		}
		json.NewEncoder(w).Encode(rows[0])
		return
	}

	json.NewEncoder(w).Encode(result.Data)
}

// writeError writes a PostgREST-compatible error response.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
		"details": nil,
		"hint":    nil,
	})
}

// getAuthContextFromRequest extracts auth context from request.
func getAuthContextFromRequest(r *http.Request) *rls.AuthContext {
	ctx := &rls.AuthContext{}

	// Check for service_role API key - bypasses RLS
	apiKeyRole, _ := r.Context().Value("apikey_role").(string)
	if apiKeyRole == "service_role" {
		ctx.BypassRLS = true
		ctx.Role = "service_role"
		return ctx
	}

	// Extract user claims from Bearer token (set by auth middleware)
	// The middleware stores *jwt.MapClaims in context
	if claims, ok := r.Context().Value("claims").(*jwt.MapClaims); ok && claims != nil {
		if sub, ok := (*claims)["sub"].(string); ok {
			ctx.UserID = sub
		}
		if email, ok := (*claims)["email"].(string); ok {
			ctx.Email = email
		}
		if role, ok := (*claims)["role"].(string); ok {
			ctx.Role = role
		}
		// Copy all claims for auth.jwt() access
		ctx.Claims = make(map[string]any)
		for k, v := range *claims {
			ctx.Claims[k] = v
		}
	}

	if apiKeyRole != "" {
		ctx.Role = apiKeyRole
	}

	return ctx
}
