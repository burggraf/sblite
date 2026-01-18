# Anonymous Sign-In Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement Supabase-compatible anonymous sign-in allowing users to authenticate without credentials and convert to permanent accounts.

**Architecture:** Anonymous users are stored in `auth_users` with `is_anonymous=1`, no email, and no password. They receive full `authenticated` role JWTs with `is_anonymous: true` claim. Conversion to permanent user happens via `updateUser` with email/password or OAuth linking.

**Tech Stack:** Go, SQLite, JWT (HS256), @supabase/supabase-js client

---

## Task 1: Add `is_anonymous` Column to Database Schema

**Files:**
- Modify: `internal/db/migrations.go:7-28`

**Step 1: Add is_anonymous column to authSchema**

Edit `internal/db/migrations.go` to add the column after `is_super_admin`:

```go
const authSchema = `
CREATE TABLE IF NOT EXISTS auth_users (
    id                    TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    email                 TEXT UNIQUE,
    encrypted_password    TEXT,
    email_confirmed_at    TEXT,
    invited_at            TEXT,
    confirmation_token    TEXT,
    confirmation_sent_at  TEXT,
    recovery_token        TEXT,
    recovery_sent_at      TEXT,
    email_change_token    TEXT,
    email_change          TEXT,
    last_sign_in_at       TEXT,
    raw_app_meta_data     TEXT DEFAULT '{}' CHECK (json_valid(raw_app_meta_data)),
    raw_user_meta_data    TEXT DEFAULT '{}' CHECK (json_valid(raw_user_meta_data)),
    is_super_admin        INTEGER DEFAULT 0,
    is_anonymous          INTEGER DEFAULT 0,
    role                  TEXT DEFAULT 'authenticated',
    created_at            TEXT DEFAULT (datetime('now')),
    updated_at            TEXT DEFAULT (datetime('now')),
    banned_until          TEXT,
    deleted_at            TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_users_email ON auth_users(email);
CREATE INDEX IF NOT EXISTS idx_auth_users_confirmation_token ON auth_users(confirmation_token);
CREATE INDEX IF NOT EXISTS idx_auth_users_recovery_token ON auth_users(recovery_token);
CREATE INDEX IF NOT EXISTS idx_auth_users_is_anonymous ON auth_users(is_anonymous);
`
```

**Step 2: Run tests to verify schema change doesn't break existing functionality**

Run: `go test ./internal/db/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/db/migrations.go
git commit -m "feat(db): add is_anonymous column to auth_users"
```

---

## Task 2: Add IsAnonymous Field to User Struct

**Files:**
- Modify: `internal/auth/user.go:18-29`
- Modify: `internal/auth/user.go:91-134` (GetUserByID)

**Step 1: Add IsAnonymous to User struct**

Edit `internal/auth/user.go`:

```go
type User struct {
	ID                string         `json:"id"`
	Email             string         `json:"email"`
	EncryptedPassword string         `json:"-"`
	EmailConfirmedAt  *time.Time     `json:"email_confirmed_at,omitempty"`
	LastSignInAt      *time.Time     `json:"last_sign_in_at,omitempty"`
	AppMetadata       map[string]any `json:"app_metadata"`
	UserMetadata      map[string]any `json:"user_metadata"`
	Role              string         `json:"role"`
	IsAnonymous       bool           `json:"is_anonymous"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}
