package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
	"testing"
	"time"
)

func pageCount(b []byte) int {
	return len(regexp.MustCompile(`/Type /Page\b`).FindAll(b, -1))
}

func TestGenerateSignedDocument_TwoPages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 40))
	for x := 10; x < 110; x++ {
		img.Set(x, 20, color.Black)
	}
	var sig bytes.Buffer
	_ = png.Encode(&sig, img)
	for _, s := range [][]byte{nil, sig.Bytes()} {
		out, err := GenerateSignedDocument(SignedDocument{
			CompanyName: "Acme", Title: "Offer of Employment", Body: "Dear Ana,\n\n## Terms\nWelcome — “quoted” text.",
			SignerName: "Ana Cruz", SignedName: "Ana Cruz", SignaturePNG: s, SignedAt: time.Now(), SentAt: time.Now(),
			RequestID: 1, BodySHA256: strings.Repeat("a", 64), SignedIP: "1.2.3.4", UserAgent: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := pageCount(out); n != 2 {
			t.Errorf("expected document + audit page, got %d pages", n)
		}
	}
}
