package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/testutil"
)

type mockSender struct {
	texts     []string
	keyboards int
	callbacks []string
	edits     []string
}

func (m *mockSender) SendText(_ context.Context, _ string, text string) error {
	m.texts = append(m.texts, text)
	return nil
}
func (m *mockSender) SendMarkdown(_ context.Context, _ string, _ string) error { return nil }
func (m *mockSender) SendDraftConfirmation(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (m *mockSender) SendWithKeyboard(_ context.Context, _ string, _ string, _ [][]InlineButton) error {
	m.keyboards++
	return nil
}
func (m *mockSender) EditMessage(_ context.Context, _ string, _ int, text string) error {
	m.edits = append(m.edits, text)
	return nil
}
func (m *mockSender) AnswerCallback(_ context.Context, _ string, text string) error {
	m.callbacks = append(m.callbacks, text)
	return nil
}

// breakLogScanValues returns 11 mock values matching the BreakLog scan order.
func breakLogScanValues(id int64, breakType string) []interface{} {
	return []interface{}{
		id,                           // ID
		int64(1),                     // CompanyID
		int64(10),                    // EmployeeID
		int64(100),                   // AttendanceLogID
		breakType,                    // BreakType
		time.Now(),                   // StartAt
		pgtype.Timestamptz{},         // EndAt
		(*int32)(nil),                // DurationMinutes
		(*int32)(nil),                // OvertimeMinutes
		(*string)(nil),               // Note
		time.Now(),                   // CreatedAt
	}
}

// attendanceLogScanValues returns 26 mock values matching the AttendanceLog scan order.
func attendanceLogScanValues(id int64) []interface{} {
	return []interface{}{
		id,                           // ID
		int64(1),                     // CompanyID
		int64(10),                    // EmployeeID
		pgtype.Timestamptz{Time: time.Now(), Valid: true}, // ClockInAt
		pgtype.Timestamptz{},         // ClockOutAt
		"web",                        // ClockInSource
		(*string)(nil),               // ClockOutSource
		pgtype.Numeric{},             // ClockInLat
		pgtype.Numeric{},             // ClockInLng
		pgtype.Numeric{},             // ClockOutLat
		pgtype.Numeric{},             // ClockOutLng
		(*string)(nil),               // ClockInNote
		(*string)(nil),               // ClockOutNote
		pgtype.Numeric{},             // WorkHours
		pgtype.Numeric{},             // OvertimeHours
		(*int32)(nil),                // LateMinutes
		(*int32)(nil),                // UndertimeMinutes
		"open",                       // Status
		false,                        // IsCorrected
		(*int64)(nil),                // CorrectedBy
		time.Now(),                   // CreatedAt
		time.Now(),                   // UpdatedAt
		(*int64)(nil),                // ClockInGeofenceID
		(*string)(nil),               // ClockInGeofenceStatus
		(*int64)(nil),                // ClockOutGeofenceID
		(*string)(nil),               // ClockOutGeofenceStatus
	}
}

func TestHandleBreakStart_NotClockedIn(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	// GetOpenAttendance fails (not clocked in)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "❌ Please clock in first" {
		t.Fatalf("expected not clocked in message, got: %v", sender.texts)
	}
}

func TestHandleBreakStart_ShowsKeyboard(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails (no active break)
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))
	// GetOpenAttendance succeeds
	mockDB.OnQueryRow(testutil.NewRow(attendanceLogScanValues(100)...))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	if sender.keyboards != 1 {
		t.Fatalf("expected keyboard to be sent, got %d keyboards", sender.keyboards)
	}
}

func TestHandleBreakStart_AlreadyOnBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak succeeds (already on break)
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues(5, "meal")...))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	// Should show "end break" keyboard instead
	if sender.keyboards != 1 {
		t.Fatalf("expected end-break keyboard, got %d keyboards", sender.keyboards)
	}
}

func TestHandleBreakStart_NoEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 0, CompanyID: 1}

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStart(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "Your account is not associated with an employee record." {
		t.Fatalf("expected no employee message, got: %v", sender.texts)
	}
}

func TestHandleBreakEnd_NoActiveBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakEnd(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "❌ No active break" {
		t.Fatalf("expected no active break message, got: %v", sender.texts)
	}
}

func TestHandleBreakEnd_NoEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 0, CompanyID: 1}

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakEnd(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "Your account is not associated with an employee record." {
		t.Fatalf("expected no employee message, got: %v", sender.texts)
	}
}

func TestHandleBreakStatus_NoBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak fails
	mockDB.OnQueryRow(testutil.NewErrorRow(fmt.Errorf("not found")))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStatus(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "✅ No active break" {
		t.Fatalf("expected no break message, got: %v", sender.texts)
	}
}

func TestHandleBreakStatus_ActiveBreak(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 10, CompanyID: 1}

	// GetActiveBreak succeeds
	mockDB.OnQueryRow(testutil.NewRow(breakLogScanValues(5, "rest")...))

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStatus(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 {
		t.Fatal("expected status message, got none")
	}
	if sender.texts[0] == "✅ No active break" {
		t.Fatal("should report active break, not no break")
	}
}

func TestHandleBreakStatus_NoEmployee(t *testing.T) {
	mockDB := testutil.NewMockDBTX()
	queries := store.New(mockDB)
	d := &Dispatcher{queries: queries}
	sender := &mockSender{}

	identity := &UserIdentity{EmployeeID: 0, CompanyID: 1}

	msg := IncomingMessage{ChatID: "123", Platform: "telegram"}
	d.handleBreakStatus(context.Background(), msg, identity, sender)

	if len(sender.texts) == 0 || sender.texts[0] != "Your account is not associated with an employee record." {
		t.Fatalf("expected no employee message, got: %v", sender.texts)
	}
}
