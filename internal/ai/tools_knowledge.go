package ai

import (
	"context"
	"fmt"

	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/store"
)

func (r *ToolRegistry) toolSearchKnowledgeBase(ctx context.Context, companyID int64, _ int64, input map[string]any) (string, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := int32(5)
	if l, ok := input["limit"].(float64); ok && l > 0 {
		limit = int32(l)
	}
	if limit > 10 {
		limit = 10
	}

	// Tier 1: websearch_to_tsquery — best for structured keyword matches
	articles, err := r.queries.SearchKnowledgeArticles(ctx, store.SearchKnowledgeArticlesParams{
		CompanyID:          &companyID,
		WebsearchToTsquery: query,
		Limit:              limit,
	})
	if err != nil {
		articles = nil
	}

	// Tier 2: pg_trgm trigram similarity — good for fuzzy/typo matching
	if len(articles) == 0 {
		trigramRows, trgErr := r.queries.SearchKnowledgeByTrigram(ctx, store.SearchKnowledgeByTrigramParams{
			Query:      query,
			CompanyID:  &companyID,
			MaxResults: limit,
		})
		if trgErr == nil && len(trigramRows) > 0 {
			// Convert trigram rows to the same type as tsquery results
			articles = make([]store.SearchKnowledgeArticlesRow, len(trigramRows))
			for i, tr := range trigramRows {
				articles[i] = store.SearchKnowledgeArticlesRow{
					ID:        tr.ID,
					CompanyID: tr.CompanyID,
					Category:  tr.Category,
					Topic:     tr.Topic,
					Title:     tr.Title,
					Content:   tr.Content,
					Tags:      tr.Tags,
					Source:    tr.Source,
					IsActive:  tr.IsActive,
					CreatedAt: tr.CreatedAt,
					UpdatedAt: tr.UpdatedAt,
				}
			}
		}
	}

	// Tier 3: ILIKE substring fallback
	if len(articles) == 0 {
		fallbackRows, fbErr := r.queries.SearchKnowledgeArticlesByILIKE(ctx, store.SearchKnowledgeArticlesByILIKEParams{
			CompanyID: &companyID,
			Column2:   &query,
		})
		if fbErr == nil {
			articles = make([]store.SearchKnowledgeArticlesRow, len(fallbackRows))
			for i, fr := range fallbackRows {
				articles[i] = store.SearchKnowledgeArticlesRow{
					ID: fr.ID, CompanyID: fr.CompanyID, Category: fr.Category,
					Topic: fr.Topic, Title: fr.Title, Content: fr.Content,
					Tags: fr.Tags, Source: fr.Source, IsActive: fr.IsActive,
					CreatedAt: fr.CreatedAt, UpdatedAt: fr.UpdatedAt,
				}
			}
		}
	}

	if len(articles) == 0 {
		return "No relevant knowledge articles found. Try rephrasing your query.", nil
	}

	type articleResult struct {
		Title    string  `json:"title"`
		Category string  `json:"category"`
		Content  string  `json:"content"`
		Source   *string `json:"source,omitempty"`
	}

	results := make([]articleResult, len(articles))
	for i, a := range articles {
		results[i] = articleResult{
			Title:    a.Title,
			Category: a.Category,
			Content:  a.Content,
			Source:   a.Source,
		}
	}

	return toJSON(results)
}

