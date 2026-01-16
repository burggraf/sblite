// internal/server/auth_handlers.go
package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email and password are required")
		return
	}

	if len(req.Password) < 6 {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
		return
	}

	user, err := s.authService.CreateUser(req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.writeError(w, http.StatusBadRequest, "user_already_exists", "User already registered")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create user")
		return
	}

	response := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
