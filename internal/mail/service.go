// internal/mail/service.go
package mail

import (
	"context"
	"fmt"
	"net/url"
)

// EmailService provides high-level email sending operations.
type EmailService struct {
	mailer    Mailer
	templates *TemplateService
	config    *Config
}

// NewEmailService creates a new EmailService.
func NewEmailService(mailer Mailer, templates *TemplateService, config *Config) *EmailService {
	return &EmailService{
		mailer:    mailer,
		templates: templates,
		config:    config,
	}
}

// SendConfirmation sends an email confirmation message.
func (s *EmailService) SendConfirmation(ctx context.Context, userID, email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=signup", s.config.SiteURL, url.QueryEscape(token))

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := s.templates.Render(TypeConfirmation, data)
	if err != nil {
		return fmt.Errorf("failed to render confirmation template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeConfirmation,
		UserID:   userID,
	}

	return s.mailer.Send(ctx, msg)
}

// SendRecovery sends a password recovery email.
func (s *EmailService) SendRecovery(ctx context.Context, userID, email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=recovery", s.config.SiteURL, url.QueryEscape(token))

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "1 hour",
	}

	subject, html, text, err := s.templates.Render(TypeRecovery, data)
	if err != nil {
		return fmt.Errorf("failed to render recovery template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeRecovery,
		UserID:   userID,
	}

	return s.mailer.Send(ctx, msg)
}

// SendMagicLink sends a magic link email.
// If redirectTo is provided, it will be included in the magic link URL for post-verification redirect.
func (s *EmailService) SendMagicLink(ctx context.Context, email, token, redirectTo string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=magiclink", s.config.SiteURL, url.QueryEscape(token))
	if redirectTo != "" {
		confirmURL += "&redirect_to=" + url.QueryEscape(redirectTo)
	}

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "1 hour",
	}

	subject, html, text, err := s.templates.Render(TypeMagicLink, data)
	if err != nil {
		return fmt.Errorf("failed to render magic link template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeMagicLink,
	}

	return s.mailer.Send(ctx, msg)
}

// SendEmailChange sends an email change verification.
func (s *EmailService) SendEmailChange(ctx context.Context, userID, newEmail, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=email_change", s.config.SiteURL, url.QueryEscape(token))

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           newEmail,
		Token:           token,
		ExpiresIn:       "24 hours",
	}

	subject, html, text, err := s.templates.Render(TypeEmailChange, data)
	if err != nil {
		return fmt.Errorf("failed to render email change template: %w", err)
	}

	msg := &Message{
		To:       newEmail,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeEmailChange,
		UserID:   userID,
	}

	return s.mailer.Send(ctx, msg)
}

// SendInvite sends an invitation email.
func (s *EmailService) SendInvite(ctx context.Context, email, token string) error {
	confirmURL := fmt.Sprintf("%s/auth/v1/verify?token=%s&type=invite", s.config.SiteURL, url.QueryEscape(token))

	data := TemplateData{
		SiteURL:         s.config.SiteURL,
		ConfirmationURL: confirmURL,
		Email:           email,
		Token:           token,
		ExpiresIn:       "7 days",
	}

	subject, html, text, err := s.templates.Render(TypeInvite, data)
	if err != nil {
		return fmt.Errorf("failed to render invite template: %w", err)
	}

	msg := &Message{
		To:       email,
		From:     s.config.From,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
		Type:     TypeInvite,
	}

	return s.mailer.Send(ctx, msg)
}
