package email

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/tonypk/aigonhr/internal/config"
)

// Sender sends emails via SMTP.
type Sender struct {
	cfg    config.SMTPConfig
	logger *slog.Logger
}

// NewSender creates an email sender. Returns nil if SMTP is not enabled.
func NewSender(cfg config.SMTPConfig, logger *slog.Logger) *Sender {
	if !cfg.Enabled || cfg.Host == "" {
		return nil
	}
	return &Sender{cfg: cfg, logger: logger}
}

// Send sends an email with HTML body.
func (s *Sender) Send(to, subject, htmlBody string) error {
	if s == nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	msg := strings.Join([]string{
		"From: " + s.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		htmlBody,
	}, "\r\n")

	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg)); err != nil {
		s.logger.Error("failed to send email", "to", to, "subject", subject, "error", err)
		return err
	}

	s.logger.Info("email sent", "to", to, "subject", subject)
	return nil
}

// SendAsync sends an email in a goroutine (fire-and-forget).
func (s *Sender) SendAsync(to, subject, htmlBody string) {
	if s == nil {
		return
	}
	go func() {
		_ = s.Send(to, subject, htmlBody)
	}()
}

// Templates for common HR events.

func LeaveApprovedEmail(employeeName, leaveType, startDate, endDate string) (subject, body string) {
	subject = "Leave Request Approved"
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #2e7d32;">Leave Request Approved</h2>
<p>Dear %s,</p>
<p>Your <strong>%s</strong> leave request from <strong>%s</strong> to <strong>%s</strong> has been <span style="color: #2e7d32;">approved</span>.</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, leaveType, startDate, endDate)
	return
}

func LeaveRejectedEmail(employeeName, leaveType, startDate, endDate, reason string) (subject, body string) {
	subject = "Leave Request Rejected"
	reasonHTML := ""
	if reason != "" {
		reasonHTML = fmt.Sprintf(`<p><strong>Reason:</strong> %s</p>`, reason)
	}
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #d32f2f;">Leave Request Rejected</h2>
<p>Dear %s,</p>
<p>Your <strong>%s</strong> leave request from <strong>%s</strong> to <strong>%s</strong> has been <span style="color: #d32f2f;">rejected</span>.</p>
%s
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, leaveType, startDate, endDate, reasonHTML)
	return
}

func PayrollCompletedEmail(employeeName, cycleName, netPay string) (subject, body string) {
	subject = "Payslip Available - " + cycleName
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #1565c0;">Payslip Available</h2>
<p>Dear %s,</p>
<p>Your payslip for <strong>%s</strong> is now available.</p>
<p>Net Pay: <strong>%s</strong></p>
<p>Please log in to AigoNHR to view the full breakdown.</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, cycleName, netPay)
	return
}

func LoanApprovedEmail(employeeName, loanType, amount string) (subject, body string) {
	subject = "Loan Application Approved"
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #2e7d32;">Loan Approved</h2>
<p>Dear %s,</p>
<p>Your <strong>%s</strong> loan application for <strong>%s</strong> has been <span style="color: #2e7d32;">approved</span>.</p>
<p>Please log in to AigoNHR for details.</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, loanType, amount)
	return
}

func PerformanceReviewAssignedEmail(employeeName, cycleName, deadline string) (subject, body string) {
	subject = "Performance Review Assigned"
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #f57c00;">Performance Review</h2>
<p>Dear %s,</p>
<p>You have been assigned a performance review for cycle <strong>%s</strong>.</p>
<p>Please complete your self-assessment by <strong>%s</strong>.</p>
<p>Log in to AigoNHR to submit your review.</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, cycleName, deadline)
	return
}

func OnboardingTaskEmail(employeeName, taskName, dueDate string) (subject, body string) {
	subject = "Onboarding Task Assigned"
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2 style="color: #7b1fa2;">Onboarding Task</h2>
<p>Dear %s,</p>
<p>You have a new onboarding task: <strong>%s</strong></p>
<p>Due date: <strong>%s</strong></p>
<p>Please log in to AigoNHR to complete this task.</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, employeeName, taskName, dueDate)
	return
}

func GenericNotificationEmail(employeeName, title, message string) (subject, body string) {
	subject = title
	body = fmt.Sprintf(`<div style="font-family: Arial, sans-serif; max-width: 600px;">
<h2>%s</h2>
<p>Dear %s,</p>
<p>%s</p>
<p style="color: #666; font-size: 12px;">This is an automated notification from AigoNHR.</p>
</div>`, title, employeeName, message)
	return
}
