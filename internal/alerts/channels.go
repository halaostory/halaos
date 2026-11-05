package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Channel kinds. Slack is common in tech companies, but most Southeast Asian
// teams live in Microsoft Teams, Google Chat, Lark, Telegram or LINE groups.
const (
	KindSlack      = "slack"
	KindTeams      = "teams"
	KindGoogleChat = "google_chat"
	KindLark       = "lark"
	KindTelegram   = "telegram"
	KindLINE       = "line"
)

// AllKinds lists the channel kinds in display order.
var AllKinds = []string{KindSlack, KindTeams, KindGoogleChat, KindLark, KindTelegram, KindLINE}

// Kinds whose secret is an incoming-webhook URL; the rest use a bot token
// plus a target chat/group ID.
func usesWebhook(kind string) bool {
	return kind == KindSlack || kind == KindTeams || kind == KindGoogleChat || kind == KindLark
}

// API bases for the bot kinds (overridden in tests).
var (
	telegramBase = "https://api.telegram.org"
	lineBase     = "https://api.line.me"
)

// testWebhookPrefix, when set (tests only), is accepted as a webhook URL of any kind.
var testWebhookPrefix = ""

// validWebhook restricts each webhook kind to its platform's hosts, so a
// stored URL can never point the server somewhere else (no SSRF).
func validWebhook(kind, raw string) bool {
	if testWebhookPrefix != "" && strings.HasPrefix(raw, testWebhookPrefix) {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	switch kind {
	case KindSlack:
		return h == "hooks.slack.com"
	case KindTeams:
		// Teams "Workflows" webhooks (Power Automate) and legacy connectors.
		return strings.HasSuffix(h, ".logic.azure.com") || strings.HasSuffix(h, ".api.powerplatform.com") ||
			strings.HasSuffix(h, ".webhook.office.com")
	case KindGoogleChat:
		return h == "chat.googleapis.com" && strings.HasPrefix(u.Path, "/v1/spaces/")
	case KindLark:
		return (h == "open.larksuite.com" || h == "open.feishu.cn") && strings.HasPrefix(u.Path, "/open-apis/bot/v2/hook/")
	}
	return false
}

var (
	reTelegramToken = regexp.MustCompile(`^\d{5,15}:[A-Za-z0-9_-]{30,64}$`)
	reTelegramChat  = regexp.MustCompile(`^(-?\d{5,20}|@[A-Za-z][A-Za-z0-9_]{4,31})$`)
	reLINEGroup     = regexp.MustCompile(`^[CRU][0-9a-f]{32}$`)
	reBold          = regexp.MustCompile(`\*([^*\n]+)\*`)
)

// validateBot checks a bot token and target for the bot kinds.
func validateBot(kind, token, target string) error {
	switch kind {
	case KindTelegram:
		if token != "" && !reTelegramToken.MatchString(token) {
			return errors.New("That doesn't look like a Telegram bot token (from @BotFather, e.g. 123456789:AA…)")
		}
		if !reTelegramChat.MatchString(target) {
			return errors.New("Enter the group's chat ID (e.g. -1001234567890) — use “Find group” after adding the bot")
		}
	case KindLINE:
		if token != "" && (len(token) < 100 || strings.ContainsAny(token, " \n\t")) {
			return errors.New("Paste the channel access token (long-lived) from the LINE Developers console")
		}
		if !reLINEGroup.MatchString(target) {
			return errors.New("Enter the LINE group ID (starts with C, 33 characters)")
		}
	default:
		return errors.New("unknown channel")
	}
	return nil
}

// message is what an alert says: text uses *bold* and unicode emoji only, so
// each platform can render it; link is an app path such as "/payroll".
type message struct {
	text string
	url  string // absolute link, may be empty
}

func (s *Service) msg(text, link string) message {
	m := message{text: text}
	if link != "" && s.baseURL != "" {
		m.url = s.baseURL + link
	}
	return m
}

const linkLabel = "Open in HalaPeople"

// target is a resolved destination.
type target struct {
	kind   string
	secret string // webhook URL or bot token
	chat   string // chat/group id for bot kinds
}

// post delivers m to t using the platform's own API.
func (s *Service) post(ctx context.Context, t target, m message) error {
	switch t.kind {
	case KindSlack, KindGoogleChat: // both understand *bold* and <url|label>
		if !validWebhook(t.kind, t.secret) {
			return errors.New("not a valid webhook URL for " + t.kind)
		}
		text := m.text
		if m.url != "" {
			text += fmt.Sprintf("\n<%s|%s>", m.url, linkLabel)
		}
		return s.postJSON(ctx, t.secret, nil, map[string]any{"text": text}, nil)
	case KindTeams:
		if !validWebhook(t.kind, t.secret) {
			return errors.New("not a valid Teams workflow URL")
		}
		card := map[string]any{
			"$schema": "http://adaptivecards.io/schemas/adaptive-card.json", "type": "AdaptiveCard", "version": "1.4",
			"body": []any{map[string]any{"type": "TextBlock", "wrap": true, "text": reBold.ReplaceAllString(m.text, "**$1**")}},
		}
		if m.url != "" {
			card["actions"] = []any{map[string]any{"type": "Action.OpenUrl", "title": linkLabel, "url": m.url}}
		}
		return s.postJSON(ctx, t.secret, nil, map[string]any{"type": "message", "attachments": []any{
			map[string]any{"contentType": "application/vnd.microsoft.card.adaptive", "content": card},
		}}, nil)
	case KindLark:
		if !validWebhook(t.kind, t.secret) {
			return errors.New("not a valid Lark/Feishu bot webhook URL")
		}
		text := plain(m)
		var res struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := s.postJSON(ctx, t.secret, nil, map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}, &res); err != nil {
			return err
		}
		if res.Code != 0 { // Lark answers 200 with a non-zero code on errors
			return fmt.Errorf("lark returned %d %s", res.Code, res.Msg)
		}
		return nil
	case KindTelegram:
		if t.secret == "" {
			return errNotConfigured
		}
		text := reBold.ReplaceAllString(html.EscapeString(m.text), "<b>$1</b>")
		if m.url != "" {
			text += fmt.Sprintf("\n<a href=\"%s\">%s</a>", html.EscapeString(m.url), linkLabel)
		}
		var res struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		err := s.postJSON(ctx, telegramBase+"/bot"+t.secret+"/sendMessage", nil,
			map[string]any{"chat_id": t.chat, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true}, &res)
		if err != nil {
			return errors.New(strings.ReplaceAll(err.Error(), t.secret, "…")) // never echo the token
		}
		if !res.OK {
			return errors.New("telegram: " + res.Description)
		}
		return nil
	case KindLINE:
		if t.secret == "" {
			return errNotConfigured
		}
		return s.postJSON(ctx, lineBase+"/v2/bot/message/push", map[string]string{"Authorization": "Bearer " + t.secret},
			map[string]any{"to": t.chat, "messages": []any{map[string]string{"type": "text", "text": plain(m)}}}, nil)
	}
	return errors.New("unknown channel kind")
}