```

**Step 2: Update GetUserByID to read is_anonymous**

Modify the SQL query and Scan in GetUserByID:

```go
func (s *Service) GetUserByID(id string) (*User, error) {
	var user User
	var createdAt, updatedAt string
	var emailConfirmedAt, lastSignInAt sql.NullString
	var rawAppMetaData, rawUserMetaData string
	var isAnonymous int

	err := s.db.QueryRow(`
		SELECT id, email, encrypted_password, email_confirmed_at, last_sign_in_at,
		       role, created_at, updated_at, raw_app_meta_data, raw_user_meta_data, is_anonymous
		FROM auth_users WHERE id = ? AND deleted_at IS NULL
	`, id).Scan(&user.ID, &user.Email, &user.EncryptedPassword, &emailConfirmedAt,
		&lastSignInAt, &user.Role, &createdAt, &updatedAt, &rawAppMetaData, &rawUserMetaData, &isAnonymous)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.IsAnonymous = isAnonymous == 1
	user.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	user.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	// ... rest of function unchanged
```

**Step 3: Run tests**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/auth/user.go
git commit -m "feat(auth): add IsAnonymous field to User struct"
```

---

## Task 3: Add `is_anonymous` Claim to JWT Generation

**Files:**
- Modify: `internal/auth/jwt.go:32-54` (GenerateAccessToken)
- Test: `internal/auth/jwt_test.go`

**Step 1: Write failing test for is_anonymous claim**

Create or update `internal/auth/jwt_test.go`:

```go
func TestGenerateAccessTokenAnonymousClaim(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	// Create a regular user
	user := &User{
		ID:          "test-user-id",
		Email:       "test@example.com",
		Role:        "authenticated",
		IsAnonymous: false,
		UserMetadata: map[string]any{},
		AppMetadata:  map[string]any{"provider": "email", "providers": []string{"email"}},
	}

	token, err := service.GenerateAccessToken(user, "session-123")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Parse and verify claims
	claims, err := service.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	isAnonymous, ok := (*claims)["is_anonymous"].(bool)
	if !ok {
		t.Fatal("expected is_anonymous claim to be boolean")
	}
	if isAnonymous != false {
		t.Errorf("expected is_anonymous to be false, got %v", isAnonymous)
	}
}

func TestGenerateAccessTokenAnonymousClaimTrue(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	// Create an anonymous user
	user := &User{
		ID:          "anon-user-id",
		Email:       "",
		Role:        "authenticated",
		IsAnonymous: true,
		UserMetadata: map[string]any{},
		AppMetadata:  map[string]any{"provider": "anonymous", "providers": []string{"anonymous"}},
	}

	token, err := service.GenerateAccessToken(user, "session-456")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := service.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	isAnonymous, ok := (*claims)["is_anonymous"].(bool)
	if !ok {
		t.Fatal("expected is_anonymous claim to be boolean")
	}
	if isAnonymous != true {
		t.Errorf("expected is_anonymous to be true, got %v", isAnonymous)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestGenerateAccessTokenAnonymous -v`
Expected: FAIL (is_anonymous claim not present)

**Step 3: Update GenerateAccessToken to include is_anonymous claim**

Edit `internal/auth/jwt.go`:

```go
func (s *Service) GenerateAccessToken(user *User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"aud":          "authenticated",
		"exp":          now.Add(time.Duration(AccessTokenExpiry) * time.Second).Unix(),
		"iat":          now.Unix(),
		"iss":          "http://localhost:8080/auth/v1",
		"sub":          user.ID,
		"email":        user.Email,
		"phone":        "",
		"role":         user.Role,
		"aal":          "aal1",
		"session_id":   sessionID,
		"is_anonymous": user.IsAnonymous,
		"app_metadata":  user.AppMetadata,
		"user_metadata": user.UserMetadata,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/jwt.go internal/auth/jwt_test.go
git commit -m "feat(auth): add is_anonymous claim to JWT"
```

---

## Task 4: Create CreateAnonymousUser Method

**Files:**
- Modify: `internal/auth/user.go`
- Test: `internal/auth/user_test.go`

**Step 1: Write failing test for CreateAnonymousUser**

Add to `internal/auth/user_test.go`:

```go
func TestCreateAnonymousUser(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database, "test-secret-key-min-32-characters")

	userMeta := map[string]any{"theme": "dark"}
	user, err := service.CreateAnonymousUser(userMeta)
	if err != nil {
		t.Fatalf("failed to create anonymous user: %v", err)
	}

	// Verify user properties
	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	if user.Email != "" {
		t.Errorf("expected email to be empty, got %s", user.Email)
	}
	if !user.IsAnonymous {
		t.Error("expected IsAnonymous to be true")
	}
	if user.Role != "authenticated" {
		t.Errorf("expected role to be 'authenticated', got %s", user.Role)
	}

	// Check app_metadata
	provider, ok := user.AppMetadata["provider"]
	if !ok || provider != "anonymous" {
		t.Errorf("expected app_metadata.provider to be 'anonymous', got %v", provider)
	}

	// Check user_metadata
	theme, ok := user.UserMetadata["theme"]
	if !ok || theme != "dark" {
		t.Errorf("expected user_metadata.theme to be 'dark', got %v", theme)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestCreateAnonymousUser -v`
Expected: FAIL (method doesn't exist)

**Step 3: Implement CreateAnonymousUser**

Add to `internal/auth/user.go`:

```go
// CreateAnonymousUser creates a new anonymous user (no email/password).
func (s *Service) CreateAnonymousUser(userMetadata map[string]any) (*User, error) {
	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Marshal user metadata to JSON
	userMetaJSON := "{}"
	if userMetadata != nil {
		metaBytes, err := json.Marshal(userMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal user metadata: %w", err)
		}
		userMetaJSON = string(metaBytes)
	}

	// Anonymous users have no email, no password, is_anonymous=1
	_, err := s.db.Exec(`
		INSERT INTO auth_users (id, email, encrypted_password, email_confirmed_at, is_anonymous, raw_app_meta_data, raw_user_meta_data, created_at, updated_at)
		VALUES (?, NULL, '', ?, 1, '{"provider":"anonymous","providers":["anonymous"]}', ?, ?, ?)
	`, id, now, userMetaJSON, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create anonymous user: %w", err)
	}

	return s.GetUserByID(id)
}
```

**Step 4: Run tests**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/user.go internal/auth/user_test.go
git commit -m "feat(auth): add CreateAnonymousUser method"
```

---

## Task 5: Update handleSignup for Anonymous Sign-In

**Files:**
- Modify: `internal/server/auth_handlers.go:23-107`
- Test: `internal/server/auth_handlers_test.go`

**Step 1: Write failing test for anonymous signup**

Add to `internal/server/auth_handlers_test.go`:

```go
func TestHandleSignupAnonymous(t *testing.T) {
	// Setup test server
	srv := setupTestServer(t)

	// Empty body = anonymous signup
	req := httptest.NewRequest("POST", "/auth/v1/signup", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify response has tokens
	if resp["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if resp["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}

	// Verify user is anonymous
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user in response")
	}
	if user["is_anonymous"] != true {
		t.Errorf("expected is_anonymous to be true, got %v", user["is_anonymous"])
	}
	if user["email"] != nil {
		t.Errorf("expected email to be null, got %v", user["email"])
	}
}

func TestHandleSignupAnonymousWithMetadata(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"data": {"theme": "dark"}}`
	req := httptest.NewRequest("POST", "/auth/v1/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	user := resp["user"].(map[string]any)
	userMeta := user["user_metadata"].(map[string]any)
	if userMeta["theme"] != "dark" {
		t.Errorf("expected theme to be 'dark', got %v", userMeta["theme"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -run TestHandleSignupAnonymous -v`
Expected: FAIL (returns 400 for missing email/password)

**Step 3: Update handleSignup to handle anonymous requests**

Edit `internal/server/auth_handlers.go`:

```go
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

	// ... rest of existing handleSignup code unchanged
```

**Step 4: Add handleAnonymousSignup helper**

Add to `internal/server/auth_handlers.go`:

```go
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
```

**Step 5: Run tests**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/server/auth_handlers.go internal/server/auth_handlers_test.go
git commit -m "feat(auth): add anonymous signup handler"
```

---

## Task 6: Update User Response to Include is_anonymous

**Files:**
- Modify: `internal/server/auth_handlers.go` (handleGetUser, handleUpdateUser, handlePasswordGrant, handleRefreshGrant)

**Step 1: Update handleGetUser response**

Edit `internal/server/auth_handlers.go` in handleGetUser:

```go
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

	// ... rest unchanged
```

**Step 2: Update token response functions similarly**

Update handlePasswordGrant and handleRefreshGrant user responses to include `is_anonymous`.

**Step 3: Run all auth handler tests**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/auth_handlers.go
git commit -m "feat(auth): include is_anonymous in user responses"
```

---

## Task 7: Update handleUpdateUser for Anonymous Conversion

**Files:**
- Modify: `internal/server/auth_handlers.go:274-319` (handleUpdateUser)
- Modify: `internal/auth/user.go`
- Test: `internal/server/auth_handlers_test.go`

**Step 1: Write failing test for anonymous conversion**

Add to `internal/server/auth_handlers_test.go`:

```go
func TestHandleUpdateUserConvertAnonymous(t *testing.T) {
	srv := setupTestServer(t)

	// First create anonymous user
	signupReq := httptest.NewRequest("POST", "/auth/v1/signup", strings.NewReader("{}"))
	signupReq.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, signupReq)

	var signupResp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &signupResp)
	accessToken := signupResp["access_token"].(string)

	// Now convert to permanent user
	updateBody := `{"email": "converted@example.com", "password": "newpassword123"}`
	updateReq := httptest.NewRequest("PUT", "/auth/v1/user", strings.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+accessToken)

	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, updateReq)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)

	// Verify user is no longer anonymous
	if resp["is_anonymous"] != false {
		t.Errorf("expected is_anonymous to be false, got %v", resp["is_anonymous"])
	}
	if resp["email"] != "converted@example.com" {
		t.Errorf("expected email to be converted@example.com, got %v", resp["email"])
	}
}
```

**Step 2: Add ConvertAnonymousUser method to auth service**

Add to `internal/auth/user.go`:

```go
// ConvertAnonymousUser converts an anonymous user to a permanent user with email/password.
func (s *Service) ConvertAnonymousUser(userID, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check if email already exists
	var existingID string
	err := s.db.QueryRow("SELECT id FROM auth_users WHERE email = ? AND deleted_at IS NULL", email).Scan(&existingID)
	if err == nil {
		return fmt.Errorf("email already in use")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Update user: set email, password, is_anonymous=0, update app_metadata
	_, err = s.db.Exec(`
		UPDATE auth_users
		SET email = ?, encrypted_password = ?, is_anonymous = 0,
		    raw_app_meta_data = '{"provider":"email","providers":["email"]}',
		    updated_at = ?
		WHERE id = ? AND is_anonymous = 1
	`, email, string(hash), now, userID)

	if err != nil {
		return fmt.Errorf("failed to convert anonymous user: %w", err)
	}

	return nil
}
```

**Step 3: Update handleUpdateUser to handle conversion**

Edit `internal/server/auth_handlers.go`:

```go
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

	// Handle anonymous user conversion
	if user.IsAnonymous && req.Email != "" && req.Password != "" {
		if len(req.Password) < 6 {
			s.writeError(w, http.StatusBadRequest, "validation_failed", "Password must be at least 6 characters")
			return
		}
		if err := s.authService.ConvertAnonymousUser(user.ID, req.Email, req.Password); err != nil {
			if strings.Contains(err.Error(), "already in use") {
				s.writeError(w, http.StatusBadRequest, "email_exists", "Email already in use")
				return
			}
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to convert user")
			return
		}
	} else {
		// Existing update logic for non-anonymous users
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
```

**Step 4: Run tests**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/user.go internal/server/auth_handlers.go internal/server/auth_handlers_test.go
git commit -m "feat(auth): add anonymous to permanent user conversion"
```

---

## Task 8: Update OAuth Callback for Anonymous Conversion

**Files:**
- Modify: `internal/server/oauth_handlers.go:130-217` (handleCallback)
- Modify: `internal/server/oauth_handlers.go:219-277` (findOrCreateOAuthUser)

**Step 1: Update findOrCreateOAuthUser to handle anonymous users**

Edit `internal/server/oauth_handlers.go`:

```go
// findOrCreateOAuthUser finds an existing user or creates a new one.
// If the current user is anonymous, converts them to permanent via OAuth.
func (s *Server) findOrCreateOAuthUser(provider string, userInfo *oauth.UserInfo, currentUser *auth.User) (*auth.User, error) {
	// First, check if identity already exists
	identity, err := s.authService.GetIdentityByProvider(provider, userInfo.ProviderID)
	if err == nil {
		// Identity exists, get the user
		return s.authService.GetUserByID(identity.UserID)
	}

	// If current user is anonymous, convert them
	if currentUser != nil && currentUser.IsAnonymous {
		// Update the anonymous user with OAuth info
		if err := s.authService.ConvertAnonymousUserViaOAuth(currentUser.ID, userInfo.Email, provider); err != nil {
			return nil, err
		}

		// Create identity for the converted user
		identity := &auth.Identity{
			UserID:     currentUser.ID,
			Provider:   provider,
			ProviderID: userInfo.ProviderID,
			IdentityData: map[string]interface{}{
				"email":      userInfo.Email,
				"name":       userInfo.Name,
				"avatar_url": userInfo.AvatarURL,
			},
		}
		if err := s.authService.CreateIdentity(identity); err != nil {
			return nil, err
		}

		return s.authService.GetUserByID(currentUser.ID)
	}

	// ... rest of existing logic for non-anonymous users
```

**Step 2: Add ConvertAnonymousUserViaOAuth to auth service**

Add to `internal/auth/user.go`:

```go
// ConvertAnonymousUserViaOAuth converts an anonymous user to permanent via OAuth.
func (s *Service) ConvertAnonymousUserViaOAuth(userID, email, provider string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	now := time.Now().UTC().Format(time.RFC3339)

	// Build app_metadata with the OAuth provider
	appMetadata := map[string]interface{}{
		"provider":  provider,
		"providers": []string{provider},
	}
	appMetaJSON, err := json.Marshal(appMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal app_metadata: %w", err)
	}

	// Update user
	_, err = s.db.Exec(`
		UPDATE auth_users
		SET email = ?, is_anonymous = 0, email_confirmed_at = ?,
		    raw_app_meta_data = ?, updated_at = ?
		WHERE id = ? AND is_anonymous = 1
	`, email, now, string(appMetaJSON), now, userID)

	if err != nil {
		return fmt.Errorf("failed to convert anonymous user via OAuth: %w", err)
	}

	return nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/server/... -v`
Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/auth/user.go internal/server/oauth_handlers.go
git commit -m "feat(auth): convert anonymous users via OAuth linking"
```

---

## Task 9: Update Settings Endpoint

**Files:**
- Modify: `internal/server/auth_handlers.go:476-500` (handleSettings)

**Step 1: Add anonymous to external providers**

Edit handleSettings in `internal/server/auth_handlers.go`:

```go
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	requireConfirmation := s.dashboardHandler.GetRequireEmailConfirmation()

	settings := map[string]any{
		"external": map[string]bool{
			"anonymous": true, // Always enabled
			"email":     true,
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
```

**Step 2: Run tests**

Run: `go test ./internal/server/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/server/auth_handlers.go
git commit -m "feat(auth): add anonymous to settings endpoint"
```

---

## Task 10: Add E2E Tests for Anonymous Sign-In

**Files:**
- Create: `e2e/tests/auth/anonymous.test.ts`

**Step 1: Create comprehensive E2E test file**

Create `e2e/tests/auth/anonymous.test.ts`:

```typescript
/**
 * Auth - Anonymous Sign-In Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-signinanonymously
 */

import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - Anonymous Sign-In', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  afterEach(async () => {
    await supabase.auth.signOut()
  })

  describe('1. Sign in anonymously', () => {
    it('should create an anonymous session', async () => {
      const { data, error } = await supabase.auth.signInAnonymously()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.user).toBeDefined()
      expect(data.session).toBeDefined()
      expect(data.user?.is_anonymous).toBe(true)
    })

    it('should return user with null email', async () => {
      const { data, error } = await supabase.auth.signInAnonymously()

      expect(error).toBeNull()
      expect(data.user?.email).toBeNull()
    })

    it('should have authenticated role', async () => {
      const { data, error } = await supabase.auth.signInAnonymously()

      expect(error).toBeNull()
      expect(data.user?.role).toBe('authenticated')
    })

    it('should include is_anonymous in JWT', async () => {
      const { data, error } = await supabase.auth.signInAnonymously()

      expect(error).toBeNull()
      // Decode JWT to verify claim (simplified - just check user object)
      expect(data.user?.is_anonymous).toBe(true)
    })
  })

  describe('2. Sign in anonymously with metadata', () => {
    it('should store custom user metadata', async () => {
      const { data, error } = await supabase.auth.signInAnonymously({
        options: {
          data: { theme: 'dark', locale: 'en-US' },
        },
      })

      expect(error).toBeNull()
      expect(data.user?.user_metadata?.theme).toBe('dark')
      expect(data.user?.user_metadata?.locale).toBe('en-US')
    })
  })

  describe('3. Convert anonymous to permanent user', () => {
    it('should convert via email/password', async () => {
      // Sign in anonymously first
      const { data: anonData } = await supabase.auth.signInAnonymously()
      expect(anonData.user?.is_anonymous).toBe(true)

      // Convert to permanent user
      const email = uniqueEmail()
      const { data, error } = await supabase.auth.updateUser({
        email,
        password: 'newpassword123',
      })

      expect(error).toBeNull()
      expect(data.user?.is_anonymous).toBe(false)
      expect(data.user?.email).toBe(email)
    })

    it('should reject conversion with existing email', async () => {
      // Create a regular user first
      const email = uniqueEmail()
      await supabase.auth.signUp({ email, password: 'password123' })
      await supabase.auth.signOut()

      // Sign in anonymously
      await supabase.auth.signInAnonymously()

      // Try to convert with existing email
      const { error } = await supabase.auth.updateUser({
        email,
        password: 'newpassword123',
      })

      expect(error).not.toBeNull()
    })
  })

  describe('4. Session management', () => {
    it('should refresh anonymous session', async () => {
      const { data: signInData } = await supabase.auth.signInAnonymously()
      const refreshToken = signInData.session?.refresh_token

      // Wait a moment then refresh
      await new Promise((r) => setTimeout(r, 100))

      const { data, error } = await supabase.auth.refreshSession({
        refresh_token: refreshToken!,
      })

      expect(error).toBeNull()
      expect(data.session).toBeDefined()
      expect(data.user?.is_anonymous).toBe(true)
    })

    it('should get current anonymous user', async () => {
      await supabase.auth.signInAnonymously()

      const { data, error } = await supabase.auth.getUser()

      expect(error).toBeNull()
      expect(data.user?.is_anonymous).toBe(true)
    })
  })

  describe('5. Settings endpoint', () => {
    it('should show anonymous as enabled', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/settings`)
      const settings = await response.json()

      expect(settings.external.anonymous).toBe(true)
    })
  })
})
```

**Step 2: Run E2E tests**

Start server: `./sblite serve --db test.db`
In another terminal:
```bash
cd e2e
npm test -- --grep "Anonymous"
```
Expected: All tests PASS

**Step 3: Commit**

```bash
git add e2e/tests/auth/anonymous.test.ts
git commit -m "test(e2e): add anonymous sign-in tests"
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `e2e/TESTS.md`
- Modify: `e2e/COMPATIBILITY.md`

