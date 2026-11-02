// Package mockportal is a tiny fake government e-filing portal used in tests
// and local demos of the browser submission agent. It mimics the usual flow:
// login + CAPTCHA → OTP → upload return file → confirmation with reference no.
package mockportal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	Username = "employer01"
	Password = "s3cret!"
	Captcha  = "K7Q2P"
	OTP      = "246810"
)

// Portal is an in-memory fake portal.
type Portal struct {
	mu       sync.Mutex
	stage    map[string]string // session -> stage
	Received []Upload
}

// Upload is a file the portal accepted.
type Upload struct {
	Filename  string
	SHA256    string
	Reference string
	Period    string
}

// New creates a portal.
func New() *Portal { return &Portal{stage: map[string]string{}} }

const page = `<!doctype html><html><head><title>Mock Gov eFiling</title></head><body><h1>Mock Government eFiling</h1>%s</body></html>`

func (p *Portal) session(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie("sid"); err == nil {
		return c.Value
	}
	sid := fmt.Sprintf("s%d", len(p.stage)+1)
	http.SetCookie(w, &http.Cookie{Name: "sid", Value: sid, Path: "/"})
	return sid
}

// Handler returns the portal's HTTP handler.
func (p *Portal) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.session(w, r)
		fmt.Fprintf(w, page, `<form method="post" action="/login">
<label>User ID <input id="username" name="username"></label>
<label>Password <input id="password" name="password" type="password"></label>
<div><span id="captcha-img" style="display:inline-block;padding:6px 14px;background:#ddd;font:italic bold 26px serif;letter-spacing:6px;text-decoration:line-through;transform:skewX(-12deg)">`+Captcha+`</span> <span id="captcha-hint">Type the characters shown</span></div>
<input id="captcha" name="captcha">
<button id="login-btn" type="submit">Log in</button></form>`)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		sid := p.session(w, r)
		if r.FormValue("username") != Username || r.FormValue("password") != Password || r.FormValue("captcha") != Captcha {
			fmt.Fprintf(w, page, `<p id="error">Invalid credentials or captcha</p>`)
			return
		}
		p.stage[sid] = "otp"
		fmt.Fprintf(w, page, `<form method="post" action="/otp"><p>A one-time PIN was sent to your registered mobile.</p>
<input id="otp" name="otp"><button id="otp-btn" type="submit">Verify</button></form>`)
	})
	mux.HandleFunc("/otp", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		sid := p.session(w, r)
		if p.stage[sid] != "otp" || r.FormValue("otp") != OTP {
			fmt.Fprintf(w, page, `<p id="error">Invalid OTP</p>`)
			return
		}
		p.stage[sid] = "in"
		fmt.Fprintf(w, page, `<form method="post" action="/upload" enctype="multipart/form-data">
<label>Return period <select id="period" name="period"><option value="">--</option>`+periodOptions()+`</select></label>
<input id="file" name="file" type="file"><button id="submit-btn" type="submit">Submit return</button></form>`)
	})
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		sid := p.session(w, r)
		if p.stage[sid] != "in" {
			http.Error(w, "not logged in", http.StatusUnauthorized)
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			fmt.Fprintf(w, page, `<p id="error">No file</p>`)
			return
		}
		defer f.Close()
		h := sha256.New()
		_, _ = io.Copy(h, f)
		ref := fmt.Sprintf("MOCK-%06d", len(p.Received)+1)
		p.Received = append(p.Received, Upload{Filename: hdr.Filename, SHA256: hex.EncodeToString(h.Sum(nil)), Reference: ref, Period: r.FormValue("period")})
		fmt.Fprintf(w, page, fmt.Sprintf(`<div id="confirmation">Your return %s for period %s was received. Reference No: <b id="ref">%s</b></div>`,
			html.EscapeString(hdr.Filename), html.EscapeString(r.FormValue("period")), ref))
	})
	return mux
}

func periodOptions() string {
	var b strings.Builder
	now := time.Now()
	for i := 18; i >= 0; i-- {
		fmt.Fprintf(&b, "<option>%s</option>", now.AddDate(0, -i, 0).Format("2006-01"))
	}
	return b.String()
}
