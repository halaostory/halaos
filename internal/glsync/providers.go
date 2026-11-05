// Package glsync pushes payroll journal entries straight into Xero or
// QuickBooks Online over their OAuth 2.0 APIs (Warp: "after each payroll
// run, Warp automatically creates journal entries"). Xero journals are
// created as drafts so the accountant posts them; QuickBooks has no draft
// journals, so entries are created directly and can be voided there.
package glsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/payroll"
)

// Tokens is an OAuth token pair.
type Tokens struct {
	Access    string
	Refresh   string
	ExpiresAt time.Time
}

// Endpoints are overridable for tests and sandboxes.
type Endpoints struct {
	Authorize   string
	Token       string
	API         string
	Connections string // Xero only
}

// Provider is one accounting system.
type Provider struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scope        string
	EP           Endpoints
	HTTP         *http.Client
}

// XeroEndpoints are Xero's production endpoints.
var XeroEndpoints = Endpoints{
	Authorize:   "https://login.xero.com/identity/connect/authorize",
	Token:       "https://identity.xero.com/connect/token",
	API:         "https://api.xero.com/api.xro/2.0",
	Connections: "https://api.xero.com/connections",
}

// QuickBooksEndpoints are Intuit's production endpoints.
var QuickBooksEndpoints = Endpoints{
	Authorize: "https://appcenter.intuit.com/connect/oauth2",
	Token:     "https://oauth.platform.intuit.com/oauth2/v1/tokens/bearer",
	API:       "https://quickbooks.api.intuit.com",
}

// Configured reports whether the server has app credentials for it.
func (p *Provider) Configured() bool { return p != nil && p.ClientID != "" && p.ClientSecret != "" }

// AuthURL is where the admin consents.
func (p *Provider) AuthURL(state string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURL},
		"scope":         {p.Scope},
		"state":         {state},
	}
	return p.EP.Authorize + "?" + q.Encode()
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func (p *Provider) token(ctx context.Context, form url.Values) (Tokens, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.EP.Token, strings.NewReader(form.Encode()))
	req.SetBasicAuth(p.ClientID, p.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	var t tokenResp
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&t)
	if resp.StatusCode != http.StatusOK || t.AccessToken == "" {
		return Tokens{}, fmt.Errorf("%s token: %s %s", p.Name, resp.Status, t.Error)
	}
	if t.ExpiresIn == 0 {
		t.ExpiresIn = 1800
	}
	return Tokens{Access: t.AccessToken, Refresh: t.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(t.ExpiresIn-60) * time.Second)}, nil
}

// Exchange trades the authorization code for tokens.
func (p *Provider) Exchange(ctx context.Context, code string) (Tokens, error) {
	return p.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.RedirectURL}})
}

// Refresh renews an expired access token.
func (p *Provider) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	t, err := p.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if err == nil && t.Refresh == "" {
		t.Refresh = refresh
	}
	return t, err
}

func (p *Provider) call(ctx context.Context, method, u, access string, headers map[string]string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("%s %s: %s %s", p.Name, method, resp.Status, msg)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// XeroTenant returns the first organisation the connection can access.
func (p *Provider) XeroTenant(ctx context.Context, access string) (id, name string, err error) {
	var conns []struct {
		TenantID   string `json:"tenantId"`
		TenantName string `json:"tenantName"`
		TenantType string `json:"tenantType"`
	}
	if err := p.call(ctx, http.MethodGet, p.EP.Connections, access, nil, nil, &conns); err != nil {
		return "", "", err
	}
	for _, c := range conns {
		if c.TenantType == "" || c.TenantType == "ORGANISATION" {
			return c.TenantID, c.TenantName, nil
		}
	}
	return "", "", errors.New("xero: the connection has no organisation")
}

// PushXero creates a draft manual journal and returns its ID.
func (p *Provider) PushXero(ctx context.Context, access, tenant string, j payroll.Journal) (string, error) {
	type line struct {
		LineAmount  float64 `json:"LineAmount"`
		AccountCode string  `json:"AccountCode"`
		Description string  `json:"Description"`
	}
	var lines []line
	for _, l := range j.Lines {
		lines = append(lines, line{LineAmount: round2(l.Debit - l.Credit), AccountCode: l.AccountCode, Description: l.Description})
	}
	body := map[string]any{"ManualJournals": []map[string]any{{
		"Narration": j.Memo, "Date": j.Date, "Status": "DRAFT", "LineAmountTypes": "NoTax", "JournalLines": lines,
	}}}
	var out struct {
		ManualJournals []struct {
			ManualJournalID string `json:"ManualJournalID"`
		} `json:"ManualJournals"`
	}
	if err := p.call(ctx, http.MethodPut, p.EP.API+"/ManualJournals", access, map[string]string{"xero-tenant-id": tenant}, body, &out); err != nil {
		return "", err
	}
	if len(out.ManualJournals) == 0 || out.ManualJournals[0].ManualJournalID == "" {
		return "", errors.New("xero: no journal id returned")
	}
	return out.ManualJournals[0].ManualJournalID, nil
}

// PushQuickBooks creates a journal entry (accounts matched by account
// number, then by name) and returns its ID.
func (p *Provider) PushQuickBooks(ctx context.Context, access, realm string, j payroll.Journal) (string, error) {
	base := fmt.Sprintf("%s/v3/company/%s", strings.TrimRight(p.EP.API, "/"), url.PathEscape(realm))
	var accts struct {
		QueryResponse struct {
			Account []struct {
				ID      string `json:"Id"`
				Name    string `json:"Name"`
				AcctNum string `json:"AcctNum"`
			} `json:"Account"`
		} `json:"QueryResponse"`
	}
	q := url.QueryEscape("select Id, Name, AcctNum from Account maxresults 1000")
	if err := p.call(ctx, http.MethodGet, base+"/query?query="+q, access, nil, nil, &accts); err != nil {
		return "", err
	}
	byNum, byName := map[string]string{}, map[string]string{}
	for _, a := range accts.QueryResponse.Account {
		if a.AcctNum != "" {
			byNum[a.AcctNum] = a.ID
		}
		byName[strings.ToLower(a.Name)] = a.ID
	}
	var lines []map[string]any
	var missing []string
	for _, l := range j.Lines {
		id := byNum[l.AccountCode]
		if id == "" {
			id = byName[strings.ToLower(l.AccountName)]
		}
		if id == "" {
			missing = append(missing, l.AccountCode+" "+l.AccountName)
			continue
		}
		posting, amt := "Debit", l.Debit
		if l.Credit > 0 {
			posting, amt = "Credit", l.Credit
		}
		lines = append(lines, map[string]any{
			"Amount": round2(amt), "DetailType": "JournalEntryLineDetail", "Description": l.Description,
			"JournalEntryLineDetail": map[string]any{"PostingType": posting, "AccountRef": map[string]string{"value": id}},
		})
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("quickbooks has no account for: %s — map them under Payroll → Journal", strings.Join(missing, "; "))
	}
	var out struct {
		JournalEntry struct {
			ID string `json:"Id"`
		} `json:"JournalEntry"`
	}
	body := map[string]any{"TxnDate": j.Date, "DocNumber": j.Number, "PrivateNote": j.Memo, "Line": lines}
	if err := p.call(ctx, http.MethodPost, base+"/journalentry?minorversion=70", access, nil, body, &out); err != nil {
		return "", err
	}
	if out.JournalEntry.ID == "" {
		return "", errors.New("quickbooks: no journal id returned")
	}
	return out.JournalEntry.ID, nil
}

func round2(v float64) float64 {
	if v < 0 {
		return -float64(int64(-v*100+0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}
