package telegram

// Update represents a Telegram update.
type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID int       `json:"message_id"`
	From      *User     `json:"from"`
	Chat      Chat      `json:"chat"`
	Text      string    `json:"text"`
	Location  *Location `json:"location"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// Location represents a Telegram location.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CallbackQuery represents a Telegram callback query (inline keyboard).
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}
