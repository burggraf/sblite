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

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken  string         `json:"access_token"`
	TokenType    string         `json:"token_type"`
	ExpiresIn    int            `json:"expires_in"`
	RefreshToken string         `json:"refresh_token"`
	User         map[string]any `json:"user"`
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	grantType := r.URL.Query().Get("grant_type")

	switch grantType {
	case "password":
		s.handlePasswordGrant(w, r)
	case "refresh_token":
		s.handleRefreshGrant(w, r)
	default:
		s.writeError(w, http.StatusBadRequest, "invalid_grant", "Unsupported grant type")
	}
}

func (s *Server) handlePasswordGrant(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := s.authService.GetUserByEmail(req.Email)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if !s.authService.ValidatePassword(user, req.Password) {
		s.writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	session, refreshToken, err := s.authService.CreateSession(user)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create session")
		return
	}

	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	// Update last sign in
	s.authService.UpdateLastSignIn(user.ID)

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User: map[string]any{
			"id":            user.ID,
			"email":         user.Email,
			"role":          user.Role,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleRefreshGrant(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, session, refreshToken, err := s.authService.RefreshSession(req.RefreshToken)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid_grant", "Invalid refresh token")
		return
	}

	accessToken, err := s.authService.GenerateAccessToken(user, session.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User: map[string]any{
			"id":            user.ID,
			"email":         user.Email,
			"role":          user.Role,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "User not found in context")
		return
	}

	response := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	if user.EmailConfirmedAt != nil {
		response["email_confirmed_at"] = user.EmailConfirmedAt
	}
	if user.LastSignInAt != nil {
		response["last_sign_in_at"] = user.LastSignInAt
	}

	json.NewEncoder(w).Encode(response)
}

type UpdateUserRequest struct {
	Email        string         `json:"email,omitempty"`
	Password     string         `json:"password,omitempty"`
	UserMetadata map[string]any `json:"user_metadata,omitempty"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "User not found in context")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.UserMetadata != nil {
		if err := s.authService.UpdateUserMetadata(user.ID, req.UserMetadata); err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to update user")
			return
		}
	}

	if req.Password != "" {
		if len(req.Password) < 6 {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
			return
		}
		if err := s.authService.UpdatePassword(user.ID, req.Password); err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to update password")
			return
		}
	}

	// Refetch user to get updated data
	user, _ = s.authService.GetUserByID(user.ID)

	response := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	json.NewEncoder(w).Encode(response)
}
