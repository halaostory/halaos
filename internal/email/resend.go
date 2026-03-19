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

// DripStep defines which onboarding email to send.
type DripStep int

const (
	DripGettingStarted   DripStep = 1 // Day 1: set up company, add departments
	DripFirstEmployee    DripStep = 2 // Day 3: add employees
	DripFirstPayroll     DripStep = 3 // Day 7: run payroll
	DripExploreFeatures  DripStep = 4 // Day 14: compliance, leave, AI features
)

// DripStepDelay returns the minimum age (in days) a company must be before receiving each step.
func DripStepDelay(step DripStep) int {
	switch step {
	case DripGettingStarted:
		return 1
	case DripFirstEmployee:
		return 3
	case DripFirstPayroll:
		return 7
	case DripExploreFeatures:
		return 14
	default:
		return 0
	}
}

// SendDripEmail sends an onboarding drip email for the given step.
func (s *Service) SendDripEmail(toEmail, firstName, companyName string, step DripStep) error {
	subject, html := buildDripEmail(s.baseURL, firstName, companyName, step)
	if subject == "" {
		return fmt.Errorf("unknown drip step: %d", step)
	}

	if s.client == nil {
		s.logger.Info("email service not configured, logging drip email",
			"to", toEmail, "step", step, "subject", subject)
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
		s.logger.Error("failed to send drip email", "to", toEmail, "step", step, "error", err)
		return fmt.Errorf("failed to send drip email: %w", err)
	}

	s.logger.Info("drip email sent", "to", toEmail, "step", step)
	return nil
}

func buildDripEmail(baseURL, firstName, companyName string, step DripStep) (string, string) {
	loginURL := baseURL + "/login"

	wrapper := func(subject, preheader, content string) (string, string) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 24px; color: #1a1a2e;">
  <span style="display:none;font-size:1px;color:#fff;">%s</span>
  <div style="text-align: center; margin-bottom: 32px;">
    <h1 style="color: #4f46e5; font-size: 28px; margin: 0;">HalaOS</h1>
  </div>
  %s
  <div style="text-align: center; margin: 32px 0;">
    <a href="%s" style="display: inline-block; padding: 14px 32px; background: #4f46e5; color: #fff; text-decoration: none; border-radius: 8px; font-size: 16px; font-weight: 600;">Log In to HalaOS</a>
  </div>
  <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 32px 0;">
  <p style="font-size: 12px; color: #94a3b8; text-align: center;">
    HalaOS &mdash; Free HR, Payroll &amp; Tax Compliance<br>
    <a href="%s" style="color: #94a3b8;">Unsubscribe</a>
  </p>
</body>
</html>`, preheader, content, loginURL, baseURL+"/settings")
		return subject, html
	}

	switch step {
	case DripGettingStarted:
		return wrapper(
			"Get started with HalaOS — Set up your company",
			"Complete your company profile in 2 minutes",
			fmt.Sprintf(`<h2 style="font-size: 20px; margin-bottom: 16px;">Hi %s, let's get you started!</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    Welcome to HalaOS! Your company <strong>%s</strong> is ready. Here are 3 quick steps to get the most out of your free account:
  </p>
  <ol style="font-size: 15px; line-height: 2; color: #334155;">
    <li><strong>Complete your company profile</strong> — Add your logo, address, and tax IDs</li>
    <li><strong>Set up departments</strong> — Organize your team structure</li>
    <li><strong>Configure leave types</strong> — Customize vacation, sick leave, and more</li>
  </ol>
  <p style="font-size: 15px; color: #64748b;">This takes about 2 minutes and unlocks the full power of HalaOS.</p>`, firstName, companyName))

	case DripFirstEmployee:
		return wrapper(
			"Add your first employee to HalaOS",
			"Start managing your team in one place",
			fmt.Sprintf(`<h2 style="font-size: 20px; margin-bottom: 16px;">Ready to add your team, %s?</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    Now that <strong>%s</strong> is set up, it's time to add your employees. HalaOS makes it easy:
  </p>
  <ul style="font-size: 15px; line-height: 2; color: #334155;">
    <li><strong>Add employees one by one</strong> — Fill in basic info and employment details</li>
    <li><strong>Bulk import via CSV</strong> — Upload your entire roster at once</li>
    <li><strong>Assign salary structures</strong> — Set up compensation with automatic government deductions</li>
  </ul>
  <p style="font-size: 15px; color: #64748b;">
    HalaOS automatically calculates SSS, PhilHealth, Pag-IBIG, and tax withholding for Philippine employees.
  </p>`, firstName, companyName))

	case DripFirstPayroll:
		return wrapper(
			"Run your first payroll — it's free!",
			"Automated payroll with government compliance",
			fmt.Sprintf(`<h2 style="font-size: 20px; margin-bottom: 16px;">Time to run payroll, %s!</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    Everything is set up for <strong>%s</strong>. Running payroll with HalaOS is simple:
  </p>
  <ol style="font-size: 15px; line-height: 2; color: #334155;">
    <li><strong>Create a payroll cycle</strong> — Choose your pay period (monthly, semi-monthly, weekly)</li>
    <li><strong>Review calculations</strong> — HalaOS auto-computes gross pay, deductions, and net pay</li>
    <li><strong>Approve and distribute</strong> — Generate payslips and export bank files</li>
  </ol>
  <p style="font-size: 15px; color: #334155;">
    <strong>Included for free:</strong> Government contributions (SSS, PhilHealth, Pag-IBIG), tax withholding, 13th month pay, overtime, and holiday pay.
  </p>`, firstName, companyName))

	case DripExploreFeatures:
		return wrapper(
			"Discover more HalaOS features",
			"AI assistant, compliance, analytics and more",
			fmt.Sprintf(`<h2 style="font-size: 20px; margin-bottom: 16px;">There's so much more, %s!</h2>
  <p style="font-size: 16px; line-height: 1.6; color: #334155;">
    You've been using HalaOS for 2 weeks. Here are powerful features you might not have explored yet:
  </p>
  <table style="width: 100%%; border-collapse: collapse; margin: 16px 0;">
    <tr>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9; vertical-align: top; width: 40px; font-size: 20px;">📊</td>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9;"><strong>Analytics Dashboard</strong><br><span style="color: #64748b; font-size: 14px;">Headcount trends, turnover rates, department costs</span></td>
    </tr>
    <tr>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9; vertical-align: top; font-size: 20px;">🤖</td>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9;"><strong>AI HR Assistant</strong><br><span style="color: #64748b; font-size: 14px;">Ask questions about labor law, compute deductions, draft policies</span></td>
    </tr>
    <tr>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9; vertical-align: top; font-size: 20px;">📋</td>
      <td style="padding: 12px; border-bottom: 1px solid #f1f5f9;"><strong>BIR Compliance</strong><br><span style="color: #64748b; font-size: 14px;">Auto-generate 2316, 1601C, 2550M/Q and more</span></td>
    </tr>
    <tr>
      <td style="padding: 12px; vertical-align: top; font-size: 20px;">👥</td>
      <td style="padding: 12px;"><strong>Employee Self-Service</strong><br><span style="color: #64748b; font-size: 14px;">Let employees view payslips, request leave, clock in/out</span></td>
    </tr>
  </table>
  <p style="font-size: 15px; color: #64748b;">
    All of this is <strong>100%% free</strong> for %s. No limits, no hidden fees.
  </p>`, firstName, companyName))
	}

	return "", ""
}
