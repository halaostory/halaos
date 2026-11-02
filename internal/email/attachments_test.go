package email

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestBuildMIME_ParsesWithAttachment(t *testing.T) {
	raw, id := BuildMIME(Message{
		From: "hr@acme.ph", To: []string{"esubmission@bir.gov.ph"}, Cc: []string{"cfo@acme.ph"},
		Subject: "1604-C Alphalist – ACME", TextBody: "Please see attached.",
		Attachments: []Attachment{{Filename: "alphalist.dat", Data: bytes.Repeat([]byte("A"), 200)}},
	}, "acme.ph", time.Date(2026, 1, 20, 9, 0, 0, 0, time.UTC))
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.Get("Message-Id") != id || !strings.HasSuffix(id, "@acme.ph>") {
		t.Errorf("message id %q / %q", m.Header.Get("Message-Id"), id)
	}
	_, params, _ := strings.Cut(m.Header.Get("Content-Type"), "boundary=")
	r := multipart.NewReader(m.Body, strings.Trim(params, `"`))
	_, _ = r.NextPart() // body
	att, err := r.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if att.FileName() != "alphalist.dat" {
		t.Errorf("filename %q", att.FileName())
	}
	data, _ := io.ReadAll(att) // multipart decodes base64 transparently? no — check raw
	if len(data) == 0 {
		t.Error("empty attachment")
	}
}

func TestSendWithAttachments_NotConfigured(t *testing.T) {
	var s *Sender
	if _, err := s.SendWithAttachments(Message{}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("expected ErrNotConfigured, got %v", err)
	}
}
