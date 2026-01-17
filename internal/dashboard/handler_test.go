package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandlerServesUI(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "sblite") {
		t.Error("expected body to contain 'sblite'")
	}
}

func TestHandlerServesCSS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/style.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("expected text/css content type, got %s", contentType)
	}
}

func TestHandlerServesJS(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/app.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "javascript") {
		t.Errorf("expected javascript content type, got %s", contentType)
	}
}

func TestHandlerAuthStatus(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected application/json content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "needs_setup") {
		t.Error("expected body to contain 'needs_setup'")
	}
	if !strings.Contains(body, "authenticated") {
		t.Error("expected body to contain 'authenticated'")
	}
}

func TestHandlerSetupAndLogin(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Test setup
	setupReq := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(`{"password":"testpassword123"}`))
	setupReq.Header.Set("Content-Type", "application/json")
	setupW := httptest.NewRecorder()

	r.ServeHTTP(setupW, setupReq)

	if setupW.Code != http.StatusOK {
		t.Errorf("setup: expected status 200, got %d", setupW.Code)
	}

	// Check cookie was set
	cookies := setupW.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_sblite_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Error("setup: expected session cookie to be set")
	}

	// Test login with correct password
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"password":"testpassword123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()

	r.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Errorf("login: expected status 200, got %d", loginW.Code)
	}

	// Test login with wrong password
	wrongReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"password":"wrongpassword"}`))
	wrongReq.Header.Set("Content-Type", "application/json")
	wrongW := httptest.NewRecorder()

	r.ServeHTTP(wrongW, wrongReq)

	if wrongW.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: expected status 401, got %d", wrongW.Code)
	}
}

func TestHandlerLogout(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Setup first
	setupReq := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(`{"password":"testpassword123"}`))
	setupReq.Header.Set("Content-Type", "application/json")
	setupW := httptest.NewRecorder()
	r.ServeHTTP(setupW, setupReq)

	// Test logout
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutW := httptest.NewRecorder()

	r.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("logout: expected status 200, got %d", logoutW.Code)
	}

	// Check cookie was cleared
	cookies := logoutW.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_sblite_session" && c.MaxAge != -1 {
			t.Error("logout: expected session cookie to be cleared")
		}
	}
}

func TestHandlerSPARouting(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Test various SPA routes return index.html
	routes := []string{"/tables", "/users", "/settings"}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("route %s: expected status 200, got %d", route, w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("route %s: expected text/html content type, got %s", route, contentType)
		}

		body := w.Body.String()
		if !strings.Contains(body, "sblite") {
			t.Errorf("route %s: expected body to contain 'sblite'", route)
		}
	}
}

func TestHandlerStaticNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewHandler(database.DB)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/static/nonexistent.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
