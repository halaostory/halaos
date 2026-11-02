package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// SetAI enables the AI inbox brief (optional).
func (h *Handler) SetAI(p provider.Provider) { h.ai = p }

// Brief is a prioritised summary of the HR inbox.
type Brief struct {
	Summary   string   `json:"summary"`
	Actions   []string `json:"actions"`
	Generated string   `json:"generated_by"` // ai | rules
}

var kindLabel = map[string]string{
	"gov_human_input": "government portal waiting for a CAPTCHA/OTP", "gov_filing_failed": "failed government filing",
	"gov_filing_approval": "government filing ready for approval", "gov_filing_blocked": "government filing blocked by pre-flight issues",
	"gov_filing_manual": "approved filing to submit manually", "leave_approval": "leave approval", "overtime_approval": "overtime approval",
	"expense_approval": "expense approval", "attendance_correction": "attendance correction", "onboarding_task": "onboarding task",
	"offboarding_task": "offboarding task", "document_expiring": "expiring document", "compliance_alert": "compliance alert",
	"contract_milestone": "contract milestone", "provisioning_approval": "app provisioning issue", "device_action": "device action",
	"final_pay_approval": "final pay to review", "self_onboarding_review": "new-hire details to review", "signature_declined": "declined signature",
}

// kindWeight orders what HR should handle first (time-critical external actions first).
var kindWeight = map[string]int{
	"gov_human_input": 100, "gov_filing_failed": 90, "gov_filing_approval": 80, "gov_filing_manual": 75, "signature_declined": 70,
	"device_action": 65, "final_pay_approval": 60, "gov_filing_blocked": 55, "compliance_alert": 50, "provisioning_approval": 45,
	"leave_approval": 40, "overtime_approval": 35, "expense_approval": 30, "attendance_correction": 25, "self_onboarding_review": 20,
	"document_expiring": 15, "contract_milestone": 12, "onboarding_task": 10, "offboarding_task": 10,
}

func rulesBrief(items []store.ListHRInboxItemsRow) Brief {
	if len(items) == 0 {
		return Brief{Summary: "Nothing is waiting on HR right now.", Actions: []string{}, Generated: "rules"}
	}
	counts := map[string]int{}
	high := map[string]int{}
	for _, it := range items {
		counts[it.Kind]++
		if it.Priority == "high" {
			high[it.Kind]++
		}
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool {
		wi, wj := kindWeight[kinds[i]]+high[kinds[i]]*5, kindWeight[kinds[j]]+high[kinds[j]]*5
		if wi != wj {
			return wi > wj
		}
		return kinds[i] < kinds[j]
	})
	var acts []string
	for _, k := range kinds {
		label := kindLabel[k]
		if label == "" {
			label = strings.ReplaceAll(k, "_", " ")
		}
		a := fmt.Sprintf("%d × %s", counts[k], label)
		if high[k] > 0 {
			a += fmt.Sprintf(" (%d urgent)", high[k])
		}
		acts = append(acts, a)
		if len(acts) == 5 {
			break
		}
	}
	urgent := 0
	for _, n := range high {
		urgent += n
	}
	return Brief{Summary: fmt.Sprintf("%d items are waiting on HR, %d of them urgent. Start with: %s.", len(items), urgent, acts[0]), Actions: acts, Generated: "rules"}
}

const briefSystem = `You are the HR operations co-pilot of HalaOS. You get a JSON list of open inbox items
(kind, title, due date, priority; no personal data). Write a short brief for the HR admin:
"summary": 2 sentences on what matters today and why (deadlines, legal/statutory risk first),
"actions": up to 5 concrete next steps, most urgent first, each under 20 words.
Reply with JSON only: {"summary": "...", "actions": ["..."]}`

func (h *Handler) aiBrief(ctx context.Context, items []store.ListHRInboxItemsRow) (*Brief, error) {
	type it struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Due      string `json:"due,omitempty"`
		Priority string `json:"priority"`
	}
	in := make([]it, 0, len(items))
	for i, x := range items {
		if i == 40 {
			break
		}
		in = append(in, it{Kind: x.Kind, Title: x.Title, Due: x.DueDate, Priority: x.Priority})
	}
	body, _ := json.Marshal(map[string]any{"today": time.Now().Format("2006-01-02"), "total_items": len(items), "items": in})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := h.ai.Generate(ctx, provider.Request{System: briefSystem, Messages: []provider.Message{{Role: provider.RoleUser, Content: string(body)}}, MaxTokens: 500})
	if err != nil {
		return nil, err
	}
	raw := resp.Content
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var b Brief
	if err := json.Unmarshal([]byte(raw), &b); err != nil || b.Summary == "" {
		return nil, fmt.Errorf("unparseable brief")
	}
	if len(b.Actions) > 5 {
		b.Actions = b.Actions[:5]
	}
	b.Generated = "ai"
	return &b, nil
}

// GetInboxBrief handles GET /inbox/brief.
func (h *Handler) GetInboxBrief(c *gin.Context) {
	items, err := h.queries.ListHRInboxItems(c.Request.Context(), store.ListHRInboxItemsParams{CompanyID: auth.GetCompanyID(c), Lim: 200})
	if err != nil {
		response.InternalError(c, "Failed to load inbox")
		return
	}
	if h.ai != nil && len(items) > 0 {
		if b, err := h.aiBrief(c.Request.Context(), items); err == nil {
			response.OK(c, b)
			return
		} else {
			h.logger.Warn("lifecycle: ai brief failed, using rules", "error", err)
		}
	}
	response.OK(c, rulesBrief(items))
}