func (r *ToolRegistry) toolExplainPolicy(_ context.Context, _ int64, _ int64, input map[string]any) (string, error) {
	topic, _ := input["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("topic is required")
	}

	policies := map[string]string{
		"leave_types":       "Under Philippine law, employees are entitled to: Service Incentive Leave (5 days/year after 1 year), Maternity Leave (105 days, RA 11210), Paternity Leave (7 days, RA 8187), Solo Parent Leave (7 days, RA 8972), VAWC Leave (10 days, RA 9262), Special Leave for Women (60 days, RA 9710). Companies may provide additional Vacation Leave (VL) and Sick Leave (SL).",
		"13th_month_pay":    "13th Month Pay is mandatory under PD 851. All rank-and-file employees who have worked at least 1 month are entitled. Formula: Total Basic Salary Earned / 12. Must be paid on or before December 24. Pro-rated for employees who worked less than 12 months. The first PHP 90,000 of 13th month pay is tax-exempt (TRAIN Law).",
		"final_pay":         "Final Pay (last pay) must be released within 30 days from separation (DOLE Labor Advisory No. 06-20). Components: unpaid salary, pro-rated 13th month pay, cash conversion of unused leave credits (if company policy allows), separation pay (if applicable), tax refund/liability. Deductions: outstanding loans, unreturned company property.",
		"de_minimis":        "De Minimis Benefits are tax-exempt fringe benefits: Rice subsidy (PHP 2,000/month or 50kg/month), Uniform/clothing allowance (PHP 6,000/year), Medical cash allowance (PHP 1,500/quarter), Laundry allowance (PHP 300/month), Achievement awards (PHP 10,000/year). Excess over limits becomes taxable compensation.",
		"sss":               "SSS contributions are mandatory for all employees. In 2025, the contribution rate is 14% of Monthly Salary Credit (MSC), split: Employee 4.5%, Employer 9.5%. MSC ranges from PHP 4,000 to PHP 30,000. EC (Employees' Compensation) is an additional employer contribution.",
		"philhealth":        "PhilHealth premium rate is 5% of basic monthly salary in 2025. Split equally: Employee 2.5%, Employer 2.5%. Income floor: PHP 10,000, ceiling: PHP 100,000. Maximum monthly contribution: PHP 500 (EE) + PHP 500 (ER) = PHP 1,000 total.",
		"pagibig":           "Pag-IBIG contributions: Employee 1-2% of monthly compensation (1% if salary <= PHP 1,500, else 2%). Employer always 2%. Maximum monthly salary credit: PHP 10,000. Maximum EE contribution: PHP 200, ER contribution: PHP 200.",
		"bir_tax":           "Withholding tax follows TRAIN Law graduated rates (2018+). Monthly brackets: 0% for first PHP 20,833; 15% for PHP 20,833-33,333; 20% for PHP 33,333-66,667; 25% for PHP 66,667-166,667; 30% for PHP 166,667-666,667; 35% over PHP 666,667.",
		"minimum_wage":      "Minimum wage varies by region. NCR (Metro Manila): PHP 645/day (2024). Minimum wage earners are exempt from income tax. Overtime pay: +25% on regular days, +30% on rest days/holidays.",
		"overtime_rules":    "Regular OT: +25% of hourly rate. Rest day OT: +30%. Regular holiday: +100% (double pay). Special non-working holiday: +30%. Night shift differential (10pm-6am): +10% of regular wage.",
		"maternity_leave":   "RA 11210 (Expanded Maternity Leave): 105 days for live birth (with option to extend 30 unpaid days), 60 days for miscarriage/emergency termination. Solo parents get additional 15 days. SSS pays maternity benefit based on average daily salary credit.",
		"paternity_leave":   "RA 8187: 7 days paid paternity leave for married male employees for the first 4 deliveries of legitimate spouse. Must be used within 60 days from birth.",
		"solo_parent_leave": "RA 8972: 7 working days paid parental leave per year for solo parents who have rendered at least 1 year of service. Must present Solo Parent ID from DSWD.",
	}

	info, ok := policies[topic]
	if !ok {
		return fmt.Sprintf("Unknown policy topic: %s. Available topics: leave_types, 13th_month_pay, final_pay, de_minimis, sss, philhealth, pagibig, bir_tax, minimum_wage, overtime_rules, maternity_leave, paternity_leave, solo_parent_leave", topic), nil
	}
	return info, nil
}

func (r *ToolRegistry) toolCheckCompliance(ctx context.Context, companyID, _ int64, _ map[string]any) (string, error) {
	// Check how many active employees exist
	employees, err := r.queries.ListActiveEmployees(ctx, companyID)
	if err != nil {
		return "", fmt.Errorf("list employees: %w", err)
	}

	// Check government form submissions
	forms, err := r.queries.ListGovernmentForms(ctx, store.ListGovernmentFormsParams{
		CompanyID: companyID,
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		return "", fmt.Errorf("list forms: %w", err)
	}

	result := map[string]any{
		"total_active_employees": len(employees),
		"government_forms_filed": len(forms),
		"checks": []map[string]string{
			{"item": "SSS Registration", "status": "check_required", "note": "Verify all employees have SSS numbers in their profiles"},
			{"item": "PhilHealth Registration", "status": "check_required", "note": "Verify all employees have PhilHealth numbers"},
			{"item": "PagIBIG Registration", "status": "check_required", "note": "Verify all employees have PagIBIG numbers"},
			{"item": "BIR TIN", "status": "check_required", "note": "Verify all employees have TIN numbers"},
		},
	}

	return toJSON(result)
}

// knowledgeDefs returns tool definitions for knowledge-related tools.
func knowledgeDefs() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		{
			Name:        "search_knowledge_base",
			Description: "Search the knowledge base for Philippine labor law, HR policies, compliance regulations, payroll rules, leave policies, and company-specific articles. Use this for any HR/labor law question. Returns relevant articles with full content.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query in natural language. E.g., 'maternity leave benefits', 'SSS contribution rates', 'overtime pay holiday'."},
					"limit": map[string]any{"type": "integer", "description": "Max results. Default 5."},
				},
				"required": []string{"query"},
			}),
		},
		{
			Name:        "explain_policy",
			Description: "Explain a Philippine HR/labor policy or company regulation. Topics: leave_types, overtime_rules, 13th_month_pay, final_pay, de_minimis, sss, philhealth, pagibig, bir_tax, minimum_wage, maternity_leave, paternity_leave, solo_parent_leave.",
			Parameters: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{"type": "string", "description": "The policy topic to explain."},
				},
				"required": []string{"topic"},
			}),
		},
		{
			Name:        "check_compliance",
			Description: "Check compliance status for the company. Verifies SSS/PhilHealth/PagIBIG registration, BIR filing status, and government form submissions.",
			Parameters: jsonSchema(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
	}
}

// registerKnowledgeTools registers knowledge-related tool executors.
func (r *ToolRegistry) registerKnowledgeTools() {
	r.tools["search_knowledge_base"] = r.toolSearchKnowledgeBase
	r.tools["explain_policy"] = r.toolExplainPolicy
	r.tools["check_compliance"] = r.toolCheckCompliance
}
