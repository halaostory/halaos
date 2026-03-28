package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/tonypk/aigonhr/internal/ai/agent"
	"github.com/tonypk/aigonhr/internal/ai/draft"
	"github.com/tonypk/aigonhr/internal/store"
)

// Dispatcher routes incoming bot messages to the appropriate handler.
type Dispatcher struct {
	linker   *Linker
	sessions *SessionManager
	executor *agent.Executor
	draftSvc *draft.Service
	queries  *store.Queries
	limiter  *RateLimiter
	logger   *slog.Logger
}

// NewDispatcher creates a message dispatcher.
func NewDispatcher(
	linker *Linker,
	sessions *SessionManager,
	executor *agent.Executor,
	draftSvc *draft.Service,
	queries *store.Queries,
	limiter *RateLimiter,
	logger *slog.Logger,
) *Dispatcher {
	return &Dispatcher{
		linker:   linker,
		sessions: sessions,
		executor: executor,
		draftSvc: draftSvc,
		queries:  queries,
		limiter:  limiter,
		logger:   logger,
	}
}

// Dispatch processes an incoming message and returns the response text.
func (d *Dispatcher) Dispatch(ctx context.Context, msg IncomingMessage, sender MessageSender) {
	// Rate limit
	if !d.limiter.Allow(ctx, msg.Platform, msg.UserID) {
		sender.SendText(ctx, msg.ChatID, "You're sending messages too fast. Please wait a moment.")
		return
	}

	// Handle /start with link code
	if msg.IsCommand && msg.Command == "start" && msg.Args != "" {
		d.handleStart(ctx, msg, sender)
		return
	}

	// Resolve identity
	identity, err := d.linker.ResolveIdentity(ctx, msg.Platform, msg.UserID)
	if err != nil {
		sender.SendText(ctx, msg.ChatID,
			"Your account is not linked. Please link your account from the AIGoNHR web app first, then send /start <code>.")
		return
	}

	if msg.IsCommand {
		d.dispatchCommand(ctx, msg, identity, sender)
		return
	}

	// Free text → AI conversation
	d.handleFreeText(ctx, msg, identity, sender)
}

// HandleCallback processes an inline keyboard callback.
func (d *Dispatcher) HandleCallback(ctx context.Context, cb CallbackQuery, sender MessageSender) {
	identity, err := d.linker.ResolveIdentity(ctx, "telegram", cb.UserID)
	if err != nil {
		sender.AnswerCallback(ctx, cb.ID, "Account not linked")
		return
	}

	data := cb.Data
	if len(data) < 3 {
		sender.AnswerCallback(ctx, cb.ID, "Invalid action")
		return
	}

	// Handle break callbacks (prefix "bk:")
	if strings.HasPrefix(data, "bk:") {
		breakAction := data[3:]
		if breakAction == "end" {
			d.handleBreakEndCallback(ctx, cb, identity, sender)
		} else {
			// breakAction is the break type (meal, bathroom, rest, leave_post)
			d.handleBreakTypeCallback(ctx, cb, identity, breakAction, sender)
		}
		return
	}

	action := data[:2]   // "dc" = confirm, "dr" = reject
	draftIDStr := data[3:] // skip the colon

	draftID, err := uuid.Parse(draftIDStr)
	if err != nil {
		sender.AnswerCallback(ctx, cb.ID, "Invalid draft ID")
		return
	}

	switch action {
	case "dc":
		confirmed, err := d.draftSvc.Confirm(ctx, identity.CompanyID, identity.UserID, draftID)
		if err != nil {
			sender.AnswerCallback(ctx, cb.ID, "Failed to confirm action")
			return
		}

		// Execute the confirmed draft
		var toolInput map[string]any
		json.Unmarshal(confirmed.ToolInput, &toolInput)

		result, execErr := d.queries.GetActionDraft(ctx, store.GetActionDraftParams{
			ID:        draftID,
			CompanyID: identity.CompanyID,
			UserID:    identity.UserID,
		})
		_ = result

		if execErr != nil {
			d.draftSvc.MarkFailed(ctx, draftID, execErr.Error())
			sender.EditMessage(ctx, cb.ChatID, cb.MessageID, "Action failed: "+execErr.Error())
		} else {
			sender.EditMessage(ctx, cb.ChatID, cb.MessageID, "Action confirmed and executed.")
		}
		sender.AnswerCallback(ctx, cb.ID, "Confirmed")

	case "dr":
		d.draftSvc.Reject(ctx, identity.CompanyID, identity.UserID, draftID)
		sender.EditMessage(ctx, cb.ChatID, cb.MessageID, "Action cancelled.")
		sender.AnswerCallback(ctx, cb.ID, "Cancelled")

	default:
		sender.AnswerCallback(ctx, cb.ID, "Unknown action")
	}
}