**Step 1: Update CLAUDE.md API endpoints table**

Add anonymous sign-in info to the Authentication endpoints section.

**Step 2: Update e2e/TESTS.md**

Add the new anonymous tests to the test inventory.

**Step 3: Update e2e/COMPATIBILITY.md**

Mark anonymous sign-in as implemented.

**Step 4: Commit**

```bash
git add CLAUDE.md e2e/TESTS.md e2e/COMPATIBILITY.md
git commit -m "docs: add anonymous sign-in documentation"
```

---

## Task 12: Run Full Test Suite

**Step 1: Run all Go tests**

```bash
go test ./... -v
```
Expected: All PASS

**Step 2: Run E2E tests**

```bash
cd e2e && npm test
```
Expected: All PASS (with anonymous tests included)

**Step 3: Final commit and merge**

```bash
git log --oneline -10  # Review commits
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add is_anonymous column | `internal/db/migrations.go` |
| 2 | Add IsAnonymous to User struct | `internal/auth/user.go` |
| 3 | Add is_anonymous JWT claim | `internal/auth/jwt.go` |
| 4 | CreateAnonymousUser method | `internal/auth/user.go` |
| 5 | handleSignup for anonymous | `internal/server/auth_handlers.go` |
| 6 | Include is_anonymous in responses | `internal/server/auth_handlers.go` |
| 7 | Convert anonymous via updateUser | `internal/auth/user.go`, `internal/server/auth_handlers.go` |
| 8 | Convert anonymous via OAuth | `internal/server/oauth_handlers.go` |
| 9 | Update settings endpoint | `internal/server/auth_handlers.go` |
| 10 | E2E tests | `e2e/tests/auth/anonymous.test.ts` |
| 11 | Documentation | `CLAUDE.md`, `e2e/*.md` |
| 12 | Full test suite | - |
