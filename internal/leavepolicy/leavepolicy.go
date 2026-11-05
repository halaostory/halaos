// Package leavepolicy assigns time-off entitlements by rule, Warp-style:
// a rule gives N days of a leave type to employees matching a department,
// employment type, gender and tenure band. The most specific matching rule
// wins (highest priority, then the longest minimum tenure); without one the
// leave type's default applies. Tenure-based upgrades ("15 days after 5
// years") are picked up automatically by a daily re-apply.
package leavepolicy

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/pkg/response"
)

// Employee is what rules match on.
type Employee struct {
	ID             int64
	Name           string
	DepartmentID   *int64
	WorkplaceID    *int64
	EmploymentType string
	Gender         string
	HireDate       time.Time
}

// TenureMonths is the number of whole months since hire on day `on`.
func TenureMonths(hire, on time.Time) int {
	if on.Before(hire) {
		return 0
	}
	m := (on.Year()-hire.Year())*12 + int(on.Month()) - int(hire.Month())
	if on.Day() < hire.Day() {
		m--
	}
	return max(m, 0)
}

// Matches reports whether a rule applies to an employee on a date.
func Matches(r store.LeavePolicyRule, e Employee, on time.Time) bool {
	if !r.IsActive {
		return false
	}
	if len(r.DepartmentIds) > 0 && (e.DepartmentID == nil || !slices.Contains(r.DepartmentIds, *e.DepartmentID)) {
		return false
	}
	if len(r.WorkplaceIds) > 0 && (e.WorkplaceID == nil || !slices.Contains(r.WorkplaceIds, *e.WorkplaceID)) {
		return false
	}
	if len(r.EmploymentTypes) > 0 && !slices.Contains(r.EmploymentTypes, e.EmploymentType) {
		return false
	}
	if r.Gender != "" && !strings.EqualFold(r.Gender, e.Gender) {
		return false
	}
	t := TenureMonths(e.HireDate, on)
	if t < int(r.MinTenureMonths) {
		return false
	}
	if r.MaxTenureMonths != nil && t >= int(*r.MaxTenureMonths) {
		return false
	}
	return true
}

// Resolve returns the entitlement for one leave type and the rule that set
// it (nil = the leave type default).
func Resolve(rules []store.LeavePolicyRule, leaveTypeID int64, defaultDays float64, e Employee, on time.Time) (float64, *store.LeavePolicyRule) {
	var best *store.LeavePolicyRule
	for i := range rules {
		r := &rules[i]
		if r.LeaveTypeID != leaveTypeID || !Matches(*r, e, on) {
			continue
		}
		if best == nil || r.Priority > best.Priority || (r.Priority == best.Priority && r.MinTenureMonths > best.MinTenureMonths) {
			best = r
		}
	}
	if best == nil {
		return defaultDays, nil
	}
	return num(best.Days), best
}

func num(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(math.Round(f*10)/10, 'f', 1, 64))
	return n
}

// Change is one entitlement that differs from what is on file.
type Change struct {
	EmployeeID   int64   `json:"employee_id"`
	EmployeeName string  `json:"employee_name"`
	LeaveTypeID  int64   `json:"leave_type_id"`
	LeaveType    string  `json:"leave_type"`
	From         float64 `json:"from"`
	To           float64 `json:"to"`
	Rule         string  `json:"rule"`
}

// Service applies the rules.
type Service struct {
	q      *store.Queries
	logger *slog.Logger
	now    func() time.Time
}

// NewService builds the service.
func NewService(q *store.Queries, logger *slog.Logger) *Service {
	return &Service{q: q, logger: logger, now: time.Now}
}

// Entitlement is used when initialising a new hire's balances.
func (s *Service) Entitlement(ctx context.Context, companyID int64, lt store.LeaveType, e Employee) float64 {
	rules, err := s.q.ListLeavePolicyRules(ctx, companyID)
	if err != nil {
		return num(lt.DefaultDays)
	}
	on := s.now()
	if e.HireDate.After(on) {
		on = e.HireDate
	}
	days, _ := Resolve(rules, lt.ID, num(lt.DefaultDays), e, on)
	return days
}

