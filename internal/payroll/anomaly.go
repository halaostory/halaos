package payroll

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/tonypk/aigonhr/internal/store"
)

// AnomalyType classifies the kind of anomaly detected.
type AnomalyType string

const (
	AnomalyGrossPayDeviation     AnomalyType = "gross_pay_deviation"
	AnomalyNetPayDeviation       AnomalyType = "net_pay_deviation"
	AnomalyOTExcessive           AnomalyType = "excessive_overtime"
	AnomalyZeroContribution      AnomalyType = "zero_contribution"
	AnomalyZeroTax               AnomalyType = "zero_withholding_tax"
	AnomalyNegativeNet           AnomalyType = "negative_net_pay"
	AnomalyWorkDaysExceeded      AnomalyType = "work_days_exceeded"
	AnomalyHighLateDeduction     AnomalyType = "high_late_deduction"
	AnomalyDuplicateEmployee     AnomalyType = "duplicate_employee"
	AnomalySalaryJump            AnomalyType = "salary_jump"
)

// Severity indicates how serious the anomaly is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// Anomaly represents a single detected anomaly in a payroll run.
type Anomaly struct {
	Type         AnomalyType `json:"type"`
	Severity     Severity    `json:"severity"`
	EmployeeID   int64       `json:"employee_id"`
	EmployeeName string      `json:"employee_name"`
	EmployeeNo   string      `json:"employee_no"`
	Description  string      `json:"description"`
	CurrentValue float64     `json:"current_value"`
	ExpectedValue float64    `json:"expected_value,omitempty"`
	Deviation    float64     `json:"deviation_pct,omitempty"`
}

// AnomalyReport is the complete anomaly detection result for a payroll run.
type AnomalyReport struct {
	RunID      int64     `json:"run_id"`
	CycleID    int64     `json:"cycle_id"`
	TotalItems int       `json:"total_items"`
	Anomalies  []Anomaly `json:"anomalies"`
	Summary    struct {
		Critical int `json:"critical"`
		High     int `json:"high"`
		Medium   int `json:"medium"`
		Low      int `json:"low"`
	} `json:"summary"`
}

