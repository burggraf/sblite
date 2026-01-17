// internal/mail/viewer/handler_test.go
package viewer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/mail"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return database
}

func TestHandler_ListEmails(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	// Add a test email
	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyHTML: "<p>Test</p>",
		Type:     mail.TypeConfirmation,
	})

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/emails", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var emails []mail.CaughtEmail
	json.NewDecoder(w.Body).Decode(&emails)
	if len(emails) != 1 {
		t.Errorf("expected 1 email, got %d", len(emails))
	}
}

func TestHandler_GetEmail(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyHTML: "<p>Test body</p>",
		Type:     mail.TypeConfirmation,
	})

	emails, _ := catchMailer.ListEmails(1, 0)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/emails/"+emails[0].ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_DeleteEmail(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	catchMailer.Send(nil, &mail.Message{
		To:       "test@example.com",
		From:     "noreply@example.com",
		Subject:  "Test",
		BodyHTML: "<p>Test</p>",
		Type:     mail.TypeConfirmation,
	})

	emails, _ := catchMailer.ListEmails(1, 0)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/api/emails/"+emails[0].ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify deleted
	remaining, _ := catchMailer.ListEmails(10, 0)
	if len(remaining) != 0 {
		t.Errorf("expected 0 emails, got %d", len(remaining))
	}
}

func TestHandler_ClearAll(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	for i := 0; i < 3; i++ {
		catchMailer.Send(nil, &mail.Message{
			To:       "test@example.com",
			From:     "noreply@example.com",
			Subject:  "Test",
			BodyHTML: "<p>Test</p>",
			Type:     mail.TypeConfirmation,
		})
	}

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("DELETE", "/api/emails", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	remaining, _ := catchMailer.ListEmails(10, 0)
	if len(remaining) != 0 {
		t.Errorf("expected 0 emails, got %d", len(remaining))
	}
}

func TestHandler_ServeUI(t *testing.T) {
	database := setupTestDB(t)
	catchMailer := mail.NewCatchMailer(database)
	handler := NewHandler(catchMailer)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected Content-Type text/html, got %s", ct)
	}
}
