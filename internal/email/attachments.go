package email

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
	"time"
)

// ErrNotConfigured is returned when an e-mail must really be delivered
// (e.g. a government e-submission) but SMTP is not configured.
var ErrNotConfigured = errors.New("smtp is not configured")

// Attachment is a file attached to an e-mail.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Message is an e-mail with attachments.
type Message struct {
	From        string
	To          []string
	Cc          []string
	Subject     string
	TextBody    string
	Attachments []Attachment
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildMIME renders msg as a multipart/mixed RFC 5322 message and returns it
// with the generated Message-ID.
func BuildMIME(msg Message, domain string, now time.Time) ([]byte, string) {
	if domain == "" {
		domain = "halaos.local"
	}
	msgID := fmt.Sprintf("<%s.%d@%s>", randomHex(8), now.UnixNano(), domain)
	boundary := "halaos-" + randomHex(12)
	var b bytes.Buffer
	w := func(s string) { b.WriteString(s + "\r\n") }
	w("From: " + msg.From)
	w("To: " + strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		w("Cc: " + strings.Join(msg.Cc, ", "))
	}
	w("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject))
	w("Date: " + now.Format(time.RFC1123Z))
	w("Message-ID: " + msgID)
	w("MIME-Version: 1.0")
	w(`Content-Type: multipart/mixed; boundary="` + boundary + `"`)
	w("")
	w("--" + boundary)
	w("Content-Type: text/plain; charset=UTF-8")
	w("Content-Transfer-Encoding: 8bit")
	w("")
	w(msg.TextBody)
	for _, a := range msg.Attachments {
		ct := a.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		w("--" + boundary)
		w(fmt.Sprintf(`Content-Type: %s; name="%s"`, ct, a.Filename))
		w("Content-Transfer-Encoding: base64")
		w(fmt.Sprintf(`Content-Disposition: attachment; filename="%s"`, a.Filename))
		w("")
		enc := base64.StdEncoding.EncodeToString(a.Data)
		for len(enc) > 76 {
			w(enc[:76])
			enc = enc[76:]
		}
		w(enc)
	}
	w("--" + boundary + "--")
	return b.Bytes(), msgID
}

// SendWithAttachments delivers msg and returns its Message-ID. Unlike Send,
// it fails with ErrNotConfigured when SMTP is off instead of silently dropping.
func (s *Sender) SendWithAttachments(msg Message) (string, error) {
	if s == nil {
		return "", ErrNotConfigured
	}
	if msg.From == "" {
		msg.From = s.cfg.From
	}
	domain := ""
	if i := strings.LastIndex(s.cfg.From, "@"); i >= 0 {
		domain = strings.Trim(s.cfg.From[i+1:], "> ")
	}
	raw, id := BuildMIME(msg, domain, time.Now())
	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}
	rcpts := append(append([]string{}, msg.To...), msg.Cc...)
	if err := smtp.SendMail(fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), auth, s.cfg.From, rcpts, raw); err != nil {
		s.logger.Error("failed to send email with attachments", "subject", msg.Subject, "error", err)
		return "", err
	}
	s.logger.Info("email with attachments sent", "subject", msg.Subject, "attachments", len(msg.Attachments))
	return id, nil
}
