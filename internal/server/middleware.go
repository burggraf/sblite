// internal/server/middleware.go
package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/auth"
)

type contextKey string

const (
	UserContextKey   contextKey = "user"
	ClaimsContextKey contextKey = "claims"
)

// contextKeyStr is used for storing claims in context for access by rest handler
// Using a package-level string avoids type mismatch issues between packages
const contextKeyStr = "claims"

// apiKeyRoleContextKey is used for storing API key role in context
// Using a string key allows access from rest package without circular imports
const apiKeyRoleContextKey = "apikey_role"

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, http.StatusUnauthorized, "no_authorization", "Authorization header required")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			s.writeError(w, http.StatusUnauthorized, "invalid_authorization", "Invalid authorization header format")
			return
		}

		claims, err := s.authService.ValidateAccessToken(parts[1])
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}

		// Check if this is a service_role token (admin API key)
		// Service role tokens don't have a real user, so skip user lookup
		role, _ := (*claims)["role"].(string)
		if role == "service_role" {
			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
			ctx = context.WithValue(ctx, contextKeyStr, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userID := (*claims)["sub"].(string)
		user, err := s.authService.GetUserByID(userID)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "user_not_found", "User not found")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, ClaimsContextKey, claims)
		ctx = context.WithValue(ctx, contextKeyStr, claims) // Also store with string key for rest handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(r *http.Request) *auth.User {
	user, _ := r.Context().Value(UserContextKey).(*auth.User)
	return user
}

func GetClaimsFromContext(r *http.Request) *jwt.MapClaims {
	claims, _ := r.Context().Value(ClaimsContextKey).(*jwt.MapClaims)
	return claims
}

// optionalAuthMiddleware extracts JWT claims if present, but doesn't block unauthenticated requests.
// This allows RLS to work when users are authenticated, but also allows public table access.
func (s *Server) optionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := s.authService.ValidateAccessToken(tokenString)
			if err == nil {
				ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
				ctx = context.WithValue(ctx, contextKeyStr, claims) // Also store with string key for rest handler
				r = r.WithContext(ctx)
			}
			// If token is invalid, just continue without claims - don't block
		}
		next.ServeHTTP(w, r)
	})
}

// apiKeyMiddleware validates the apikey header and extracts the role.
// This middleware must be applied before optionalAuthMiddleware.
func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("apikey")
		if apiKey == "" {
			s.writeError(w, http.StatusUnauthorized, "no_api_key", "API key is required")
			return
		}

		role, err := s.authService.ValidateAPIKey(apiKey)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "invalid_api_key", "Invalid API key")
			return
		}

		// Store role in context for use by rest handler
		ctx := context.WithValue(r.Context(), apiKeyRoleContextKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAPIKeyRoleFromContext returns the API key role from the request context.
// Returns empty string if no API key role is present.
func GetAPIKeyRoleFromContext(r *http.Request) string {
	role, _ := r.Context().Value(apiKeyRoleContextKey).(string)
	return role
}
