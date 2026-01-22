// internal/mail/service_test.go
package mail

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestEmailService_SendConfirmation(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendConfirmation(context.Background(), "user-123", "user@example.com", "token123")
	if err != nil {
		t.Fatalf("SendConfirmation() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user@example.com") {
		t.Error("output should contain recipient email")
	}
	if !strings.Contains(output, "CONFIRMATION") {
		t.Error("output should indicate confirmation type")
	}
}

func TestEmailService_SendRecovery(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendRecovery(context.Background(), "user-123", "user@example.com", "token456")
	if err != nil {
		t.Fatalf("SendRecovery() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "RECOVERY") {
		t.Error("output should indicate recovery type")
	}
}

func TestEmailService_SendMagicLink(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendMagicLink(context.Background(), "user@example.com", "token789", "")
	if err != nil {
		t.Fatalf("SendMagicLink() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MAGIC_LINK") {
		t.Error("output should indicate magic_link type")
	}
}

func TestEmailService_SendEmailChange(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendEmailChange(context.Background(), "user-123", "new@example.com", "tokenABC")
	if err != nil {
		t.Fatalf("SendEmailChange() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "new@example.com") {
		t.Error("output should contain new email")
	}
	if !strings.Contains(output, "EMAIL_CHANGE") {
		t.Error("output should indicate email_change type")
	}
}

func TestEmailService_SendInvite(t *testing.T) {
	database := setupTestDB(t)
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)
	templates := NewTemplateService(database)

	svc := NewEmailService(mailer, templates, &Config{
		From:    "noreply@example.com",
		SiteURL: "https://example.com",
	})

	err := svc.SendInvite(context.Background(), "invited@example.com", "tokenXYZ")
	if err != nil {
		t.Fatalf("SendInvite() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "invited@example.com") {
		t.Error("output should contain invited email")
	}
	if !strings.Contains(output, "INVITE") {
		t.Error("output should indicate invite type")
	}
}