// Plan computes the changes rules would make to this year's balances. Only
// entitlements that differ from what is stored are returned; with apply the
// changes are written.
func (s *Service) Plan(ctx context.Context, companyID int64, apply bool) ([]Change, error) {
	rules, err := s.q.ListLeavePolicyRules(ctx, companyID)
	if err != nil {
		return nil, err
	}
	types, err := s.q.ListLeaveTypes(ctx, companyID)
	if err != nil {
		return nil, err
	}
	emps, err := s.q.ListEmployeesForLeavePolicy(ctx, companyID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	year := int32(now.Year())
	bals, err := s.q.ListLeaveBalancesForYear(ctx, store.ListLeaveBalancesForYearParams{CompanyID: companyID, Year: year})
	if err != nil {
		return nil, err
	}
	type key struct{ e, lt int64 }
	have := map[key]float64{}
	for _, b := range bals {
		have[key{b.EmployeeID, b.LeaveTypeID}] = num(b.Earned)
	}
	changes := []Change{}
	for _, row := range emps {
		e := Employee{ID: row.ID, Name: strings.TrimSpace(row.FirstName + " " + row.LastName), DepartmentID: row.DepartmentID,
			WorkplaceID: row.WorkplaceID, EmploymentType: row.EmploymentType, Gender: row.Gender, HireDate: row.HireDate}
		for _, lt := range types {
			if !lt.IsActive {
				continue
			}
			cur, ok := have[key{e.ID, lt.ID}]
			if !ok {
				continue // balances are created at hire / year start; rules only adjust them
			}
			days, rule := Resolve(rules, lt.ID, num(lt.DefaultDays), e, now)
			if rule == nil || math.Abs(days-cur) < 0.05 {
				continue // no rule: leave manual / default entitlements alone
			}
			changes = append(changes, Change{EmployeeID: e.ID, EmployeeName: e.Name, LeaveTypeID: lt.ID, LeaveType: lt.Name, From: cur, To: days, Rule: rule.Name})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].EmployeeName != changes[j].EmployeeName {
			return changes[i].EmployeeName < changes[j].EmployeeName
		}
		return changes[i].LeaveType < changes[j].LeaveType
	})
	if apply {
		for _, c := range changes {
			if err := s.q.SetLeaveEarned(ctx, store.SetLeaveEarnedParams{CompanyID: companyID, EmployeeID: c.EmployeeID, LeaveTypeID: c.LeaveTypeID, Year: year, Earned: numeric(c.To)}); err != nil {
				return changes, err
			}
		}
	}
	return changes, nil
}

// ApplyAll re-applies every company's rules (tenure upgrades). Run daily.
func (s *Service) ApplyAll(ctx context.Context) int {
	ids, err := s.q.ListCompaniesWithLeavePolicies(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, id := range ids {
		ch, err := s.Plan(ctx, id, true)
		if err != nil {
			s.logger.Warn("leavepolicy: apply", "company_id", id, "error", err)
			continue
		}
		n += len(ch)
	}
	return n
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

// RegisterRoutes registers /leave-policies routes (admins).
func (s *Service) RegisterRoutes(protected *gin.RouterGroup) {
	g := protected.Group("/leave-policies", auth.AdminOnly())
	g.GET("/rules", s.list)
	g.POST("/rules", s.create)
	g.PUT("/rules/:id", s.update)
	g.DELETE("/rules/:id", s.remove)
	g.POST("/preview", func(c *gin.Context) { s.plan(c, false) })
	g.POST("/apply", func(c *gin.Context) { s.plan(c, true) })
}

type ruleRequest struct {
	LeaveTypeID     int64    `json:"leave_type_id" binding:"required"`
	Name            string   `json:"name" binding:"required"`
	Priority        int32    `json:"priority"`
	DepartmentIDs   []int64  `json:"department_ids"`
	WorkplaceIDs    []int64  `json:"workplace_ids"`
	EmploymentTypes []string `json:"employment_types"`
	Gender          string   `json:"gender"`
	MinTenureMonths int32    `json:"min_tenure_months"`
	MaxTenureMonths *int32   `json:"max_tenure_months"`
	Days            float64  `json:"days"`
	IsActive        *bool    `json:"is_active"`
}

func (r *ruleRequest) validate() error {
	r.Name = strings.TrimSpace(r.Name)
	switch {
	case r.Name == "" || len(r.Name) > 120:
		return errors.New("name is required (max 120 characters)")
	case r.Days < 0 || r.Days > 365:
		return errors.New("days must be between 0 and 365")
	case r.MinTenureMonths < 0 || (r.MaxTenureMonths != nil && *r.MaxTenureMonths <= r.MinTenureMonths):
		return errors.New("tenure band is invalid (max must be above min)")
	case r.Gender != "" && r.Gender != "male" && r.Gender != "female":
		return errors.New("gender must be male, female or empty")
	}
	if r.DepartmentIDs == nil {
		r.DepartmentIDs = []int64{}
	}
	if r.WorkplaceIDs == nil {
		r.WorkplaceIDs = []int64{}
	}
	if r.EmploymentTypes == nil {
		r.EmploymentTypes = []string{}
	}
	return nil
}

func (s *Service) list(c *gin.Context) {
	rows, err := s.q.ListLeavePolicyRules(c.Request.Context(), auth.GetCompanyID(c))
	if err != nil {
		response.InternalError(c, "Failed to load rules")
		return
	}
	if rows == nil {
		rows = []store.LeavePolicyRule{}
	}
	response.OK(c, rows)
}

func (s *Service) create(c *gin.Context) {
	var req ruleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "leave_type_id and name are required")
		return
	}
	if err := req.validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	active := req.IsActive == nil || *req.IsActive
	row, err := s.q.CreateLeavePolicyRule(c.Request.Context(), store.CreateLeavePolicyRuleParams{
		CompanyID: auth.GetCompanyID(c), LeaveTypeID: req.LeaveTypeID, Name: req.Name, Priority: req.Priority,
		DepartmentIds: req.DepartmentIDs, EmploymentTypes: req.EmploymentTypes, Gender: req.Gender,
		MinTenureMonths: req.MinTenureMonths, MaxTenureMonths: req.MaxTenureMonths, Days: numeric(req.Days), IsActive: active,
		WorkplaceIds: req.WorkplaceIDs,
	})
	if err != nil {
		response.BadRequest(c, "Could not save the rule (check the leave type)")
		return
	}
	response.Created(c, row)
}