// plain renders for platforms without inline formatting (Lark text, LINE).
func plain(m message) string {
	text := reBold.ReplaceAllString(m.text, "$1")
	if m.url != "" {
		text += "\n" + linkLabel + ": " + m.url
	}
	if len(text) > 4900 { // LINE's limit is 5000 characters
		text = text[:4900] + "…"
	}
	return text
}

func (s *Service) postJSON(ctx context.Context, endpoint string, headers map[string]string, payload, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid destination")
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return ue.Err // drop the URL (it may carry a secret)
		}
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if out != nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
	if resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(raw))
		if len(detail) > 200 {
			detail = detail[:200]
		}
		if detail != "" {
			return fmt.Errorf("%s (%s)", resp.Status, detail)
		}
		return fmt.Errorf("returned %s", resp.Status)
	}
	return nil
}

// telegramGroups lists the group chats a bot has recently seen, so an admin
// can pick the chat ID instead of looking it up. Only works while nothing
// else consumes the bot's updates (a dedicated alerts bot).
func (s *Service) telegramGroups(ctx context.Context, token string) ([]map[string]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, telegramBase+"/bot"+token+"/getUpdates?limit=100", nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("could not reach Telegram")
	}
	defer resp.Body.Close()
	var res struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      []struct {
			Message *struct {
				Chat struct {
					ID    int64  `json:"id"`
					Type  string `json:"type"`
					Title string `json:"title"`
				} `json:"chat"`
			} `json:"message"`
			MyChatMember *struct {
				Chat struct {
					ID    int64  `json:"id"`
					Type  string `json:"type"`
					Title string `json:"title"`
				} `json:"chat"`
			} `json:"my_chat_member"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&res); err != nil {
		return nil, errors.New("unexpected answer from Telegram")
	}
	if !res.OK {
		return nil, errors.New("telegram: " + res.Description)
	}
	seen := map[int64]bool{}
	out := []map[string]string{}
	for _, u := range res.Result {
		var id int64
		var typ, title string
		switch {
		case u.Message != nil:
			id, typ, title = u.Message.Chat.ID, u.Message.Chat.Type, u.Message.Chat.Title
		case u.MyChatMember != nil:
			id, typ, title = u.MyChatMember.Chat.ID, u.MyChatMember.Chat.Type, u.MyChatMember.Chat.Title
		default:
			continue
		}
		if (typ != "group" && typ != "supergroup" && typ != "channel") || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, map[string]string{"chat_id": fmt.Sprint(id), "title": title})
	}
	return out, nil
}
