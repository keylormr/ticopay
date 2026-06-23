// Package email sends transactional mail (password reset, email verification).
// It hides the provider behind a small Sender interface so the rest of the app
// never imports the SDK directly, and so we can fall back to logging links in
// development (no API key) instead of failing.
package email

import (
	"context"
	"log/slog"

	"github.com/resend/resend-go/v3"
)

// Sender delivers one transactional email. Implementations must be safe for
// concurrent use.
type Sender interface {
	Send(ctx context.Context, to, subject, html string) error
}

// Config is the subset of app config this package needs.
type Config struct {
	APIKey string // RESEND_API_KEY; empty → dev log sender
	From   string // RESEND_FROM, e.g. "Tico Pay <onboarding@resend.dev>"
	Debug  bool   // EMAIL_DEBUG: dev log sender prints the link. NEVER in prod.
}

// New returns a Resend-backed sender when an API key is configured, otherwise a
// dev sender that logs what *would* be sent (so flows are testable without a
// provider). The returned bool reports whether real email is enabled.
func New(cfg Config, logger *slog.Logger) (Sender, bool) {
	if cfg.APIKey == "" {
		logger.Warn("email: no RESEND_API_KEY set — using dev log sender (no real emails)")
		return &logSender{logger: logger, debug: cfg.Debug}, false
	}
	from := cfg.From
	if from == "" {
		from = "onboarding@resend.dev"
	}
	return &resendSender{client: resend.NewClient(cfg.APIKey), from: from, logger: logger}, true
}

type resendSender struct {
	client *resend.Client
	from   string
	logger *slog.Logger
}

func (s *resendSender) Send(ctx context.Context, to, subject, html string) error {
	sent, err := s.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    html,
	})
	if err != nil {
		s.logger.Error("email send failed", "to", to, "subject", subject, "error", err)
		return err
	}
	s.logger.Info("email sent", "to", to, "subject", subject, "id", sent.Id)
	return nil
}

// logSender is the no-provider fallback. By default it logs only non-sensitive
// metadata; the token-bearing HTML is logged ONLY when EMAIL_DEBUG=true, so a
// misconfigured prod (no API key) never writes working reset links to logs.
type logSender struct {
	logger *slog.Logger
	debug  bool
}

func (s *logSender) Send(_ context.Context, to, subject, html string) error {
	if s.debug {
		s.logger.Info("email (dev, not sent)", "to", to, "subject", subject, "html", html)
	} else {
		s.logger.Info("email (dev, not sent; set EMAIL_DEBUG=true to log the link)", "to", to, "subject", subject)
	}
	return nil
}