func (s *Service) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	var req ruleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "leave_type_id and name are required")
		return
	}
	if err := req.validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	active := req.IsActive == nil || *req.IsActive
	row, err := s.q.UpdateLeavePolicyRule(c.Request.Context(), store.UpdateLeavePolicyRuleParams{
		ID: id, CompanyID: auth.GetCompanyID(c), LeaveTypeID: req.LeaveTypeID, Name: req.Name, Priority: req.Priority,
		DepartmentIds: req.DepartmentIDs, EmploymentTypes: req.EmploymentTypes, Gender: req.Gender,
		MinTenureMonths: req.MinTenureMonths, MaxTenureMonths: req.MaxTenureMonths, Days: numeric(req.Days), IsActive: active,
		WorkplaceIds: req.WorkplaceIDs,
	})
	if err != nil {
		response.NotFound(c, "Rule not found")
		return
	}
	response.OK(c, row)
}

func (s *Service) remove(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid id")
		return
	}
	if err := s.q.DeleteLeavePolicyRule(c.Request.Context(), store.DeleteLeavePolicyRuleParams{ID: id, CompanyID: auth.GetCompanyID(c)}); err != nil {
		response.InternalError(c, "Failed to delete")
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (s *Service) plan(c *gin.Context, apply bool) {
	changes, err := s.Plan(c.Request.Context(), auth.GetCompanyID(c), apply)
	if err != nil {
		s.logger.Error("leavepolicy: plan", "error", err)
		response.InternalError(c, "Failed to compute entitlements")
		return
	}
	response.OK(c, gin.H{"changes": changes, "applied": apply})
}

// FromEmployee adapts a stored employee (workplace may be nil).
func FromEmployee(e store.Employee, workplaceID *int64) Employee {
	g := ""
	if e.Gender != nil {
		g = *e.Gender
	}
	return Employee{ID: e.ID, Name: strings.TrimSpace(e.FirstName + " " + e.LastName), DepartmentID: e.DepartmentID,
		WorkplaceID: workplaceID, EmploymentType: e.EmploymentType, Gender: g, HireDate: e.HireDate}
}

// InitialDays is a new hire's entitlement for a leave type, as of their
// start date (or today if later).
func InitialDays(rules []store.LeavePolicyRule, lt store.LeaveType, e Employee, now time.Time) pgtype.Numeric {
	on := now
	if e.HireDate.After(on) {
		on = e.HireDate
	}
	days, rule := Resolve(rules, lt.ID, num(lt.DefaultDays), e, on)
	if rule == nil {
		return lt.DefaultDays
	}
	return numeric(days)
}