func (d *Dispatcher) handleStart(ctx context.Context, msg IncomingMessage, sender MessageSender) {
	code := strings.TrimSpace(msg.Args)
	identity, err := d.linker.VerifyLinkCode(ctx, code, msg.Platform, msg.UserID)
	if err != nil {
		sender.SendText(ctx, msg.ChatID, "Invalid or expired link code. Please generate a new one from the web app.")
		return
	}

	// Look up user name
	user, _ := d.queries.GetUserByID(ctx, identity.UserID)
	name := "there"
	if user.ID > 0 {
		name = strings.Split(user.Email, "@")[0]
	}

	tutorial := fmt.Sprintf(`🎉 Linked successfully! Welcome, %s!

Your Telegram is now connected to AIGoNHR. Here's what you can do:

📋 *Quick Commands*
/balance — Check your leave balances
/payslip — View your latest payslip
/clock — Clock in or out (share your location for GPS tracking)
/break — Start a break (meal, bathroom, rest, leave post)
/break_end — End current break
/break_status — Check if you're on break
/leave <reason> — Request leave via AI (e.g. /leave sick leave tomorrow)
/new — Start a fresh conversation
/help — Show command list

💬 *AI Chat*
Just type any message and the AI assistant will help you with:
• HR questions (policies, benefits, etc.)
• Leave requests and approvals
• Payroll inquiries
• Attendance records
• Any work-related questions

📍 *Tips*
• Share your location with /clock for automatic GPS check-in
• You can ask questions in natural language — no special format needed
• Type /new to reset the conversation if the AI gets confused

Get started by typing a question or using a command above!`, name)

	sender.SendText(ctx, msg.ChatID, tutorial)
}

func (d *Dispatcher) dispatchCommand(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	switch msg.Command {
	case "help":
		d.handleHelp(ctx, msg, sender)
	case "balance":
		d.handleBalance(ctx, msg, identity, sender)
	case "payslip":
		d.handlePayslip(ctx, msg, identity, sender)
	case "clock":
		d.handleClock(ctx, msg, identity, sender)
	case "leave":
		d.handleLeaveRequest(ctx, msg, identity, sender)
	case "break":
		d.handleBreakStart(ctx, msg, identity, sender)
	case "break_end":
		d.handleBreakEnd(ctx, msg, identity, sender)
	case "break_status":
		d.handleBreakStatus(ctx, msg, identity, sender)
	case "new":
		d.handleNewSession(ctx, msg, identity, sender)
	default:
		sender.SendText(ctx, msg.ChatID, "Unknown command. Type /help for available commands.")
	}
}

func (d *Dispatcher) handleHelp(ctx context.Context, msg IncomingMessage, sender MessageSender) {
	help := `Available commands:
/balance - Check leave balances
/payslip - View latest payslip
/clock - Clock in/out (share location)
/break - Start a break (meal, bathroom, rest, leave post)
/break_end - End current break
/break_status - Check active break
/leave <description> - Request leave via AI
/new - Start a new conversation
/help - Show this help

Or just type a message to chat with the AI assistant.`
	sender.SendText(ctx, msg.ChatID, help)
}

func (d *Dispatcher) handleBalance(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	// Use AI to get balance — the AI agent has access to query_leave_balance tool
	d.handleFreeText(ctx, IncomingMessage{
		Platform: msg.Platform,
		ChatID:   msg.ChatID,
		UserID:   msg.UserID,
		Text:     "Show me my leave balances for this year",
	}, identity, sender)
}

func (d *Dispatcher) handlePayslip(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	// Use AI to get payslip — the AI agent has access to query_latest_payslip tool
	d.handleFreeText(ctx, IncomingMessage{
		Platform: msg.Platform,
		ChatID:   msg.ChatID,
		UserID:   msg.UserID,
		Text:     "Show me my latest payslip",
	}, identity, sender)
}

func (d *Dispatcher) handleClock(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	if identity.EmployeeID == 0 {
		sender.SendText(ctx, msg.ChatID, "Your account is not associated with an employee record.")
		return
	}

	if msg.Location != nil {
		// Direct clock in/out with location
		d.handleFreeText(ctx, IncomingMessage{
			Platform: msg.Platform,
			ChatID:   msg.ChatID,
			UserID:   msg.UserID,
			Text:     fmt.Sprintf("Clock me in/out at location %.6f, %.6f", msg.Location.Latitude, msg.Location.Longitude),
		}, identity, sender)
		return
	}

	// No location — ask via AI
	d.handleFreeText(ctx, IncomingMessage{
		Platform: msg.Platform,
		ChatID:   msg.ChatID,
		UserID:   msg.UserID,
		Text:     "I want to clock in or out. What's my current status?",
	}, identity, sender)
}

func (d *Dispatcher) handleLeaveRequest(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	text := msg.Args
	if text == "" {
		text = "I want to request leave"
	}
	d.handleFreeText(ctx, IncomingMessage{
		Platform: msg.Platform,
		ChatID:   msg.ChatID,
		UserID:   msg.UserID,
		Text:     text,
	}, identity, sender)
}

func (d *Dispatcher) handleNewSession(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	d.sessions.ResetSession(ctx, identity, "general")
	sender.SendText(ctx, msg.ChatID, "New conversation started. How can I help you?")
}

func (d *Dispatcher) handleFreeText(ctx context.Context, msg IncomingMessage, identity *UserIdentity, sender MessageSender) {
	agentSlug := "general"
	sessionID := d.sessions.GetOrCreateSession(ctx, identity, agentSlug)

	resp, err := d.executor.Chat(ctx, identity.CompanyID, identity.UserID, agentSlug, agent.ChatRequest{
		Message:   msg.Text,
		SessionID: sessionID,
	})
	if err != nil {
		d.logger.Error("bot AI chat failed", "error", err, "user_id", identity.UserID)
		sender.SendText(ctx, msg.ChatID, "Sorry, I encountered an error. Please try again.")
		return
	}

	// Send AI response
	sender.SendText(ctx, msg.ChatID, resp.Message)

	// Check for pending drafts and send confirmation cards
	pendingDrafts, err := d.draftSvc.ListPending(ctx, identity.CompanyID, identity.UserID)
	if err == nil {
		for _, d := range pendingDrafts {
			sender.SendDraftConfirmation(ctx, msg.ChatID, d.Description, d.ID.String())
		}
	}
}
