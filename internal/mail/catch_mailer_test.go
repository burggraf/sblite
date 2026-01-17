// internal/mail/catch_mailer_test.go
package mail

import (
	"context"
	"os"
	"testing"

	"github.com/markb/sblite/internal/db"
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

func TestCatchMailer_Send(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>Test HTML</p>",
		BodyText: "Test text",
		Type:     TypeConfirmation,
		UserID:   "user-123",
	}

	err := mailer.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify email was stored
	emails, err := mailer.ListEmails(10, 0)
	if err != nil {
		t.Fatalf("ListEmails() error = %v", err)
	}
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	if emails[0].To != "user@example.com" {
		t.Errorf("expected to = user@example.com, got %s", emails[0].To)
	}
}

func TestCatchMailer_GetEmail(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>Test</p>",
		Type:     TypeRecovery,
	}

	_ = mailer.Send(context.Background(), msg)

	emails, _ := mailer.ListEmails(10, 0)
	email, err := mailer.GetEmail(emails[0].ID)
	if err != nil {
		t.Fatalf("GetEmail() error = %v", err)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("expected subject = Test Subject, got %s", email.Subject)
	}
}

func TestCatchMailer_DeleteEmail(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test",
		BodyHTML: "<p>Test</p>",
		Type:     TypeConfirmation,
	}

	_ = mailer.Send(context.Background(), msg)
	emails, _ := mailer.ListEmails(10, 0)

	err := mailer.DeleteEmail(emails[0].ID)
	if err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}

	emails, _ = mailer.ListEmails(10, 0)
	if len(emails) != 0 {
		t.Errorf("expected 0 emails after delete, got %d", len(emails))
	}
}

func TestCatchMailer_ClearAll(t *testing.T) {
	database := setupTestDB(t)
	mailer := NewCatchMailer(database)

	for i := 0; i < 3; i++ {
		msg := &Message{
			To:       "user@example.com",
			From:     "noreply@example.com",
			Subject:  "Test",
			BodyHTML: "<p>Test</p>",
			Type:     TypeConfirmation,
		}
		_ = mailer.Send(context.Background(), msg)
	}

	err := mailer.ClearAll()
	if err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}

	emails, _ := mailer.ListEmails(10, 0)
	if len(emails) != 0 {
		t.Errorf("expected 0 emails after clear, got %d", len(emails))
	}
}
