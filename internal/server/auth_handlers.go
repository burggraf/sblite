// internal/server/auth_handlers.go
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type SignupRequest struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Data     map[string]any `json:"data,omitempty"`
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

	// Check if this is an anonymous signup (no email AND no password)
	if req.Email == "" && req.Password == "" {
		s.handleAnonymousSignup(w, r, req.Data)
		return
	}

	// Regular signup - require both email and password
	if req.Email == "" || req.Password == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email and password are required")
		return
	}

	if len(req.Password) < 6 {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
		return
	}

	// Check if email confirmation is required
	requireConfirmation := s.dashboardHandler.GetRequireEmailConfirmation()

	// Create user (auto-confirm only if confirmation not required)
	user, err := s.authService.CreateUserWithOptions(req.Email, req.Password, req.Data, !requireConfirmation)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			s.writeError(w, http.StatusBadRequest, "user_already_exists", "User already registered")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create user")
		return
	}

	if requireConfirmation {
		// Generate confirmation token and send email
		token, err := s.authService.GenerateConfirmationToken(user.ID)
		if err != nil {
			slog.Error("failed to generate confirmation token", "error", err)
		} else {
			s.emailService.SendConfirmation(r.Context(), user.ID, user.Email, token)
		}

		// Return response indicating confirmation needed (no tokens)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":                          user.ID,
			"email":                       user.Email,
			"confirmation_sent_at":        time.Now().UTC().Format(time.RFC3339),
			"email_confirmation_required": true,
		})
		return
	}

	// Create session and generate tokens (when confirmation not required)
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
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleAnonymousSignup(w http.ResponseWriter, r *http.Request, userMetadata map[string]any) {
	user, err := s.authService.CreateAnonymousUser(userMetadata)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to create anonymous user")
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

	s.authService.UpdateLastSignIn(user.ID)

	// Build user response with null email
	userResponse := map[string]any{
		"id":            user.ID,
		"email":         nil,
		"role":          user.Role,
		"is_anonymous":  true,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User:         userResponse,
	}

	w.Header().Set("Content-Type", "application/json")
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

	// Check if email confirmation is required and user hasn't confirmed
	requireConfirmation := s.dashboardHandler.GetRequireEmailConfirmation()
	if requireConfirmation && user.EmailConfirmedAt == nil {
		s.writeError(w, http.StatusForbidden, "email_not_confirmed",
			"Email not confirmed. Please check your email for a confirmation link.")
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

	// Build user response
	userResponse := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"is_anonymous":  user.IsAnonymous,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	// Set email to null for anonymous users
	if user.IsAnonymous {
		userResponse["email"] = nil
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User:         userResponse,
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

	// Build user response
	userResponse := map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"role":          user.Role,
		"is_anonymous":  user.IsAnonymous,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	// Set email to null for anonymous users
	if user.IsAnonymous {
		userResponse["email"] = nil
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		ExpiresIn:    3600,
		RefreshToken: refreshToken,
		User:         userResponse,
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
		"is_anonymous":  user.IsAnonymous,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	// Set email to null for anonymous users
	if user.IsAnonymous {
		response["email"] = nil
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
	Email    string         `json:"email,omitempty"`
	Password string         `json:"password,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
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

	// Check for anonymous user conversion (email + password provided for anonymous user)
	if user.IsAnonymous && req.Email != "" && req.Password != "" {
		// Validate password length
		if len(req.Password) < 6 {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
			return
		}

		// Convert anonymous user to regular user
		if err := s.authService.ConvertAnonymousUser(user.ID, req.Email, req.Password); err != nil {
			if strings.Contains(err.Error(), "email already in use") {
				s.writeError(w, http.StatusBadRequest, "email_exists", "Email address is already in use")
				return
			}
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to convert anonymous user")
			return
		}

		// Update metadata if provided
		if req.Data != nil {
			s.authService.UpdateUserMetadata(user.ID, req.Data)
		}

		// Refetch user to get updated data (user was just converted, so this should succeed)
		user, _ = s.authService.GetUserByID(user.ID)

		response := map[string]any{
			"id":            user.ID,
			"email":         user.Email,
			"role":          user.Role,
			"is_anonymous":  user.IsAnonymous,
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
			"app_metadata":  user.AppMetadata,
			"user_metadata": user.UserMetadata,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.Data != nil {
		if err := s.authService.UpdateUserMetadata(user.ID, req.Data); err != nil {
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
		"is_anonymous":  user.IsAnonymous,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	// Set email to null for anonymous users
	if user.IsAnonymous {
		response["email"] = nil
	}

	json.NewEncoder(w).Encode(response)
}

type LogoutRequest struct {
	Scope string `json:"scope"` // local, global, others
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r)
	if claims == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "Claims not found in context")
		return
	}

	sessionID, ok := (*claims)["session_id"].(string)
	if !ok || sessionID == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_session", "Session ID not found in token")
		return
	}

	userID, ok := (*claims)["sub"].(string)
	if !ok || userID == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_token", "User ID not found in token")
		return
	}

	// Parse scope from query parameter or request body (default to "local")
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		var req LogoutRequest
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&req)
		}
		scope = req.Scope
	}
	if scope == "" {
		scope = "local"
	}

	var err error
	switch scope {
	case "global":
		err = s.authService.RevokeAllUserSessions(userID)
	case "others":
		err = s.authService.RevokeOtherSessions(userID, sessionID)
	case "local":
		err = s.authService.RevokeSession(sessionID)
	default:
		s.writeError(w, http.StatusBadRequest, "invalid_scope", "Scope must be local, global, or others")
		return
	}

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to revoke session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type RecoverRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	var req RecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate token and send email
	token, err := s.authService.GenerateRecoveryToken(req.Email)
	if err == nil && token != "" {
		user, _ := s.authService.GetUserByEmail(req.Email)
		if user != nil {
			s.emailService.SendRecovery(r.Context(), user.ID, req.Email, token)
		}
	}

	// Always return success to prevent email enumeration
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email exists, a recovery link has been sent",
	})
}

type VerifyRequest struct {
	Type     string `json:"type"`     // "signup" or "recovery"
	Token    string `json:"token"`
	Password string `json:"password"` // Required for recovery type
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	var token, verifyType, password string

	if r.Method == "GET" {
		token = r.URL.Query().Get("token")
		verifyType = r.URL.Query().Get("type")
		password = r.URL.Query().Get("password")
	} else {
		var req VerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
			return
		}
		token = req.Token
		verifyType = req.Type
		password = req.Password
	}

	if token == "" {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Token is required")
		return
	}

	switch verifyType {
	case "signup", "email", "":
		user, err := s.authService.VerifyEmail(token)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Email verified successfully",
			"user":    user,
		})
	case "recovery":
		if password == "" {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password is required")
			return
		}
		if len(password) < 6 {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
			return
		}
		user, err := s.authService.ResetPassword(token, password)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Password reset successfully",
			"user":    user,
		})
	default:
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid verification type")
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	// mailer_autoconfirm is the inverse of require_email_confirmation
	requireConfirmation := s.dashboardHandler.GetRequireEmailConfirmation()

	settings := map[string]any{
		"external": map[string]bool{
			"anonymous": true, // Always enabled
			"email":     true, // Always enabled
			"phone":     false,
			"google":    s.oauthRegistry != nil && s.oauthRegistry.IsEnabled("google"),
			"github":    s.oauthRegistry != nil && s.oauthRegistry.IsEnabled("github"),
			"facebook":  false,
			"twitter":   false,
			"apple":     false,
			"discord":   false,
			"twitch":    false,
		},
		"disable_signup":     false,
		"mailer_autoconfirm": !requireConfirmation,
		"phone_autoconfirm":  false,
		"sms_provider":       "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

type MagicLinkRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleMagicLink(w http.ResponseWriter, r *http.Request) {
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate magic link token
	token, err := s.authService.GenerateMagicLinkToken(req.Email)
	if err != nil {
		// Don't reveal if user exists - always return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "If the email exists, a magic link has been sent",
		})
		return
	}

	// Send magic link email
	if err := s.emailService.SendMagicLink(r.Context(), req.Email, token); err != nil {
		// Log error but don't expose to user
		slog.Error("failed to send magic link email", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If the email exists, a magic link has been sent",
	})
}

type InviteRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	// Check if user has admin privileges (service_role)
	claims := GetClaimsFromContext(r)
	if claims == nil {
		s.writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	role, _ := (*claims)["role"].(string)
	if role != "service_role" {
		s.writeError(w, http.StatusForbidden, "forbidden", "Admin access required")
		return
	}

	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	// Generate invite token
	token, err := s.authService.GenerateInviteToken(req.Email)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "user_already_exists", "User already registered")
		return
	}

	// Send invite email
	if err := s.emailService.SendInvite(r.Context(), req.Email, token); err != nil {
		slog.Error("failed to send invite email", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Invitation sent",
	})
}

type ResendRequest struct {
	Type  string `json:"type"`  // confirmation, recovery
	Email string `json:"email"`
}

func (s *Server) handleResend(w http.ResponseWriter, r *http.Request) {
	var req ResendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Email == "" {
		s.writeError(w, http.StatusBadRequest, "validation_failed", "Email is required")
		return
	}

	switch req.Type {
	case "confirmation", "signup":
		user, err := s.authService.GetUserByEmail(req.Email)
		if err == nil && user.EmailConfirmedAt == nil {
			token, _ := s.authService.GenerateConfirmationToken(user.ID)
			s.emailService.SendConfirmation(r.Context(), user.ID, req.Email, token)
		}
	case "recovery":
		token, _ := s.authService.GenerateRecoveryToken(req.Email)
		if token != "" {
			user, _ := s.authService.GetUserByEmail(req.Email)
			if user != nil {
				s.emailService.SendRecovery(r.Context(), user.ID, req.Email, token)
			}
		}
	}

	// Always return success to prevent enumeration
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "If applicable, an email has been sent",
	})
}
