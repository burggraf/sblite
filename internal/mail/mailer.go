// Package mail provides email sending functionality for sblite.
package mail

import (
	"context"
	"fmt"
)

// Email types
const (
	TypeConfirmation = "confirmation"
	TypeRecovery     = "recovery"
	TypeMagicLink    = "magic_link"
	TypeEmailChange  = "email_change"
	TypeInvite       = "invite"
)

// Mail modes
const (
	ModeLog   = "log"
	ModeCatch = "catch"
	ModeSMTP  = "smtp"
)

// Message represents an email message.
type Message struct {
	To       string
	From     string
	Subject  string
	BodyHTML string
	BodyText string
	Type     string
	UserID   string
	Metadata map[string]any
}

// Validate checks if the message has all required fields.
func (m *Message) Validate() error {
	if m.To == "" {
		return fmt.Errorf("to address is required")
	}
	if m.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if m.BodyHTML == "" && m.BodyText == "" {
		return fmt.Errorf("body (html or text) is required")
	}
	return nil
}

// Mailer is the interface for sending emails.
type Mailer interface {
	Send(ctx context.Context, msg *Message) error
}

// Config holds email configuration.
type Config struct {
	Mode     string // log, catch, smtp
	From     string // default sender address
	SiteURL  string // base URL for email links
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Mode:     ModeLog,
		From:     "noreply@localhost",
		SiteURL:  "http://localhost:8080",
		SMTPPort: 587,
	}
}
