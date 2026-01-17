// internal/mail/log_mailer.go
package mail

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogMailer logs emails to an io.Writer (default: stdout).
type LogMailer struct {
	w io.Writer
}

// NewLogMailer creates a new LogMailer. If w is nil, os.Stdout is used.
func NewLogMailer(w io.Writer) *LogMailer {
	if w == nil {
		w = os.Stdout
	}
	return &LogMailer{w: w}
}

// Send logs the email to the writer.
func (m *LogMailer) Send(ctx context.Context, msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  EMAIL [%s] %s\n", strings.ToUpper(string(msg.Type)), time.Now().Format(time.RFC3339)))
	sb.WriteString("════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  From:    %s\n", msg.From))
	sb.WriteString(fmt.Sprintf("  To:      %s\n", msg.To))
	sb.WriteString(fmt.Sprintf("  Subject: %s\n", msg.Subject))
	sb.WriteString("────────────────────────────────────────────────────────────\n")

	// Prefer text body for log output, fall back to HTML
	body := msg.BodyText
	if body == "" {
		body = msg.BodyHTML
	}
	sb.WriteString(body)
	sb.WriteString("\n")
	sb.WriteString("════════════════════════════════════════════════════════════\n\n")

	_, err := fmt.Fprint(m.w, sb.String())
	return err
}
