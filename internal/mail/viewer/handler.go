// Package viewer provides a web UI for viewing caught emails.
package viewer

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/mail"
)

//go:embed static/index.html
var staticFS embed.FS

// Handler serves the mail viewer web UI.
type Handler struct {
	catcher *mail.CatchMailer
}

// NewHandler creates a new Handler.
func NewHandler(catcher *mail.CatchMailer) *Handler {
	return &Handler{catcher: catcher}
}

// RegisterRoutes registers the mail viewer routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.serveUI)
	r.Get("/api/emails", h.listEmails)
	r.Get("/api/emails/{id}", h.getEmail)
	r.Delete("/api/emails/{id}", h.deleteEmail)
	r.Delete("/api/emails", h.clearAll)
}

func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func (h *Handler) listEmails(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

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

	emails, err := h.catcher.ListEmails(limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emails)
}

func (h *Handler) getEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	email, err := h.catcher.GetEmail(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(email)
}

func (h *Handler) deleteEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.catcher.DeleteEmail(id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clearAll(w http.ResponseWriter, r *http.Request) {
	if err := h.catcher.ClearAll(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
