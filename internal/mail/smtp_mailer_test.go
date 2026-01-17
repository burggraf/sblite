// internal/mail/smtp_mailer_test.go
package mail

import (
	"context"
	"testing"
)

func TestSMTPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  SMTPConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
				User: "user",
				Pass: "pass",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: SMTPConfig{
				Port: 587,
				User: "user",
				Pass: "pass",
			},
			wantErr: true,
		},
		{
			name: "missing credentials",
			config: SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSMTPMailer_BuildMessage(t *testing.T) {
	mailer := NewSMTPMailer(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		User: "user",
		Pass: "pass",
	})

	msg := &Message{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Subject",
		BodyHTML: "<p>HTML body</p>",
		BodyText: "Text body",
		Type:     TypeConfirmation,
	}

	raw := mailer.buildMessage(msg)

	if len(raw) == 0 {
		t.Error("buildMessage should return non-empty byte slice")
	}
}

func TestSMTPMailer_SendValidationError(t *testing.T) {
	mailer := NewSMTPMailer(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		User: "user",
		Pass: "pass",
	})

	msg := &Message{} // invalid

	err := mailer.Send(context.Background(), msg)
	if err == nil {
		t.Error("Send() should return error for invalid message")
	}
}
