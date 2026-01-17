// internal/mail/templates.go
package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"sync"
	texttemplate "text/template"
	"time"

	"github.com/markb/sblite/internal/db"
)

// EmailTemplate represents a stored email template.
type EmailTemplate struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Subject   string    `json:"subject"`
	BodyHTML  string    `json:"body_html"`
	BodyText  string    `json:"body_text,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateData contains variables for template rendering.
type TemplateData struct {
	SiteURL         string
	ConfirmationURL string
	Email           string
	Token           string
	ExpiresIn       string
}

// TemplateService manages email templates.
type TemplateService struct {
	db    *db.DB
	cache map[string]*EmailTemplate
	mu    sync.RWMutex
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(database *db.DB) *TemplateService {
	return &TemplateService{
		db:    database,
		cache: make(map[string]*EmailTemplate),
	}
}

// GetTemplate retrieves a template by type.
func (s *TemplateService) GetTemplate(templateType string) (*EmailTemplate, error) {
	// Check cache first
	s.mu.RLock()
	if tpl, ok := s.cache[templateType]; ok {
		s.mu.RUnlock()
		return tpl, nil
	}
	s.mu.RUnlock()

	// Query database
	var tpl EmailTemplate
	var bodyText *string
	var updatedAt string

	err := s.db.QueryRow(`
		SELECT id, type, subject, body_html, body_text, updated_at
		FROM auth_email_templates WHERE type = ?
	`, templateType).Scan(&tpl.ID, &tpl.Type, &tpl.Subject, &tpl.BodyHTML, &bodyText, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	if bodyText != nil {
		tpl.BodyText = *bodyText
	}
	tpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Update cache
	s.mu.Lock()
	s.cache[templateType] = &tpl
	s.mu.Unlock()

	return &tpl, nil
}

// ListTemplates returns all templates.
func (s *TemplateService) ListTemplates() ([]EmailTemplate, error) {
	rows, err := s.db.Query(`
		SELECT id, type, subject, body_html, body_text, updated_at
		FROM auth_email_templates ORDER BY type
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	var templates []EmailTemplate
	for rows.Next() {
		var tpl EmailTemplate
		var bodyText *string
		var updatedAt string

		err := rows.Scan(&tpl.ID, &tpl.Type, &tpl.Subject, &tpl.BodyHTML, &bodyText, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}

		if bodyText != nil {
			tpl.BodyText = *bodyText
		}
		tpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		templates = append(templates, tpl)
	}

	return templates, nil
}

// UpdateTemplate updates a template by type.
func (s *TemplateService) UpdateTemplate(templateType, subject, bodyHTML, bodyText string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.Exec(`
		UPDATE auth_email_templates
		SET subject = ?, body_html = ?, body_text = ?, updated_at = ?
		WHERE type = ?
	`, subject, bodyHTML, bodyText, now, templateType)

	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	// Invalidate cache
	s.mu.Lock()
	delete(s.cache, templateType)
	s.mu.Unlock()

	return nil
}

// Render renders a template with the given data.
func (s *TemplateService) Render(templateType string, data TemplateData) (subject, html, text string, err error) {
	tpl, err := s.GetTemplate(templateType)
	if err != nil {
		return "", "", "", err
	}

	// Render subject (plain text template)
	subjectTpl, err := texttemplate.New("subject").Parse(tpl.Subject)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTpl.Execute(&subjectBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render subject: %w", err)
	}
	subject = subjectBuf.String()

	// Render HTML body
	htmlTpl, err := template.New("html").Parse(tpl.BodyHTML)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse HTML template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := htmlTpl.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("failed to render HTML: %w", err)
	}
	html = htmlBuf.String()

	// Render text body
	if tpl.BodyText != "" {
		textTpl, err := texttemplate.New("text").Parse(tpl.BodyText)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to parse text template: %w", err)
		}
		var textBuf bytes.Buffer
		if err := textTpl.Execute(&textBuf, data); err != nil {
			return "", "", "", fmt.Errorf("failed to render text: %w", err)
		}
		text = textBuf.String()
	}

	return subject, html, text, nil
}

// InvalidateCache clears the template cache.
func (s *TemplateService) InvalidateCache() {
	s.mu.Lock()
	s.cache = make(map[string]*EmailTemplate)
	s.mu.Unlock()
}
