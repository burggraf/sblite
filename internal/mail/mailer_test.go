// internal/mail/mailer_test.go
package mail

import (
	"testing"
)

func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
	}{
		{
			name: "valid message",
			msg: &Message{
				To:       "user@example.com",
				From:     "noreply@example.com",
				Subject:  "Test",
				BodyHTML: "<p>Test</p>",
				Type:     TypeConfirmation,
			},
			wantErr: false,
		},
		{
			name: "missing to",
			msg: &Message{
				From:    "noreply@example.com",
				Subject: "Test",
				Type:    TypeConfirmation,
			},
			wantErr: true,
		},
		{
			name: "missing subject",
			msg: &Message{
				To:   "user@example.com",
				From: "noreply@example.com",
				Type: TypeConfirmation,
			},
			wantErr: true,
		},
		{
			name: "missing body",
			msg: &Message{
				To:      "user@example.com",
				From:    "noreply@example.com",
				Subject: "Test",
				Type:    TypeConfirmation,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
