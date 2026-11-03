package glsync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/halaostory/halaos/internal/payroll"
)

// NetSuite uses token-based authentication (OAuth 1.0a, HMAC-SHA256) and
// the REST Record API. Accounts are matched by number through SuiteQL.

// NetSuiteCreds are the integration record + access token of an account.
type NetSuiteCreds struct {
	AccountID      string `json:"account_id"` // e.g. 1234567 or 1234567_SB1
	ConsumerKey    string `json:"consumer_key"`
	ConsumerSecret string `json:"consumer_secret"`
	TokenID        string `json:"token_id"`
	TokenSecret    string `json:"token_secret"`
	SubsidiaryID   string `json:"subsidiary_id,omitempty"`
}

// NetSuite is a minimal SuiteTalk REST client.
type NetSuite struct {
	Creds NetSuiteCreds
	Base  string // default https://<account>.suitetalk.api.netsuite.com
	HTTP  *http.Client
	now   func() time.Time
	nonce func() string
}

func (n *NetSuite) base() string {
	if n.Base != "" {
		return strings.TrimRight(n.Base, "/")
	}
	host := strings.ToLower(strings.ReplaceAll(n.Creds.AccountID, "_", "-"))
	return "https://" + host + ".suitetalk.api.netsuite.com"
}

func pctEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// authHeader builds the OAuth 1.0a header (realm = account ID, upper case).
func (n *NetSuite) authHeader(method, rawURL string) string {
	ts := strconv.FormatInt(n.nowFn().Unix(), 10)
	nonce := n.nonceFn()
	params := map[string]string{
		"oauth_consumer_key":     n.Creds.ConsumerKey,
		"oauth_token":            n.Creds.TokenID,
		"oauth_signature_method": "HMAC-SHA256",
		"oauth_timestamp":        ts,
		"oauth_nonce":            nonce,
		"oauth_version":          "1.0",
	}
	u, _ := url.Parse(rawURL)
	all := map[string][]string{}
	for k, v := range params {
		all[k] = []string{v}
	}
	for k, vs := range u.Query() {
		all[k] = append(all[k], vs...)
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		vs := all[k]
		sort.Strings(vs)
		for _, v := range vs {
			pairs = append(pairs, pctEncode(k)+"="+pctEncode(v))
		}
	}
	baseURL := u.Scheme + "://" + u.Host + u.Path
	sigBase := strings.ToUpper(method) + "&" + pctEncode(baseURL) + "&" + pctEncode(strings.Join(pairs, "&"))
	mac := hmac.New(sha256.New, []byte(pctEncode(n.Creds.ConsumerSecret)+"&"+pctEncode(n.Creds.TokenSecret)))
	mac.Write([]byte(sigBase))
	params["oauth_signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	var hdr []string
	for _, k := range []string{"oauth_consumer_key", "oauth_token", "oauth_signature_method", "oauth_timestamp", "oauth_nonce", "oauth_version", "oauth_signature"} {
		hdr = append(hdr, fmt.Sprintf(`%s="%s"`, k, pctEncode(params[k])))
	}
	return fmt.Sprintf(`OAuth realm="%s",%s`, strings.ToUpper(n.Creds.AccountID), strings.Join(hdr, ","))
}

func (n *NetSuite) nowFn() time.Time {
	if n.now != nil {
		return n.now()
	}
	return time.Now()
}

func (n *NetSuite) nonceFn() string {
	if n.nonce != nil {
		return n.nonce()
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (n *NetSuite) call(ctx context.Context, method, path string, body any, extra map[string]string) (*http.Response, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	u := n.base() + path
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", n.authHeader(method, u))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("netsuite: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return resp, raw, fmt.Errorf("netsuite %s %s: %s %s", method, path, resp.Status, msg)
	}
	return resp, raw, nil
}

// accounts maps account number → internal ID.
func (n *NetSuite) accounts(ctx context.Context) (map[string]string, map[string]string, error) {
	_, raw, err := n.call(ctx, http.MethodPost, "/services/rest/query/v1/suiteql?limit=1000",
		map[string]string{"q": "SELECT id, acctnumber, accountsearchdisplaynamecopy AS name FROM account WHERE isinactive = 'F'"},
		map[string]string{"Prefer": "transient"})
	if err != nil {
		return nil, nil, err
	}
	var out struct {
		Items []struct {
			ID         string `json:"id"`
			AcctNumber string `json:"acctnumber"`
			Name       string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, err
	}
	byNum, byName := map[string]string{}, map[string]string{}
	for _, a := range out.Items {
		if a.AcctNumber != "" {
			byNum[a.AcctNumber] = a.ID
		}
		byName[strings.ToLower(a.Name)] = a.ID
	}
	return byNum, byName, nil
}

// Check verifies the credentials with a trivial SuiteQL query.
func (n *NetSuite) Check(ctx context.Context) error {
	_, _, err := n.accounts(ctx)
	return err
}

// Push creates a journal entry and returns its internal ID.
func (n *NetSuite) Push(ctx context.Context, j payroll.Journal) (string, error) {
	byNum, byName, err := n.accounts(ctx)
	if err != nil {
		return "", err
	}
	var items []map[string]any
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
		it := map[string]any{"account": map[string]string{"id": id}, "memo": l.Description}
		if l.Debit > 0 {
			it["debit"] = round2(l.Debit)
		} else {
			it["credit"] = round2(l.Credit)
		}
		items = append(items, it)
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("netsuite has no account for: %s — map them under Payroll → Journal", strings.Join(missing, "; "))
	}
	body := map[string]any{"tranDate": j.Date, "memo": j.Memo, "externalId": "halapeople-" + j.Number, "line": map[string]any{"items": items}}
	if n.Creds.SubsidiaryID != "" {
		body["subsidiary"] = map[string]string{"id": n.Creds.SubsidiaryID}
	}
	resp, _, err := n.call(ctx, http.MethodPost, "/services/rest/record/v1/journalEntry", body, nil)
	if err != nil {
		return "", err
	}
	// The new record's URL is in Location: …/journalEntry/<id>
	loc := resp.Header.Get("Location")
	if i := strings.LastIndex(loc, "/"); i >= 0 && i < len(loc)-1 {
		return loc[i+1:], nil
	}
	return "", errors.New("netsuite: no record id returned")
}
