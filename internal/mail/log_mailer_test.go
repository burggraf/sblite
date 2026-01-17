// internal/mail/log_mailer_test.go
package mail

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLogMailer_Send(t *testing.T) {
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyText: "Test body content",
		Type:     TypeConfirmation,
	}

	err := mailer.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "user@example.com") {
		t.Errorf("output should contain recipient email")
	}
	if !strings.Contains(output, "Test Subject") {
		t.Errorf("output should contain subject")
	}
	if !strings.Contains(output, "Test body content") {
		t.Errorf("output should contain body")
	}
}

func TestLogMailer_SendValidationError(t *testing.T) {
	var buf bytes.Buffer
	mailer := NewLogMailer(&buf)

	msg := &Message{} // invalid - missing required fields

	err := mailer.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("Send() should return error for invalid message")
	}
}
