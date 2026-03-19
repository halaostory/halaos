package email

import (
	"fmt"
	"log/slog"

	"github.com/resend/resend-go/v2"
)

// Service sends emails via Resend.com.
type Service struct {
	client  *resend.Client
	from    string
	baseURL string // e.g. "https://halaos.com" for building verification links
	logger  *slog.Logger
}

// NewService creates a new email service. If apiKey is empty, emails are logged but not sent.
func NewService(apiKey, from, baseURL string, logger *slog.Logger) *Service {
	var client *resend.Client
	if apiKey != "" {
		client = resend.NewClient(apiKey)
	}
	return &Service{
		client:  client,
		from:    from,
		baseURL: baseURL,
		logger:  logger,
	}
}

// IsEnabled returns true if the email service is configured.
func (s *Service) IsEnabled() bool {
	return s.client != nil
}

// SendVerificationEmail sends an email verification link to the user.
func (s *Service) SendVerificationEmail(toEmail, firstName, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, token)

	subject := "Verify your HalaOS account"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; color: #1a1a2e;">
  <div style="text-align: center; margin-bottom: 32px;">
    <h1 style="color: #4f46e5; font-size: 28px; margin: 0;">HalaOS</h1>
    <p style="color: #64748b; font-size: 14px;">Unified HR &amp; Accounting Platform</p>
  </div>
  <h2 style="font-size: 20px; margin-bottom: 16px;">Welcome, %s!</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    Thank you for registering. Please verify your email address to activate your account.
  </p>
  <div style="text-align: center; margin: 32px 0;">
    <a href="%s" style="display: inline-block; padding: 14px 32px; background: #4f46e5; color: #fff; text-decoration: none; border-radius: 8px; font-size: 16px; font-weight: 600;">
      Verify Email Address
    </a>
  </div>
  <p style="font-size: 14px; color: #64748b;">
    Or copy and paste this link: <br>
    <a href="%s" style="color: #4f46e5; word-break: break-all;">%s</a>
  </p>
  <p style="font-size: 14px; color: #94a3b8; margin-top: 32px;">
    This link expires in 24 hours. If you didn't create an account, you can ignore this email.
  </p>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 32px 0;">
  <p style="font-size: 12px; color: #94a3b8; text-align: center;">
    HalaOS &mdash; HR, Payroll &amp; Tax Compliance
  </p>
</body>
</html>`, firstName, verifyURL, verifyURL, verifyURL)

	if s.client == nil {
		s.logger.Info("email service not configured, logging email",
			"to", toEmail,
			"subject", subject,
			"verify_url", verifyURL,
		)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	sent, err := s.client.Emails.Send(params)
	if err != nil {
		s.logger.Error("failed to send verification email", "to", toEmail, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("verification email sent", "to", toEmail, "email_id", sent.Id)
	return nil
}

// SendWelcomeEmail sends a welcome email after successful verification.
func (s *Service) SendWelcomeEmail(toEmail, firstName string) error {
	subject := "Welcome to HalaOS!"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; color: #1a1a2e;">
  <div style="text-align: center; margin-bottom: 32px;">
    <h1 style="color: #4f46e5; font-size: 28px; margin: 0;">HalaOS</h1>
  </div>
  <h2 style="font-size: 20px; margin-bottom: 16px;">Your account is verified, %s!</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    Your email has been verified and your account is now active. You can start using HalaOS to manage your HR, payroll, and tax compliance.
  </p>
  <div style="text-align: center; margin: 32px 0;">
    <a href="%s/login" style="display: inline-block; padding: 14px 32px; background: #4f46e5; color: #fff; text-decoration: none; border-radius: 8px; font-size: 16px; font-weight: 600;">
      Log In Now
    </a>
  </div>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 32px 0;">
  <p style="font-size: 12px; color: #94a3b8; text-align: center;">
    HalaOS &mdash; HR, Payroll &amp; Tax Compliance
  </p>
</body>
</html>`, firstName, s.baseURL)

	if s.client == nil {
		s.logger.Info("email service not configured, logging welcome email", "to", toEmail)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    html,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		s.logger.Error("failed to send welcome email", "to", toEmail, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendContactForm sends a contact form submission to the support inbox.
func (s *Service) SendContactForm(toAddr, firstName, lastName, fromEmail, company, subject, message string) error {
	subjectLine := fmt.Sprintf("[HalaOS Contact] %s — %s %s", subject, firstName, lastName)
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; color: #1a1a2e;">
  <h2 style="color: #4f46e5; margin: 0 0 24px;">New Contact Form Submission</h2>
  <table style="width: 100%%; border-collapse: collapse;">
    <tr><td style="padding: 8px 0; color: #64748b; width: 120px;">Name:</td><td style="padding: 8px 0; font-weight: 600;">%s %s</td></tr>
    <tr><td style="padding: 8px 0; color: #64748b;">Email:</td><td style="padding: 8px 0;"><a href="mailto:%s" style="color: #4f46e5;">%s</a></td></tr>
    <tr><td style="padding: 8px 0; color: #64748b;">Company:</td><td style="padding: 8px 0;">%s</td></tr>
    <tr><td style="padding: 8px 0; color: #64748b;">Subject:</td><td style="padding: 8px 0;">%s</td></tr>
  </table>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
  <div style="background: #f8fafc; border-radius: 8px; padding: 16px; white-space: pre-wrap; font-size: 14px; line-height: 1.7; color: #334155;">%s</div>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 20px 0;">
  <p style="font-size: 12px; color: #94a3b8;">Sent via HalaOS contact form</p>
</body>
</html>`,
		firstName, lastName, fromEmail, fromEmail, company, subject, message)

	if s.client == nil {
		s.logger.Info("email service not configured, logging contact form",
			"from", fromEmail, "subject", subject)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{toAddr},
		ReplyTo: fromEmail,
		Subject: subjectLine,
		Html:    html,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		s.logger.Error("failed to send contact form email", "from", fromEmail, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("contact form email sent", "from", fromEmail, "subject", subject)
	return nil
}
