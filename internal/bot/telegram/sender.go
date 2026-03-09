package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Sender implements bot.MessageSender for Telegram.
type Sender struct {
	token  string
	client *http.Client
}

// NewSender creates a Telegram message sender.
func NewSender(token string, client *http.Client) *Sender {
	return &Sender{token: token, client: client}
}

func (s *Sender) SendText(ctx context.Context, chatID string, text string) error {
	return s.send(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
}

func (s *Sender) SendMarkdown(ctx context.Context, chatID string, markdown string) error {
	return s.send(ctx, "sendMessage", map[string]any{
		"chat_id":    chatID,
		"text":       markdown,
		"parse_mode": "MarkdownV2",
	})
}

func (s *Sender) SendDraftConfirmation(ctx context.Context, chatID string, text string, draftID string) error {
	keyboard := InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "Confirm", CallbackData: "dc:" + draftID},
				{Text: "Cancel", CallbackData: "dr:" + draftID},
			},
		},
	}

	return s.send(ctx, "sendMessage", map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": keyboard,
	})
}

func (s *Sender) EditMessage(ctx context.Context, chatID string, messageID int, text string) error {
	return s.send(ctx, "editMessageText", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	})
}

func (s *Sender) AnswerCallback(ctx context.Context, callbackID string, text string) error {
	return s.send(ctx, "answerCallbackQuery", map[string]any{
		"callback_query_id": callbackID,
		"text":              text,
	})
}

func (s *Sender) send(ctx context.Context, method string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.token, method)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram %s failed: %s", method, string(respBody))
	}

	return nil
}
