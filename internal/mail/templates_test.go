// internal/mail/templates_test.go
package mail

import (
	"testing"
)

func TestTemplateService_GetTemplate(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	tpl, err := svc.GetTemplate(TypeConfirmation)
	if err != nil {
		t.Fatalf("GetTemplate() error = %v", err)
	}

	if tpl.Type != TypeConfirmation {
		t.Errorf("expected type = confirmation, got %s", tpl.Type)
	}
	if tpl.Subject == "" {
		t.Error("expected non-empty subject")
	}
}

func TestTemplateService_ListTemplates(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	templates, err := svc.ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}

	if len(templates) != 5 {
		t.Errorf("expected 5 templates, got %d", len(templates))
	}
}

func TestTemplateService_UpdateTemplate(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	err := svc.UpdateTemplate(TypeConfirmation, "New Subject", "<p>New HTML</p>", "New text")
	if err != nil {
		t.Fatalf("UpdateTemplate() error = %v", err)
	}

	tpl, _ := svc.GetTemplate(TypeConfirmation)
	if tpl.Subject != "New Subject" {
		t.Errorf("expected subject = New Subject, got %s", tpl.Subject)
	}
}

func TestTemplateService_Render(t *testing.T) {
	database := setupTestDB(t)
	svc := NewTemplateService(database)

	data := TemplateData{
		SiteURL:         "https://example.com",
		ConfirmationURL: "https://example.com/auth/v1/verify?token=abc123",
		Email:           "user@example.com",
		Token:           "abc123",
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := svc.Render(TypeConfirmation, data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if subject == "" {
		t.Error("expected non-empty subject")
	}
	if html == "" {
		t.Error("expected non-empty HTML")
	}
	if text == "" {
		t.Error("expected non-empty text")
	}
}
