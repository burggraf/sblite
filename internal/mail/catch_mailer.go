// internal/mail/catch_mailer.go
package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/markb/sblite/internal/db"
)

// CaughtEmail represents a stored email in catch mode.
type CaughtEmail struct {
	ID        string         `json:"id"`
	To        string         `json:"to"`
	From      string         `json:"from"`
	Subject   string         `json:"subject"`
	BodyHTML  string         `json:"body_html,omitempty"`
	BodyText  string         `json:"body_text,omitempty"`
	Type      string         `json:"type"`
	UserID    string         `json:"user_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// CatchMailer stores emails in the database for local development.
type CatchMailer struct {
	db *db.DB
}

// NewCatchMailer creates a new CatchMailer.
func NewCatchMailer(database *db.DB) *CatchMailer {
	return &CatchMailer{db: database}
}

// Send stores the email in the database.
func (m *CatchMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	var metadataJSON *string
	if msg.Metadata != nil {
		b, err := json.Marshal(msg.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		s := string(b)
		metadataJSON = &s
	}

	_, err := m.db.Exec(`
		INSERT INTO auth_emails (id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, msg.To, msg.From, msg.Subject, msg.BodyHTML, msg.BodyText, msg.Type, msg.UserID, now, metadataJSON)

	if err != nil {
		return fmt.Errorf("failed to store email: %w", err)
	}

	return nil
}

// ListEmails returns caught emails, newest first.
func (m *CatchMailer) ListEmails(limit, offset int) ([]CaughtEmail, error) {
	rows, err := m.db.Query(`
		SELECT id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata
		FROM auth_emails
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list emails: %w", err)
	}
	defer rows.Close()

	var emails []CaughtEmail
	for rows.Next() {
		var e CaughtEmail
		var bodyHTML, bodyText, userID, metadataJSON *string
		var createdAt string

		err := rows.Scan(&e.ID, &e.To, &e.From, &e.Subject, &bodyHTML, &bodyText, &e.Type, &userID, &createdAt, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}

		if bodyHTML != nil {
			e.BodyHTML = *bodyHTML
		}
		if bodyText != nil {
			e.BodyText = *bodyText
		}
		if userID != nil {
			e.UserID = *userID
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if metadataJSON != nil {
			json.Unmarshal([]byte(*metadataJSON), &e.Metadata)
		}

		emails = append(emails, e)
	}

	return emails, nil
}

// GetEmail returns a single email by ID.
func (m *CatchMailer) GetEmail(id string) (*CaughtEmail, error) {
	var e CaughtEmail
	var bodyHTML, bodyText, userID, metadataJSON *string
	var createdAt string

	err := m.db.QueryRow(`
		SELECT id, to_email, from_email, subject, body_html, body_text, email_type, user_id, created_at, metadata
		FROM auth_emails WHERE id = ?
	`, id).Scan(&e.ID, &e.To, &e.From, &e.Subject, &bodyHTML, &bodyText, &e.Type, &userID, &createdAt, &metadataJSON)

	if err != nil {
		return nil, fmt.Errorf("email not found: %w", err)
	}

	if bodyHTML != nil {
		e.BodyHTML = *bodyHTML
	}
	if bodyText != nil {
		e.BodyText = *bodyText
	}
	if userID != nil {
		e.UserID = *userID
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if metadataJSON != nil {
		json.Unmarshal([]byte(*metadataJSON), &e.Metadata)
	}

	return &e, nil
}

// DeleteEmail removes a single email.
func (m *CatchMailer) DeleteEmail(id string) error {
	_, err := m.db.Exec("DELETE FROM auth_emails WHERE id = ?", id)
	return err
}

// ClearAll removes all caught emails.
func (m *CatchMailer) ClearAll() error {
	_, err := m.db.Exec("DELETE FROM auth_emails")
	return err
}

// Count returns the total number of caught emails.
func (m *CatchMailer) Count() (int, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM auth_emails").Scan(&count)
	return count, err
}
