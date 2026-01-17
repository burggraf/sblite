// internal/mail/smtp_mailer.go
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"time"
)

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
}

// Validate checks if the SMTP configuration is complete.
func (c *SMTPConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if c.User == "" || c.Pass == "" {
		return fmt.Errorf("SMTP credentials are required")
	}
	return nil
}

// SMTPMailer sends emails via SMTP.
type SMTPMailer struct {
	config SMTPConfig
}

// NewSMTPMailer creates a new SMTPMailer.
func NewSMTPMailer(config SMTPConfig) *SMTPMailer {
	return &SMTPMailer{config: config}
}

// Send sends the email via SMTP.
func (m *SMTPMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	auth := smtp.PlainAuth("", m.config.User, m.config.Pass, m.config.Host)

	// Build the email message
	body := m.buildMessage(msg)

	// Connect with timeout
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, m.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// STARTTLS for port 587
	if m.config.Port == 587 {
		tlsConfig := &tls.Config{ServerName: m.config.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// buildMessage creates a MIME multipart email message.
func (m *SMTPMailer) buildMessage(msg *Message) []byte {
	var buf bytes.Buffer

	// Create multipart writer for boundary
	writer := multipart.NewWriter(&buf)
	boundary := writer.Boundary()
	writer.Close()

	// Build message
	buf.Reset()
	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "\r\n")

	// Text part
	if msg.BodyText != "" {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n")
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "%s\r\n", msg.BodyText)
	}

	// HTML part
	if msg.BodyHTML != "" {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=utf-8\r\n")
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "%s\r\n", msg.BodyHTML)
	}

	// Close boundary
	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return buf.Bytes()
}