// DetectAnomalies runs rule-based anomaly detection on a payroll run.
func (calc *Calculator) DetectAnomalies(ctx context.Context, runID, companyID int64) (*AnomalyReport, error) {
	// Get company for country-aware checks
	company, err := calc.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("get company: %w", err)
	}

	// Get the run
	run, err := calc.queries.GetPayrollRun(ctx, store.GetPayrollRunParams{
		ID:        runID,
		CompanyID: companyID,
	})
	if err != nil {
		return nil, fmt.Errorf("get payroll run: %w", err)
	}

	// Get items for this run
	items, err := calc.queries.GetPayrollItemsForRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get payroll items: %w", err)
	}

	report := &AnomalyReport{
		RunID:      runID,
		CycleID:    run.CycleID,
		TotalItems: len(items),
	}

	// Check for duplicate employees
	employeeCount := make(map[int64]int)
	for _, item := range items {
		employeeCount[item.EmployeeID]++
	}

	for _, item := range items {
		name := item.FirstName + " " + item.LastName

		// Rule 1: Duplicate employee in same run
		if employeeCount[item.EmployeeID] > 1 {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Type:         AnomalyDuplicateEmployee,
				Severity:     SeverityCritical,
				EmployeeID:   item.EmployeeID,
				EmployeeName: name,
				EmployeeNo:   item.EmployeeNo,
				Description:  fmt.Sprintf("Employee appears %d times in this payroll run", employeeCount[item.EmployeeID]),
				CurrentValue: float64(employeeCount[item.EmployeeID]),
			})
		}

		grossPay := numToF64(item.GrossPay)
		netPay := numToF64(item.NetPay)
		sssEE := numToF64(item.SssEe)
		philhealthEE := numToF64(item.PhilhealthEe)
		pagibigEE := numToF64(item.PagibigEe)
		withholdingTax := numToF64(item.WithholdingTax)
		taxableIncome := numToF64(item.TaxableIncome)
		otHours := numToF64(item.OtHours)
		workDays := numToF64(item.WorkDays)
		lateDeduction := numToF64(item.LateDeduction)

		// Rule 2: Negative net pay
		if netPay < 0 {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Type:         AnomalyNegativeNet,
				Severity:     SeverityCritical,
				EmployeeID:   item.EmployeeID,
				EmployeeName: name,
				EmployeeNo:   item.EmployeeNo,
				Description:  fmt.Sprintf("Net pay is negative: %s %.2f", company.Currency, netPay),
				CurrentValue: netPay,
			})
		}

		// Rule 3: Zero government contributions when gross > 0 (country-aware)
		if grossPay > 0 {
			var breakdown map[string]interface{}
			_ = json.Unmarshal(item.Breakdown, &breakdown)

			hasContributions := false
			switch company.Country {
			case "LKA":
				epfEE, _ := breakdown["epf_ee"].(float64)
				if epfEE > 0 {
					hasContributions = true
				}
			default: // PHL
				if sssEE > 0 || philhealthEE > 0 || pagibigEE > 0 {
					hasContributions = true
				}
			}
			if !hasContributions {
				report.Anomalies = append(report.Anomalies, Anomaly{
					Type:          AnomalyZeroContribution,
					Severity:      SeverityHigh,
					EmployeeID:    item.EmployeeID,
					EmployeeName:  name,
					EmployeeNo:    item.EmployeeNo,
					Description:   "All government contributions are zero despite positive gross pay",
					CurrentValue:  0,
					ExpectedValue: grossPay * 0.08,
				})
			}
		}

		// Rule 4: Zero withholding tax (country-aware thresholds)
		taxFreeThreshold := 20833.0 // PH monthly tax-free
		currencyLabel := "PHP"
		if company.Country == "LKA" {
			taxFreeThreshold = 150000.0 // LK APIT monthly tax-free
			currencyLabel = "LKR"
		}
		if taxableIncome > taxFreeThreshold && withholdingTax == 0 {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Type:          AnomalyZeroTax,
				Severity:      SeverityHigh,
				EmployeeID:    item.EmployeeID,
				EmployeeName:  name,
				EmployeeNo:    item.EmployeeNo,
				Description:   fmt.Sprintf("Zero withholding tax with taxable income %s %.2f (above %s %.0f threshold)", currencyLabel, taxableIncome, currencyLabel, taxFreeThreshold),
				CurrentValue:  withholdingTax,
				ExpectedValue: taxableIncome,
			})
		}

		// Rule 5: Excessive overtime (> 40 hours in a period)
		if otHours > 40 {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Type:         AnomalyOTExcessive,
				Severity:     SeverityMedium,
				EmployeeID:   item.EmployeeID,
				EmployeeName: name,
				EmployeeNo:   item.EmployeeNo,
				Description:  fmt.Sprintf("Overtime hours (%.1f) exceed 40-hour threshold", otHours),
				CurrentValue: otHours,
				ExpectedValue: 40,
			})
		}

		// Rule 6: Work days exceeding possible days in period (from breakdown)
		var breakdown map[string]interface{}
		if err := json.Unmarshal(item.Breakdown, &breakdown); err == nil {
			if wdip, ok := breakdown["working_days_in_period"].(float64); ok && wdip > 0 {
				if workDays > wdip {
					report.Anomalies = append(report.Anomalies, Anomaly{
						Type:         AnomalyWorkDaysExceeded,
						Severity:     SeverityHigh,
						EmployeeID:   item.EmployeeID,
						EmployeeName: name,
						EmployeeNo:   item.EmployeeNo,
						Description:  fmt.Sprintf("Work days (%.0f) exceed working days in period (%.0f)", workDays, wdip),
						CurrentValue: workDays,
						ExpectedValue: wdip,
					})
				}
			}
		}

		// Rule 7: Late deduction exceeds 20% of basic pay
		basicPay := numToF64(item.BasicPay)
		if basicPay > 0 && lateDeduction > basicPay*0.2 {
			report.Anomalies = append(report.Anomalies, Anomaly{
				Type:         AnomalyHighLateDeduction,
				Severity:     SeverityMedium,
				EmployeeID:   item.EmployeeID,
				EmployeeName: name,
				EmployeeNo:   item.EmployeeNo,
				Description:  fmt.Sprintf("Late deduction (%s %.2f) exceeds 20%% of basic pay (%s %.2f)", company.Currency, lateDeduction, company.Currency, basicPay),
				CurrentValue: lateDeduction,
				ExpectedValue: basicPay * 0.2,
			})
		}

		// Rule 8: Historical comparison — gross/net pay deviation > 20% from average
		history, err := calc.queries.GetEmployeePayrollHistory(ctx, store.GetEmployeePayrollHistoryParams{
			CompanyID:  companyID,
			EmployeeID: item.EmployeeID,
			Limit:      6,
		})
		if err == nil && len(history) > 1 {
			// Skip the first one (current run) if it matches
			var histGross, histNet []float64
			for _, h := range history {
				if h.RunID == runID {
					continue
				}
				histGross = append(histGross, numToF64(h.GrossPay))
				histNet = append(histNet, numToF64(h.NetPay))
			}

			if len(histGross) >= 2 {
				avgGross := average(histGross)
				avgNet := average(histNet)

				if avgGross > 0 {
					deviation := math.Abs(grossPay-avgGross) / avgGross * 100
					if deviation > 20 {
						report.Anomalies = append(report.Anomalies, Anomaly{
							Type:          AnomalyGrossPayDeviation,
							Severity:      severityForDeviation(deviation),
							EmployeeID:    item.EmployeeID,
							EmployeeName:  name,
							EmployeeNo:    item.EmployeeNo,
							Description:   fmt.Sprintf("Gross pay %s %.2f deviates %.1f%% from historical average %s %.2f", company.Currency, grossPay, deviation, company.Currency, avgGross),
							CurrentValue:  grossPay,
							ExpectedValue: avgGross,
							Deviation:     deviation,
						})
					}
				}

				if avgNet > 0 {
					deviation := math.Abs(netPay-avgNet) / avgNet * 100
					if deviation > 20 {
						report.Anomalies = append(report.Anomalies, Anomaly{
							Type:          AnomalyNetPayDeviation,
							Severity:      severityForDeviation(deviation),
							EmployeeID:    item.EmployeeID,
							EmployeeName:  name,
							EmployeeNo:    item.EmployeeNo,
							Description:   fmt.Sprintf("Net pay %s %.2f deviates %.1f%% from historical average %s %.2f", company.Currency, netPay, deviation, company.Currency, avgNet),
							CurrentValue:  netPay,
							ExpectedValue: avgNet,
							Deviation:     deviation,
						})
					}
				}

				// Rule 9: Salary jump — basic pay changes significantly
				prevBasic := numToF64(history[0].BasicPay)
				if history[0].RunID == runID && len(history) > 1 {
					prevBasic = numToF64(history[1].BasicPay)
				}
				if prevBasic > 0 {
					basicDeviation := math.Abs(basicPay-prevBasic) / prevBasic * 100
					if basicDeviation > 30 {
						report.Anomalies = append(report.Anomalies, Anomaly{
							Type:          AnomalySalaryJump,
							Severity:      SeverityMedium,
							EmployeeID:    item.EmployeeID,
							EmployeeName:  name,
							EmployeeNo:    item.EmployeeNo,
							Description:   fmt.Sprintf("Basic pay changed %.1f%% (%s %.2f → %s %.2f)", basicDeviation, company.Currency, prevBasic, company.Currency, basicPay),
							CurrentValue:  basicPay,
							ExpectedValue: prevBasic,
							Deviation:     basicDeviation,
						})
					}
				}
			}
		}
	}

	// Deduplicate (duplicate employee anomaly fires once per occurrence)
	seen := make(map[string]bool)
	var deduped []Anomaly
	for _, a := range report.Anomalies {
		key := fmt.Sprintf("%s:%d:%s", a.Type, a.EmployeeID, a.Description)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, a)
		}
	}
	report.Anomalies = deduped

	// Compute summary
	for _, a := range report.Anomalies {
		switch a.Severity {
		case SeverityCritical:
			report.Summary.Critical++
		case SeverityHigh:
			report.Summary.High++
		case SeverityMedium:
			report.Summary.Medium++
		case SeverityLow:
			report.Summary.Low++
		}
	}

	return report, nil
}

func numToF64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func severityForDeviation(pct float64) Severity {
	switch {
	case pct > 50:
		return SeverityCritical
	case pct > 35:
		return SeverityHigh
	case pct > 20:
		return SeverityMedium
	default:
		return SeverityLow
	}
}
