package bot

import "context"

// MessageSender abstracts sending messages to a chat platform (Telegram, WhatsApp, etc.).
type MessageSender interface {
	SendText(ctx context.Context, chatID string, text string) error
	SendMarkdown(ctx context.Context, chatID string, markdown string) error
	SendDraftConfirmation(ctx context.Context, chatID string, text string, draftID string) error
	EditMessage(ctx context.Context, chatID string, messageID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string, text string) error
}

// UserIdentity represents a linked bot user.
type UserIdentity struct {
	LinkID     int64
	UserID     int64
	CompanyID  int64
	EmployeeID int64
	FirstName  string
	LastName   string
	Locale     string
	SessionID  string
}

// IncomingMessage is a platform-agnostic incoming message.
type IncomingMessage struct {
	Platform   string
	ChatID     string
	UserID     string
	Text       string
	MessageID  int
	IsCommand  bool
	Command    string
	Args       string
	Location   *Location
}

// Location represents a shared location.
type Location struct {
	Latitude  float64
	Longitude float64
}

// CallbackQuery is a platform-agnostic inline button callback.
type CallbackQuery struct {
	ID        string
	ChatID    string
	UserID    string
	MessageID int
	Data      string
}
