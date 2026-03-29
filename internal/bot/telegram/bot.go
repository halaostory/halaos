package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/bot"
)

// Bot runs the Telegram long-polling loop.
type Bot struct {
	token      string
	client     *http.Client
	dispatcher *bot.Dispatcher
	sender     *Sender
	logger     *slog.Logger
	offset     int
}

// New creates a Telegram bot.
func New(token string, dispatcher *bot.Dispatcher, logger *slog.Logger) *Bot {
	client := &http.Client{Timeout: 60 * time.Second}
	sender := NewSender(token, client)
	return &Bot{
		token:      token,
		client:     client,
		dispatcher: dispatcher,
		sender:     sender,
		logger:     logger,
	}
}

// Run starts the long-polling loop. Blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	b.logger.Info("telegram bot started (long-polling)")

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("telegram bot stopped")
			return
		default:
			b.pollUpdates(ctx)
		}
	}
}

func (b *Bot) pollUpdates(ctx context.Context) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30&allowed_updates=[\"message\",\"callback_query\"]",
		b.token, b.offset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := b.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		b.logger.Error("telegram poll failed", "error", err)
		time.Sleep(5 * time.Second)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		b.logger.Error("telegram parse failed", "error", err)
		return
	}

	if !result.OK {
		b.logger.Error("telegram API returned not ok", "body", string(body))
		time.Sleep(5 * time.Second)
		return
	}

	for _, update := range result.Result {
		b.offset = update.UpdateID + 1
		b.handleUpdate(ctx, update)
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update Update) {
	if update.CallbackQuery != nil {
		cb := bot.CallbackQuery{
			ID:        update.CallbackQuery.ID,
			ChatID:    fmt.Sprintf("%d", update.CallbackQuery.Message.Chat.ID),
			UserID:    fmt.Sprintf("%d", update.CallbackQuery.From.ID),
			MessageID: update.CallbackQuery.Message.MessageID,
			Data:      update.CallbackQuery.Data,
		}
		b.dispatcher.HandleCallback(ctx, cb, b.sender)
		return
	}

	if update.Message == nil {
		return
	}

	msg := bot.IncomingMessage{
		Platform:  "telegram",
		ChatID:    fmt.Sprintf("%d", update.Message.Chat.ID),
		UserID:    fmt.Sprintf("%d", update.Message.From.ID),
		Text:      update.Message.Text,
		MessageID: update.Message.MessageID,
	}

	// Parse location
	if update.Message.Location != nil {
		msg.Location = &bot.Location{
			Latitude:  update.Message.Location.Latitude,
			Longitude: update.Message.Location.Longitude,
		}
	}

	// Parse commands
	if strings.HasPrefix(msg.Text, "/") {
		msg.IsCommand = true
		parts := strings.SplitN(msg.Text, " ", 2)
		msg.Command = strings.TrimPrefix(parts[0], "/")
		// Strip @botname from command
		if idx := strings.Index(msg.Command, "@"); idx != -1 {
			msg.Command = msg.Command[:idx]
		}
		if len(parts) > 1 {
			msg.Args = parts[1]
		}
	}

	b.dispatcher.Dispatch(ctx, msg, b.sender)
}
