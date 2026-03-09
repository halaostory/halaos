package email

import (
	"strings"
	"testing"

	"github.com/tonypk/aigonhr/internal/config"
)

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"john@example.com", "jo***@example.com"},
		{"ab@example.com", "a***@example.com"}, // len <= 2 shows only first char
		{"a@example.com", "a***@example.com"},
		{"@example.com", "***"},
		{"noatsign", "***"},
		{"", "***"},
	}
	for _, tc := range tests {
		got := maskEmail(tc.input)
		if got != tc.expected {
			t.Errorf("maskEmail(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNewSender_Disabled(t *testing.T) {
	s := NewSender(config.SMTPConfig{Enabled: false, Host: "smtp.example.com"}, nil)
	if s != nil {
		t.Error("expected nil sender when SMTP disabled")
	}
}

func TestNewSender_EmptyHost(t *testing.T) {
	s := NewSender(config.SMTPConfig{Enabled: true, Host: ""}, nil)
	if s != nil {
		t.Error("expected nil sender when host is empty")
	}
}

func TestNilSender_Send(t *testing.T) {
	var s *Sender
	if err := s.Send("to@example.com", "subj", "body"); err != nil {
		t.Errorf("nil sender Send should return nil, got %v", err)
	}
}

func TestNilSender_SendAsync(t *testing.T) {
	var s *Sender
	// Should not panic
	s.SendAsync("to@example.com", "subj", "body")
}

func TestLeaveApprovedEmail(t *testing.T) {
	subj, body := LeaveApprovedEmail("Juan", "Vacation", "2026-03-01", "2026-03-05")

	if subj != "Leave Request Approved" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Juan", "Vacation", "2026-03-01", "2026-03-05", "approved"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestLeaveRejectedEmail(t *testing.T) {
	subj, body := LeaveRejectedEmail("Maria", "Sick", "2026-04-01", "2026-04-02", "Insufficient balance")

	if subj != "Leave Request Rejected" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Maria", "Sick", "2026-04-01", "2026-04-02", "rejected", "Insufficient balance"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestLeaveRejectedEmail_NoReason(t *testing.T) {
	_, body := LeaveRejectedEmail("Maria", "Sick", "2026-04-01", "2026-04-02", "")

	if strings.Contains(body, "Reason:") {
		t.Error("empty reason should not show Reason field")
	}
}

func TestPayrollCompletedEmail(t *testing.T) {
	subj, body := PayrollCompletedEmail("Pedro", "March 2026", "PHP 35,000.00")

	if !strings.Contains(subj, "March 2026") {
		t.Errorf("subject = %q, want cycle name", subj)
	}
	for _, want := range []string{"Pedro", "March 2026", "PHP 35,000.00"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestLoanApprovedEmail(t *testing.T) {
	subj, body := LoanApprovedEmail("Ana", "Salary Loan", "PHP 50,000")

	if subj != "Loan Application Approved" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Ana", "Salary Loan", "PHP 50,000", "approved"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestPerformanceReviewAssignedEmail(t *testing.T) {
	subj, body := PerformanceReviewAssignedEmail("Carlos", "Q1 2026", "2026-03-31")

	if subj != "Performance Review Assigned" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Carlos", "Q1 2026", "2026-03-31"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestOnboardingTaskEmail(t *testing.T) {
	subj, body := OnboardingTaskEmail("Rosa", "Submit Documents", "2026-03-15")

	if subj != "Onboarding Task Assigned" {
		t.Errorf("subject = %q", subj)
	}
	for _, want := range []string{"Rosa", "Submit Documents", "2026-03-15"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestGenericNotificationEmail(t *testing.T) {
	subj, body := GenericNotificationEmail("Jose", "Holiday Notice", "Office closed on March 31")

	if subj != "Holiday Notice" {
		t.Errorf("subject = %q, want %q", subj, "Holiday Notice")
	}
	for _, want := range []string{"Jose", "Holiday Notice", "Office closed on March 31"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestAllTemplates_ContainFooter(t *testing.T) {
	templates := []struct {
		name string
		body string
	}{
		{"LeaveApproved", func() string { _, b := LeaveApprovedEmail("X", "Y", "A", "B"); return b }()},
		{"LeaveRejected", func() string { _, b := LeaveRejectedEmail("X", "Y", "A", "B", "R"); return b }()},
		{"Payroll", func() string { _, b := PayrollCompletedEmail("X", "Y", "Z"); return b }()},
		{"Loan", func() string { _, b := LoanApprovedEmail("X", "Y", "Z"); return b }()},
		{"Performance", func() string { _, b := PerformanceReviewAssignedEmail("X", "Y", "Z"); return b }()},
		{"Onboarding", func() string { _, b := OnboardingTaskEmail("X", "Y", "Z"); return b }()},
		{"Generic", func() string { _, b := GenericNotificationEmail("X", "Y", "Z"); return b }()},
	}
	for _, tc := range templates {
		if !strings.Contains(tc.body, "automated notification from AigoNHR") {
			t.Errorf("%s template missing footer", tc.name)
		}
	}
}
