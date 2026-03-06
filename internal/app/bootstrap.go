package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/analytics"
	"github.com/tonypk/aigonhr/internal/ai"
	"github.com/tonypk/aigonhr/internal/audit"
	"github.com/tonypk/aigonhr/internal/knowledge"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/attendance"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/company"
	"github.com/tonypk/aigonhr/internal/compliance"
	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/email"
	"github.com/tonypk/aigonhr/internal/employee"
	"github.com/tonypk/aigonhr/internal/importexport"
	"github.com/tonypk/aigonhr/internal/onboarding"
	"github.com/tonypk/aigonhr/internal/performance"
	"github.com/tonypk/aigonhr/internal/leave"
	"github.com/tonypk/aigonhr/internal/loan"
	"github.com/tonypk/aigonhr/internal/overtime"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/selfservice"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/pkg/pagination"
	"github.com/tonypk/aigonhr/pkg/response"
)

type App struct {
	Cfg     *config.Config
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Queries *store.Queries
	Router  *gin.Engine
	Logger  *slog.Logger
	Email   *email.Sender
}

func New(cfg *config.Config) (*App, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()

	// Database
	poolCfg, err := pgxpool.ParseConfig(cfg.Postgres.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	logger.Info("database connected")

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("redis not available, continuing without cache", "error", err)
	} else {
		logger.Info("redis connected")
	}

	queries := store.New(pool)

	// Router
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(logger))
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3001", "http://localhost:5173", "http://localhost:5174", "http://localhost:5175", "http://localhost:5176", "http://localhost:5177", "http://127.0.0.1:3001"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.MaxMultipartMemory = 10 << 20 // 10MB

	// Serve uploaded files (logos, etc.)
	router.Static("/uploads", "./uploads")

	// Email sender (optional)
	emailSender := email.NewSender(cfg.SMTP, logger)
	if emailSender != nil {
		logger.Info("email notifications enabled")
	} else {
		logger.Info("email notifications disabled (SMTP not configured)")
	}

	app := &App{
		Cfg:     cfg,
		Pool:    pool,
		Redis:   rdb,
		Queries: queries,
		Router:  router,
		Logger:  logger,
		Email:   emailSender,
	}

	app.setupRoutes()
	return app, nil
}

func (a *App) setupRoutes() {
	// Health check
	a.Router.GET("/health", func(c *gin.Context) {
		if err := a.Pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "db": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	jwtSvc := auth.NewJWTService(a.Cfg.JWT.Secret, a.Cfg.JWT.Expiry, a.Cfg.JWT.RefreshExpiry)

	// Services
	authHandler := auth.NewHandler(a.Queries, a.Pool, jwtSvc, a.Logger)
	companyHandler := company.NewHandler(a.Queries, a.Pool, a.Logger)
	employeeHandler := employee.NewHandler(a.Queries, a.Pool, a.Logger)
	attendanceHandler := attendance.NewHandler(a.Queries, a.Pool, a.Logger)
	leaveHandler := leave.NewHandler(a.Queries, a.Pool, a.Logger)
	overtimeHandler := overtime.NewHandler(a.Queries, a.Pool, a.Logger)
	payrollHandler := payroll.NewHandler(a.Queries, a.Pool, a.Logger)
	complianceHandler := compliance.NewHandler(a.Queries, a.Pool, a.Logger)
	onboardingHandler := onboarding.NewHandler(a.Queries, a.Pool, a.Logger)
	performanceHandler := performance.NewHandler(a.Queries, a.Pool, a.Logger)
	selfServiceHandler := selfservice.NewHandler(a.Queries, a.Pool, a.Logger)
	analyticsHandler := analytics.NewHandler(a.Queries, a.Pool, a.Logger)
	loanHandler := loan.NewHandler(a.Queries, a.Pool, a.Logger)
	notificationHandler := notification.NewHandler(a.Queries, a.Pool, a.Logger)
	knowledgeHandler := knowledge.NewHandler(a.Queries, a.Pool, a.Logger)
	auditHandler := audit.NewHandler(a.Queries, a.Pool, a.Logger)
	importExportHandler := importexport.NewHandler(a.Queries, a.Pool, a.Logger)

	// AI service (optional — only if API key configured)
	var aiHandler *ai.Handler
	if a.Cfg.AI.Enabled && a.Cfg.AI.AnthropicKey != "" {
		aiProvider := provider.NewAnthropic(a.Cfg.AI.AnthropicKey, "")
		aiService := ai.NewService(aiProvider, a.Queries, a.Pool, a.Logger)
		aiHandler = ai.NewHandler(aiService)
		a.Logger.Info("AI assistant enabled")
	} else {
		a.Logger.Info("AI assistant disabled (no API key or AI_ENABLED=false)")
	}

	api := a.Router.Group("/api/v1")

	// Public auth routes
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(auth.JWTMiddleware(jwtSvc))
	{
		// Auth (protected)
		protected.GET("/auth/me", authHandler.Me)
		protected.PUT("/auth/password", authHandler.ChangePassword)
		protected.PUT("/auth/profile", authHandler.UpdateProfile)
		protected.POST("/auth/avatar", authHandler.UploadAvatar)

		// Company
		protected.GET("/company", companyHandler.GetCompany)
		protected.PUT("/company", auth.AdminOnly(), companyHandler.UpdateCompany)
		protected.POST("/company/logo", auth.AdminOnly(), companyHandler.UploadLogo)
		protected.GET("/company/departments", companyHandler.ListDepartments)
		protected.POST("/company/departments", auth.AdminOnly(), companyHandler.CreateDepartment)
		protected.PUT("/company/departments/:id", auth.AdminOnly(), companyHandler.UpdateDepartment)
		protected.GET("/company/positions", companyHandler.ListPositions)
		protected.POST("/company/positions", auth.AdminOnly(), companyHandler.CreatePosition)
		protected.PUT("/company/positions/:id", auth.AdminOnly(), companyHandler.UpdatePosition)
		protected.GET("/company/cost-centers", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			centers, err := a.Queries.ListCostCenters(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list cost centers")
				return
			}
			response.OK(c, centers)
		})
		protected.POST("/company/cost-centers", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Code string `json:"code" binding:"required"`
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Code and name are required")
				return
			}
			companyID := auth.GetCompanyID(c)
			center, err := a.Queries.CreateCostCenter(c.Request.Context(), store.CreateCostCenterParams{
				CompanyID: companyID,
				Code:      req.Code,
				Name:      req.Name,
			})
			if err != nil {
				response.InternalError(c, "Failed to create cost center")
				return
			}
			response.Created(c, center)
		})

		// Employees
		protected.GET("/employees", auth.ManagerOrAbove(), employeeHandler.ListEmployees)
		protected.POST("/employees", auth.AdminOnly(), employeeHandler.CreateEmployee)
		protected.GET("/employees/:id", employeeHandler.GetEmployee)
		protected.PUT("/employees/:id", auth.AdminOnly(), employeeHandler.UpdateEmployee)
		protected.GET("/employees/:id/profile", employeeHandler.GetProfile)
		protected.PUT("/employees/:id/profile", auth.AdminOnly(), employeeHandler.UpdateProfile)
		protected.GET("/employees/:id/documents", employeeHandler.ListDocuments)
		protected.POST("/employees/:id/documents", auth.AdminOnly(), employeeHandler.UploadDocument)
		protected.GET("/employees/:id/documents/:doc_id/download", employeeHandler.DownloadDocument)
		protected.DELETE("/employees/:id/documents/:doc_id", auth.AdminOnly(), employeeHandler.DeleteDocument)
		protected.GET("/employees/documents/expiring", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			docs, err := a.Queries.ListExpiringDocuments(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list expiring documents")
				return
			}
			response.OK(c, docs)
		})
		protected.GET("/employees/:id/salary", auth.AdminOnly(), func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			companyID := auth.GetCompanyID(c)
			salary, err := a.Queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
				CompanyID:     companyID,
				EmployeeID:    id,
				EffectiveFrom: time.Now(),
			})
			if err != nil {
				response.OK(c, nil)
				return
			}
			response.OK(c, salary)
		})
		protected.POST("/employees/:id/salary", auth.AdminOnly(), func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			var req struct {
				BasicSalary   float64 `json:"basic_salary" binding:"required"`
				StructureID   *int64  `json:"structure_id"`
				EffectiveFrom string  `json:"effective_from" binding:"required"`
				EffectiveTo   *string `json:"effective_to"`
				Remarks       *string `json:"remarks"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)

			var basicSalary pgtype.Numeric
			_ = basicSalary.Scan(fmt.Sprintf("%.2f", req.BasicSalary))

			var effTo pgtype.Date
			if req.EffectiveTo != nil {
				parsed, _ := time.Parse("2006-01-02", *req.EffectiveTo)
				effTo = pgtype.Date{Time: parsed, Valid: true}
			}

			salary, err := a.Queries.CreateEmployeeSalary(c.Request.Context(), store.CreateEmployeeSalaryParams{
				CompanyID:     companyID,
				EmployeeID:    id,
				StructureID:   req.StructureID,
				BasicSalary:   basicSalary,
				EffectiveFrom: effFrom,
				EffectiveTo:   effTo,
				Remarks:       req.Remarks,
				CreatedBy:     &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to assign salary")
				return
			}
			response.Created(c, salary)
		})

		// Bulk Salary Update
		protected.POST("/employees/salary/bulk-update", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				EmployeeIDs   []int64 `json:"employee_ids" binding:"required"`
				UpdateType    string  `json:"update_type" binding:"required"` // "percentage" or "fixed"
				Value         float64 `json:"value" binding:"required"`
				EffectiveFrom string  `json:"effective_from" binding:"required"`
				Remarks       *string `json:"remarks"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			if req.UpdateType != "percentage" && req.UpdateType != "fixed" {
				response.BadRequest(c, "update_type must be 'percentage' or 'fixed'")
				return
			}

			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)

			var updated, failed int
			type Result struct {
				EmployeeID int64   `json:"employee_id"`
				OldSalary  float64 `json:"old_salary"`
				NewSalary  float64 `json:"new_salary"`
			}
			var results []Result

			for _, empID := range req.EmployeeIDs {
				currentSalary, err := a.Queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
					CompanyID:     companyID,
					EmployeeID:    empID,
					EffectiveFrom: time.Now(),
				})
				if err != nil {
					failed++
					continue
				}

				oldSalary := numericToFloat(currentSalary.BasicSalary)
				var newSalary float64
				if req.UpdateType == "percentage" {
					newSalary = oldSalary * (1 + req.Value/100)
				} else {
					newSalary = oldSalary + req.Value
				}

				var basicNum pgtype.Numeric
				_ = basicNum.Scan(fmt.Sprintf("%.2f", newSalary))

				_, err = a.Queries.CreateEmployeeSalary(c.Request.Context(), store.CreateEmployeeSalaryParams{
					CompanyID:     companyID,
					EmployeeID:    empID,
					StructureID:   currentSalary.StructureID,
					BasicSalary:   basicNum,
					EffectiveFrom: effFrom,
					Remarks:       req.Remarks,
					CreatedBy:     &userID,
				})
				if err != nil {
					failed++
					continue
				}
				updated++
				results = append(results, Result{EmployeeID: empID, OldSalary: oldSalary, NewSalary: newSalary})
			}

			response.OK(c, gin.H{
				"updated": updated,
				"failed":  failed,
				"results": results,
			})
		})

		// Employee Status Change
		protected.PUT("/employees/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			var req struct {
				Status  string  `json:"status" binding:"required"`
				Remarks *string `json:"remarks"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)

			// Get current employee to record old status
			oldEmp, err := a.Queries.GetEmployeeByID(c.Request.Context(), store.GetEmployeeByIDParams{
				ID: empID, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Employee not found")
				return
			}

			// Update status
			updated, err := a.Queries.UpdateEmployee(c.Request.Context(), store.UpdateEmployeeParams{
				ID:             empID,
				CompanyID:      companyID,
				FirstName:      oldEmp.FirstName,
				LastName:       oldEmp.LastName,
				MiddleName:     oldEmp.MiddleName,
				DisplayName:    oldEmp.DisplayName,
				Email:          oldEmp.Email,
				Phone:          oldEmp.Phone,
				DepartmentID:   oldEmp.DepartmentID,
				PositionID:     oldEmp.PositionID,
				CostCenterID:   oldEmp.CostCenterID,
				ManagerID:      oldEmp.ManagerID,
				EmploymentType: oldEmp.EmploymentType,
				Status:         req.Status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update status")
				return
			}

			// Create employment history record
			actionType := req.Status
			if req.Status == "active" && oldEmp.Status == "probationary" {
				actionType = "regularized"
			} else if req.Status == "separated" {
				actionType = "separated"
			} else if req.Status == "active" && oldEmp.Status == "separated" {
				actionType = "reinstated"
			} else if req.Status == "suspended" {
				actionType = "suspended"
			}

			_, _ = a.Queries.CreateEmploymentHistory(c.Request.Context(), store.CreateEmploymentHistoryParams{
				CompanyID:     companyID,
				EmployeeID:    empID,
				ActionType:    actionType,
				EffectiveDate: time.Now(),
				Remarks:       req.Remarks,
				CreatedBy:     &userID,
			})

			response.OK(c, updated)
		})

		// Employee Timeline
		protected.GET("/employees/:id/timeline", func(c *gin.Context) {
			empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			companyID := auth.GetCompanyID(c)
			items, err := a.Queries.ListEmployeeTimeline(c.Request.Context(), store.ListEmployeeTimelineParams{
				EmployeeID: empID,
				CompanyID:  companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to load timeline")
				return
			}
			response.OK(c, items)
		})

		// COE (Certificate of Employment)
		protected.GET("/employees/:id/coe", auth.ManagerOrAbove(), func(c *gin.Context) {
			empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			companyID := auth.GetCompanyID(c)

			emp, err := a.Queries.GetEmployeeForCOE(c.Request.Context(), store.GetEmployeeForCOEParams{
				ID:        empID,
				CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Employee not found")
				return
			}

			comp, err := a.Queries.GetCompanyByID(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get company")
				return
			}

			pdfBytes, err := generateCOEPDF(comp, emp)
			if err != nil {
				a.Logger.Error("failed to generate COE PDF", "error", err)
				response.InternalError(c, "Failed to generate PDF")
				return
			}

			fileName := fmt.Sprintf("COE_%s_%s.pdf", emp.EmployeeNo, time.Now().Format("20060102"))
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
			c.Data(200, "application/pdf", pdfBytes)
		})

		// Employee Letter Generation
		protected.POST("/employees/:id/letters", auth.ManagerOrAbove(), func(c *gin.Context) {
			empID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid employee ID")
				return
			}
			companyID := auth.GetCompanyID(c)

			var req struct {
				LetterType string `json:"letter_type" binding:"required"` // nte, coec, clearance, memo
				Subject    string `json:"subject"`
				Body       string `json:"body"`
				Violations string `json:"violations"` // for NTE
				Deadline   string `json:"deadline"`    // response deadline for NTE
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}

			emp, err := a.Queries.GetEmployeeForCOE(c.Request.Context(), store.GetEmployeeForCOEParams{
				ID:        empID,
				CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Employee not found")
				return
			}

			comp, err := a.Queries.GetCompanyByID(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get company")
				return
			}

			// Get salary for COEC
			var salaryAmount float64
			if req.LetterType == "coec" {
				sal, err := a.Queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
					CompanyID:     companyID,
					EmployeeID:    empID,
					EffectiveFrom: time.Now(),
				})
				if err == nil {
					var n pgtype.Numeric
					n = sal.BasicSalary
					if n.Valid {
						f, _ := n.Float64Value()
						if f.Valid {
							salaryAmount = f.Float64
						}
					}
				}
			}

			pdfBytes, err := generateLetterPDF(comp, emp, req.LetterType, req.Subject, req.Body, req.Violations, req.Deadline, salaryAmount)
			if err != nil {
				a.Logger.Error("failed to generate letter PDF", "error", err)
				response.InternalError(c, "Failed to generate PDF")
				return
			}

			fileName := fmt.Sprintf("%s_%s_%s.pdf", strings.ToUpper(req.LetterType), emp.EmployeeNo, time.Now().Format("20060102"))
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
			c.Data(200, "application/pdf", pdfBytes)
		})

		// Onboarding / Offboarding
		protected.GET("/onboarding/templates", auth.AdminOnly(), onboardingHandler.ListTemplates)
		protected.POST("/onboarding/templates", auth.AdminOnly(), onboardingHandler.CreateTemplate)
		protected.POST("/onboarding/initiate", auth.AdminOnly(), onboardingHandler.InitiateWorkflow)
		protected.GET("/onboarding/tasks/pending", auth.ManagerOrAbove(), onboardingHandler.ListPendingTasks)
		protected.GET("/onboarding/employees/:employee_id/tasks", auth.ManagerOrAbove(), onboardingHandler.ListTasks)
		protected.GET("/onboarding/employees/:employee_id/progress", onboardingHandler.GetProgress)
		protected.PUT("/onboarding/tasks/:id", auth.ManagerOrAbove(), onboardingHandler.UpdateTaskStatus)

		// Analytics
		protected.GET("/analytics/summary", auth.AdminOnly(), analyticsHandler.GetSummary)
		protected.GET("/analytics/headcount-trend", auth.AdminOnly(), analyticsHandler.GetHeadcountTrend)
		protected.GET("/analytics/turnover", auth.AdminOnly(), analyticsHandler.GetTurnoverStats)
		protected.GET("/analytics/department-costs", auth.AdminOnly(), analyticsHandler.GetDepartmentCosts)
		protected.GET("/analytics/attendance-patterns", auth.AdminOnly(), analyticsHandler.GetAttendancePatterns)
		protected.GET("/analytics/employment-types", auth.AdminOnly(), analyticsHandler.GetEmploymentTypeBreakdown)
		protected.GET("/analytics/leave-utilization", auth.AdminOnly(), analyticsHandler.GetLeaveUtilization)

		// Loans
		protected.GET("/loans/types", loanHandler.ListLoanTypes)
		protected.POST("/loans/types", auth.AdminOnly(), loanHandler.CreateLoanType)
		protected.GET("/loans", auth.AdminOnly(), loanHandler.ListLoans)
		protected.GET("/loans/my", loanHandler.ListMyLoans)
		protected.POST("/loans", loanHandler.ApplyLoan)
		protected.GET("/loans/:id", loanHandler.GetLoan)
		protected.POST("/loans/:id/approve", auth.AdminOnly(), loanHandler.ApproveLoan)
		protected.POST("/loans/:id/cancel", loanHandler.CancelLoan)
		protected.POST("/loans/:id/payments", auth.AdminOnly(), loanHandler.RecordPayment)

		// Notifications
		protected.GET("/notifications", notificationHandler.ListNotifications)
		protected.GET("/notifications/unread-count", notificationHandler.CountUnread)
		protected.POST("/notifications/:id/read", notificationHandler.MarkRead)
		protected.POST("/notifications/read-all", notificationHandler.MarkAllRead)
		protected.DELETE("/notifications/:id", notificationHandler.Delete)

		// Knowledge Base
		protected.GET("/knowledge/search", knowledgeHandler.Search)
		protected.GET("/knowledge", auth.AdminOnly(), knowledgeHandler.List)
		protected.GET("/knowledge/categories", knowledgeHandler.ListCategories)
		protected.GET("/knowledge/:id", knowledgeHandler.Get)
		protected.POST("/knowledge", auth.AdminOnly(), knowledgeHandler.Create)
		protected.PUT("/knowledge/:id", auth.AdminOnly(), knowledgeHandler.Update)
		protected.DELETE("/knowledge/:id", auth.AdminOnly(), knowledgeHandler.Delete)

		// Employee Directory & Org Chart (all authenticated users)
		protected.GET("/directory", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			search := c.Query("search")
			deptFilter := c.Query("department_id")

			var searchVal string
			if search != "" {
				searchVal = "%" + search + "%"
			}

			var deptIDVal int64
			if deptFilter != "" {
				if id, err := strconv.ParseInt(deptFilter, 10, 64); err == nil {
					deptIDVal = id
				}
			}

			employees, err := a.Queries.ListEmployeeDirectory(c.Request.Context(), store.ListEmployeeDirectoryParams{
				CompanyID: companyID,
				Column2:   searchVal,
				Column3:   deptIDVal,
			})
			if err != nil {
				response.InternalError(c, "Failed to list directory")
				return
			}
			response.OK(c, employees)
		})
		protected.GET("/directory/org-chart", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			data, err := a.Queries.GetOrgChartData(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get org chart data")
				return
			}
			response.OK(c, data)
		})

		// Audit Trail
		protected.GET("/audit/logs", auth.AdminOnly(), auditHandler.ListActivityLogs)

		// Employee Self-Service
		protected.GET("/self-service/info", selfServiceHandler.GetMyInfo)
		protected.GET("/self-service/team", selfServiceHandler.GetMyTeam)
		protected.GET("/self-service/compensation", selfServiceHandler.GetMyCompensation)
		protected.GET("/self-service/onboarding", selfServiceHandler.GetMyOnboarding)

		// Performance Management
		protected.GET("/performance/cycles", auth.ManagerOrAbove(), performanceHandler.ListCycles)
		protected.POST("/performance/cycles", auth.AdminOnly(), performanceHandler.CreateCycle)
		protected.POST("/performance/cycles/:id/initiate", auth.AdminOnly(), performanceHandler.InitiateReviews)
		protected.GET("/performance/cycles/:id/reviews", auth.ManagerOrAbove(), performanceHandler.ListReviewsByCycle)
		protected.GET("/performance/reviews/my", performanceHandler.ListMyReviews)
		protected.GET("/performance/reviews/:id", performanceHandler.GetReview)
		protected.PUT("/performance/reviews/:id/self", performanceHandler.SubmitSelfReview)
		protected.PUT("/performance/reviews/:id/manager", auth.ManagerOrAbove(), performanceHandler.SubmitManagerReview)
		protected.GET("/performance/goals", performanceHandler.ListGoals)
		protected.POST("/performance/goals", auth.ManagerOrAbove(), performanceHandler.CreateGoal)
		protected.PUT("/performance/goals/:id", performanceHandler.UpdateGoal)

		// Attendance
		protected.POST("/attendance/clock-in", attendanceHandler.ClockIn)
		protected.POST("/attendance/clock-out", attendanceHandler.ClockOut)
		protected.GET("/attendance/records", attendanceHandler.ListRecords)
		protected.GET("/attendance/summary", attendanceHandler.GetSummary)
		protected.GET("/attendance/shifts", attendanceHandler.ListShifts)
		protected.POST("/attendance/shifts", auth.AdminOnly(), attendanceHandler.CreateShift)
		protected.PUT("/attendance/shifts/:id", auth.AdminOnly(), attendanceHandler.UpdateShift)
		protected.POST("/attendance/schedules", auth.AdminOnly(), attendanceHandler.AssignSchedule)
		protected.GET("/attendance/schedules", auth.ManagerOrAbove(), attendanceHandler.ListAllSchedules)
		protected.POST("/attendance/schedules/bulk", auth.AdminOnly(), attendanceHandler.BulkAssignSchedule)
		protected.DELETE("/attendance/schedules/:schedule_id", auth.AdminOnly(), attendanceHandler.DeleteSchedule)

		// Geofencing
		protected.GET("/attendance/geofences", auth.AdminOnly(), attendanceHandler.ListGeofences)
		protected.POST("/attendance/geofences", auth.AdminOnly(), attendanceHandler.CreateGeofence)
		protected.PUT("/attendance/geofences/:id", auth.AdminOnly(), attendanceHandler.UpdateGeofence)
		protected.DELETE("/attendance/geofences/:id", auth.AdminOnly(), attendanceHandler.DeleteGeofence)
		protected.GET("/attendance/geofence-settings", attendanceHandler.GetGeofenceSettings)
		protected.PUT("/attendance/geofence-settings", auth.AdminOnly(), attendanceHandler.SetGeofenceSettings)

		// Schedule Templates
		protected.GET("/attendance/schedule-templates", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			templates, err := a.Queries.ListScheduleTemplates(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list schedule templates")
				return
			}
			response.OK(c, templates)
		})
		protected.POST("/attendance/schedule-templates", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				Description string `json:"description"`
				Days        []struct {
					DayOfWeek int   `json:"day_of_week"`
					ShiftID   int64 `json:"shift_id"`
					IsRestDay bool  `json:"is_rest_day"`
				} `json:"days"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)

			tmpl, err := a.Queries.CreateScheduleTemplate(c.Request.Context(), store.CreateScheduleTemplateParams{
				CompanyID:   companyID,
				Name:        req.Name,
				Description: &req.Description,
			})
			if err != nil {
				response.InternalError(c, "Failed to create schedule template")
				return
			}

			for _, d := range req.Days {
				var shiftID *int64
				if !d.IsRestDay && d.ShiftID > 0 {
					shiftID = &d.ShiftID
				}
				_, _ = a.Queries.UpsertScheduleTemplateDay(c.Request.Context(), store.UpsertScheduleTemplateDayParams{
					TemplateID: tmpl.ID,
					DayOfWeek:  int32(d.DayOfWeek),
					ShiftID:    shiftID,
					IsRestDay:  d.IsRestDay,
				})
			}

			response.Created(c, tmpl)
		})
		protected.GET("/attendance/schedule-templates/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)

			tmpl, err := a.Queries.GetScheduleTemplate(c.Request.Context(), store.GetScheduleTemplateParams{
				ID: id, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Template not found")
				return
			}
			days, _ := a.Queries.ListScheduleTemplateDays(c.Request.Context(), id)
			response.OK(c, gin.H{"template": tmpl, "days": days})
		})
		protected.PUT("/attendance/schedule-templates/:id", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)

			var req struct {
				Name        string `json:"name" binding:"required"`
				Description string `json:"description"`
				Days        []struct {
					DayOfWeek int   `json:"day_of_week"`
					ShiftID   int64 `json:"shift_id"`
					IsRestDay bool  `json:"is_rest_day"`
				} `json:"days"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}

			tmpl, err := a.Queries.UpdateScheduleTemplate(c.Request.Context(), store.UpdateScheduleTemplateParams{
				ID: id, CompanyID: companyID, Name: req.Name, Description: &req.Description,
			})
			if err != nil {
				response.NotFound(c, "Template not found")
				return
			}

			// Replace days
			_ = a.Queries.DeleteScheduleTemplateDays(c.Request.Context(), id)
			for _, d := range req.Days {
				var shiftID *int64
				if !d.IsRestDay && d.ShiftID > 0 {
					shiftID = &d.ShiftID
				}
				_, _ = a.Queries.UpsertScheduleTemplateDay(c.Request.Context(), store.UpsertScheduleTemplateDayParams{
					TemplateID: tmpl.ID,
					DayOfWeek:  int32(d.DayOfWeek),
					ShiftID:    shiftID,
					IsRestDay:  d.IsRestDay,
				})
			}

			response.OK(c, tmpl)
		})
		protected.DELETE("/attendance/schedule-templates/:id", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			_ = a.Queries.DeleteScheduleTemplate(c.Request.Context(), store.DeleteScheduleTemplateParams{
				ID: id, CompanyID: companyID,
			})
			response.OK(c, gin.H{"message": "Deleted"})
		})
		protected.POST("/attendance/schedule-templates/:id/assign", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)

			var req struct {
				EmployeeIDs   []int64 `json:"employee_ids" binding:"required"`
				EffectiveFrom string  `json:"effective_from" binding:"required"`
				EffectiveTo   *string `json:"effective_to"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}

			effFrom, _ := time.Parse("2006-01-02", req.EffectiveFrom)
			var effTo pgtype.Date
			if req.EffectiveTo != nil {
				parsed, _ := time.Parse("2006-01-02", *req.EffectiveTo)
				effTo = pgtype.Date{Time: parsed, Valid: true}
			}

			var assigned int
			for _, empID := range req.EmployeeIDs {
				_, err := a.Queries.AssignScheduleTemplate(c.Request.Context(), store.AssignScheduleTemplateParams{
					CompanyID:     companyID,
					EmployeeID:    empID,
					TemplateID:    id,
					EffectiveFrom: effFrom,
					EffectiveTo:   effTo,
				})
				if err == nil {
					assigned++
				}
			}
			response.OK(c, gin.H{"assigned": assigned})
		})
		protected.GET("/attendance/schedule-assignments", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			assignments, err := a.Queries.ListEmployeeScheduleAssignments(c.Request.Context(), store.ListEmployeeScheduleAssignmentsParams{
				CompanyID: companyID,
				Limit:     100,
				Offset:    0,
			})
			if err != nil {
				response.InternalError(c, "Failed to list assignments")
				return
			}
			response.OK(c, assignments)
		})

		// Attendance Corrections
		protected.POST("/attendance/corrections", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})
			if err != nil {
				response.BadRequest(c, "Employee not found")
				return
			}

			var req struct {
				AttendanceID     *int64  `json:"attendance_id"`
				CorrectionDate   string  `json:"correction_date" binding:"required"`
				RequestedClockIn *string `json:"requested_clock_in"`
				RequestedClockOut *string `json:"requested_clock_out"`
				Reason           string  `json:"reason" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}

			corrDate, err := time.Parse("2006-01-02", req.CorrectionDate)
			if err != nil {
				response.BadRequest(c, "Invalid correction date")
				return
			}

			params := store.CreateAttendanceCorrectionParams{
				CompanyID:      companyID,
				EmployeeID:     emp.ID,
				AttendanceID:   req.AttendanceID,
				CorrectionDate: corrDate,
				Reason:         req.Reason,
			}

			if req.RequestedClockIn != nil {
				t, err := time.Parse(time.RFC3339, *req.RequestedClockIn)
				if err == nil {
					params.RequestedClockIn = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			if req.RequestedClockOut != nil {
				t, err := time.Parse(time.RFC3339, *req.RequestedClockOut)
				if err == nil {
					params.RequestedClockOut = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}

			// If attendance_id provided, capture original times
			if req.AttendanceID != nil {
				origLog, err := a.Queries.GetAttendanceByID(c.Request.Context(), store.GetAttendanceByIDParams{
					ID:        *req.AttendanceID,
					CompanyID: companyID,
				})
				if err == nil {
					params.OriginalClockIn = origLog.ClockInAt
					params.OriginalClockOut = origLog.ClockOutAt
				}
			}

			correction, err := a.Queries.CreateAttendanceCorrection(c.Request.Context(), params)
			if err != nil {
				response.InternalError(c, "Failed to create correction request")
				return
			}
			response.Created(c, correction)
		})

		protected.GET("/attendance/corrections", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			corrections, err := a.Queries.ListAttendanceCorrections(c.Request.Context(), store.ListAttendanceCorrectionsParams{
				CompanyID: companyID,
				Limit:     100,
				Offset:    0,
			})
			if err != nil {
				response.InternalError(c, "Failed to list corrections")
				return
			}
			response.OK(c, corrections)
		})

		protected.GET("/attendance/corrections/pending", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			corrections, err := a.Queries.ListPendingCorrections(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list pending corrections")
				return
			}
			response.OK(c, corrections)
		})

		protected.GET("/attendance/corrections/my", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})
			if err != nil {
				response.BadRequest(c, "Employee not found")
				return
			}
			corrections, err := a.Queries.ListMyCorrections(c.Request.Context(), store.ListMyCorrectionsParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
				Limit:      50,
				Offset:     0,
			})
			if err != nil {
				response.InternalError(c, "Failed to list corrections")
				return
			}
			response.OK(c, corrections)
		})

		protected.POST("/attendance/corrections/:id/approve", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)

			var req struct {
				Note string `json:"note"`
			}
			_ = c.ShouldBindJSON(&req)

			correction, err := a.Queries.ApproveAttendanceCorrection(c.Request.Context(), store.ApproveAttendanceCorrectionParams{
				ID:         id,
				CompanyID:  companyID,
				ReviewedBy: &userID,
				ReviewNote: &req.Note,
			})
			if err != nil {
				response.InternalError(c, "Failed to approve correction")
				return
			}

			// Apply correction to attendance log if it has an attendance_id
			if correction.AttendanceID != nil {
				if correction.RequestedClockIn.Valid {
					_, _ = a.Pool.Exec(c.Request.Context(),
						"UPDATE attendance_logs SET clock_in_at = $1, is_corrected = true, corrected_by = $2 WHERE id = $3",
						correction.RequestedClockIn.Time, userID, *correction.AttendanceID)
				}
				if correction.RequestedClockOut.Valid {
					_, _ = a.Pool.Exec(c.Request.Context(),
						"UPDATE attendance_logs SET clock_out_at = $1, is_corrected = true, corrected_by = $2 WHERE id = $3",
						correction.RequestedClockOut.Time, userID, *correction.AttendanceID)
				}
			}

			response.OK(c, correction)
		})

		protected.POST("/attendance/corrections/:id/reject", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)

			var req struct {
				Note string `json:"note"`
			}
			_ = c.ShouldBindJSON(&req)

			correction, err := a.Queries.RejectAttendanceCorrection(c.Request.Context(), store.RejectAttendanceCorrectionParams{
				ID:         id,
				CompanyID:  companyID,
				ReviewedBy: &userID,
				ReviewNote: &req.Note,
			})
			if err != nil {
				response.InternalError(c, "Failed to reject correction")
				return
			}
			response.OK(c, correction)
		})

		// Leave
		protected.GET("/leaves/types", leaveHandler.ListTypes)
		protected.POST("/leaves/types", auth.AdminOnly(), leaveHandler.CreateType)
		protected.GET("/leaves/balances", leaveHandler.GetBalances)
		protected.POST("/leaves/requests", leaveHandler.CreateRequest)
		protected.GET("/leaves/requests", leaveHandler.ListRequests)
		protected.POST("/leaves/requests/:id/approve", auth.ManagerOrAbove(), leaveHandler.ApproveRequest)
		protected.POST("/leaves/requests/:id/reject", auth.ManagerOrAbove(), leaveHandler.RejectRequest)
		protected.POST("/leaves/requests/:id/cancel", leaveHandler.CancelRequest)

		// Leave Balance Management (admin)
		protected.GET("/leaves/balances/all", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
			year, _ := strconv.ParseInt(yearStr, 10, 32)
			balances, err := a.Queries.ListAllLeaveBalances(c.Request.Context(), store.ListAllLeaveBalancesParams{
				CompanyID: companyID,
				Year:      int32(year),
			})
			if err != nil {
				response.InternalError(c, "Failed to list leave balances")
				return
			}
			response.OK(c, balances)
		})
		protected.PUT("/leaves/balances/adjust", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				EmployeeID  int64   `json:"employee_id" binding:"required"`
				LeaveTypeID int64   `json:"leave_type_id" binding:"required"`
				Year        int32   `json:"year" binding:"required"`
				Adjusted    float64 `json:"adjusted"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			var adjusted pgtype.Numeric
			_ = adjusted.Scan(fmt.Sprintf("%.1f", req.Adjusted))
			balance, err := a.Queries.AdjustLeaveBalance(c.Request.Context(), store.AdjustLeaveBalanceParams{
				CompanyID:   companyID,
				EmployeeID:  req.EmployeeID,
				LeaveTypeID: req.LeaveTypeID,
				Year:        req.Year,
				Adjusted:    adjusted,
			})
			if err != nil {
				a.Logger.Error("failed to adjust leave balance", "error", err)
				response.InternalError(c, "Failed to adjust leave balance")
				return
			}
			response.OK(c, balance)
		})

		// Leave Carryover (year-end)
		protected.POST("/leaves/carryover", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				FromYear int32 `json:"from_year" binding:"required"`
				ToYear   int32 `json:"to_year" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "from_year and to_year are required")
				return
			}
			if req.ToYear != req.FromYear+1 {
				response.BadRequest(c, "to_year must be from_year + 1")
				return
			}

			// Get all balances from previous year with max_carryover info
			prevBalances, err := a.Queries.ListLeaveBalancesForCarryover(c.Request.Context(), store.ListLeaveBalancesForCarryoverParams{
				CompanyID: companyID,
				Year:      req.FromYear,
			})
			if err != nil {
				response.InternalError(c, "Failed to get previous year balances")
				return
			}

			type CarryoverResult struct {
				EmployeeNo    string  `json:"employee_no"`
				EmployeeName  string  `json:"employee_name"`
				LeaveType     string  `json:"leave_type"`
				Remaining     float64 `json:"remaining"`
				CarriedOver   float64 `json:"carried_over"`
				Forfeited     float64 `json:"forfeited"`
			}

			var carried int
			var totalForfeited float64
			results := []CarryoverResult{}
			for _, lb := range prevBalances {
				earned := numericToFloat(lb.Earned)
				used := numericToFloat(lb.Used)
				prevCarried := numericToFloat(lb.Carried)
				adjusted := numericToFloat(lb.Adjusted)
				remaining := earned + prevCarried + adjusted - used

				if remaining <= 0 {
					continue
				}

				maxCarry := numericToFloat(lb.MaxCarryover)
				if maxCarry <= 0 {
					maxCarry = 5
				}
				carryAmount := remaining
				if carryAmount > maxCarry {
					carryAmount = maxCarry
				}
				carryAmount = math.Round(carryAmount*10) / 10
				forfeited := math.Round((remaining-carryAmount)*10) / 10

				var carriedNum, earnedZero pgtype.Numeric
				_ = carriedNum.Scan(fmt.Sprintf("%.1f", carryAmount))
				_ = earnedZero.Scan("0")

				_, err := a.Queries.UpsertLeaveBalance(c.Request.Context(), store.UpsertLeaveBalanceParams{
					CompanyID:   companyID,
					EmployeeID:  lb.EmployeeID,
					LeaveTypeID: lb.LeaveTypeID,
					Year:        req.ToYear,
					Earned:      earnedZero,
					Carried:     carriedNum,
				})
				if err != nil {
					a.Logger.Error("failed to carryover leave balance",
						"employee_id", lb.EmployeeID,
						"leave_type_id", lb.LeaveTypeID,
						"error", err)
					continue
				}
				carried++
				totalForfeited += forfeited
				results = append(results, CarryoverResult{
					EmployeeNo:   lb.EmployeeNo,
					EmployeeName: lb.LastName + ", " + lb.FirstName,
					LeaveType:    lb.LeaveTypeName,
					Remaining:    remaining,
					CarriedOver:  carryAmount,
					Forfeited:    forfeited,
				})
			}

			response.OK(c, gin.H{
				"processed":       len(prevBalances),
				"carried":         carried,
				"total_forfeited": totalForfeited,
				"from_year":       req.FromYear,
				"to_year":         req.ToYear,
				"details":         results,
			})
		})

		// Leave Calendar
		protected.GET("/leaves/calendar", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			startStr := c.Query("start")
			endStr := c.Query("end")
			if startStr == "" || endStr == "" {
				response.BadRequest(c, "start and end dates are required")
				return
			}
			startDate, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				response.BadRequest(c, "Invalid start date format")
				return
			}
			endDate, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				response.BadRequest(c, "Invalid end date format")
				return
			}
			leaves, err := a.Queries.ListApprovedLeavesForCalendar(c.Request.Context(), store.ListApprovedLeavesForCalendarParams{
				CompanyID: companyID,
				EndDate:   startDate,
				StartDate: endDate,
			})
			if err != nil {
				response.InternalError(c, "Failed to list leave calendar")
				return
			}
			response.OK(c, leaves)
		})

		// Contract Milestones
		protected.GET("/milestones", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			milestoneType := c.Query("type")
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 {
				page = 1
			}
			offset := (page - 1) * limit

			milestones, err := a.Queries.ListContractMilestones(c.Request.Context(), store.ListContractMilestonesParams{
				CompanyID:     companyID,
				Status:        status,
				MilestoneType: milestoneType,
				Lim:           int32(limit),
				Off:           int32(offset),
			})
			if err != nil {
				response.InternalError(c, "Failed to list milestones")
				return
			}
			count, err := a.Queries.CountContractMilestones(c.Request.Context(), store.CountContractMilestonesParams{
				CompanyID:     companyID,
				Status:        status,
				MilestoneType: milestoneType,
			})
			if err != nil {
				response.InternalError(c, "Failed to count milestones")
				return
			}
			response.OK(c, gin.H{"data": milestones, "total": count, "page": page, "limit": limit})
		})

		protected.GET("/milestones/pending", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			milestones, err := a.Queries.ListPendingMilestonesByCompany(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list pending milestones")
				return
			}
			response.OK(c, milestones)
		})

		protected.POST("/milestones/:id/acknowledge", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Notes string `json:"notes"`
			}
			_ = c.ShouldBindJSON(&req)
			milestone, err := a.Queries.AcknowledgeMilestone(c.Request.Context(), store.AcknowledgeMilestoneParams{
				ID:             id,
				CompanyID:      companyID,
				AcknowledgedBy: &userID,
				Notes:          &req.Notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to acknowledge milestone")
				return
			}
			response.OK(c, milestone)
		})

		protected.POST("/milestones/:id/action", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Notes string `json:"notes"`
			}
			_ = c.ShouldBindJSON(&req)
			milestone, err := a.Queries.ActionMilestone(c.Request.Context(), store.ActionMilestoneParams{
				ID:        id,
				CompanyID: companyID,
				Notes:     &req.Notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to action milestone")
				return
			}
			response.OK(c, milestone)
		})

		// Leave Encashment
		protected.GET("/leaves/encashment/convertible", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			year := int32(time.Now().Year())
			if y := c.Query("year"); y != "" {
				if v, err := strconv.Atoi(y); err == nil {
					year = int32(v)
				}
			}
			balances, err := a.Queries.GetConvertibleLeaveBalances(c.Request.Context(), store.GetConvertibleLeaveBalancesParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
				Year:       year,
			})
			if err != nil {
				response.InternalError(c, "Failed to get convertible balances")
				return
			}
			response.OK(c, balances)
		})

		protected.POST("/leaves/encashment", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			empID := emp.ID

			var req struct {
				LeaveTypeID int64   `json:"leave_type_id" binding:"required"`
				Year        int32   `json:"year" binding:"required"`
				Days        float64 `json:"days" binding:"required"`
				Remarks     *string `json:"remarks"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}

			// Get employee salary for daily rate calculation
			salary, err := a.Queries.GetCurrentSalary(c.Request.Context(), store.GetCurrentSalaryParams{
				CompanyID:     companyID,
				EmployeeID:    empID,
				EffectiveFrom: time.Now(),
			})
			if err != nil {
				response.BadRequest(c, "No active salary found")
				return
			}
			monthlyF := numericToFloat(salary.BasicSalary)
			// Daily rate = monthly / 26 (working days per PH standard)
			dailyRate := monthlyF / 26.0
			totalAmount := dailyRate * req.Days

			var days, dr, ta pgtype.Numeric
			_ = days.Scan(fmt.Sprintf("%.1f", req.Days))
			_ = dr.Scan(fmt.Sprintf("%.2f", dailyRate))
			_ = ta.Scan(fmt.Sprintf("%.2f", totalAmount))

			enc, err := a.Queries.CreateLeaveEncashment(c.Request.Context(), store.CreateLeaveEncashmentParams{
				CompanyID:   companyID,
				EmployeeID:  empID,
				LeaveTypeID: req.LeaveTypeID,
				Year:        req.Year,
				Days:        days,
				DailyRate:   dr,
				TotalAmount: ta,
				Remarks:     req.Remarks,
			})
			if err != nil {
				a.Logger.Error("failed to create leave encashment", "error", err)
				response.InternalError(c, "Failed to create encashment request")
				return
			}
			response.OK(c, enc)
		})

		protected.GET("/leaves/encashment", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			var empID int64
			if e := c.Query("employee_id"); e != "" {
				empID, _ = strconv.ParseInt(e, 10, 64)
			}
			limit, offset := 50, 0
			if l := c.Query("limit"); l != "" {
				if v, err := strconv.Atoi(l); err == nil {
					limit = v
				}
			}
			if o := c.Query("offset"); o != "" {
				if v, err := strconv.Atoi(o); err == nil {
					offset = v
				}
			}
			items, err := a.Queries.ListLeaveEncashments(c.Request.Context(), store.ListLeaveEncashmentsParams{
				CompanyID: companyID,
				Column2:   status,
				Column3:   empID,
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
			if err != nil {
				response.InternalError(c, "Failed to list encashments")
				return
			}
			count, _ := a.Queries.CountLeaveEncashments(c.Request.Context(), store.CountLeaveEncashmentsParams{
				CompanyID: companyID,
				Column2:   status,
				Column3:   empID,
			})
			response.Paginated(c, items, count, offset/limit+1, limit)
		})

		protected.POST("/leaves/encashment/:id/approve", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			enc, err := a.Queries.ApproveLeaveEncashment(c.Request.Context(), store.ApproveLeaveEncashmentParams{
				ID:         id,
				CompanyID:  companyID,
				ApprovedBy: &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to approve encashment")
				return
			}
			response.OK(c, enc)
		})

		protected.POST("/leaves/encashment/:id/reject", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			var req struct {
				Remarks *string `json:"remarks"`
			}
			_ = c.ShouldBindJSON(&req)
			enc, err := a.Queries.RejectLeaveEncashment(c.Request.Context(), store.RejectLeaveEncashmentParams{
				ID:         id,
				CompanyID:  companyID,
				ApprovedBy: &userID,
				Remarks:    req.Remarks,
			})
			if err != nil {
				response.InternalError(c, "Failed to reject encashment")
				return
			}
			response.OK(c, enc)
		})

		protected.POST("/leaves/encashment/:id/paid", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			enc, err := a.Queries.MarkLeaveEncashmentPaid(c.Request.Context(), store.MarkLeaveEncashmentPaidParams{
				ID:        id,
				CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to mark encashment as paid")
				return
			}
			response.OK(c, enc)
		})

		// Overtime
		protected.POST("/overtime/requests", overtimeHandler.CreateRequest)
		protected.GET("/overtime/requests", overtimeHandler.ListRequests)
		protected.POST("/overtime/requests/:id/approve", auth.ManagerOrAbove(), overtimeHandler.ApproveRequest)
		protected.POST("/overtime/requests/:id/reject", auth.ManagerOrAbove(), overtimeHandler.RejectRequest)

		// Payroll
		protected.GET("/payroll/cycles", auth.AdminOnly(), payrollHandler.ListCycles)
		protected.POST("/payroll/cycles", auth.AdminOnly(), payrollHandler.CreateCycle)
		protected.POST("/payroll/runs", auth.AdminOnly(), payrollHandler.RunPayroll)
		protected.GET("/payroll/runs/:id/items", auth.AdminOnly(), payrollHandler.ListPayrollItems)
		protected.GET("/payroll/cycles/:id/items", auth.AdminOnly(), func(c *gin.Context) {
			cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid cycle ID")
				return
			}
			// Find latest run for this cycle
			var runID int64
			row := a.Pool.QueryRow(c.Request.Context(), "SELECT id FROM payroll_runs WHERE cycle_id = $1 ORDER BY created_at DESC LIMIT 1", cycleID)
			if err := row.Scan(&runID); err != nil {
				response.OK(c, []any{})
				return
			}
			items, err := a.Queries.ListPayrollItems(c.Request.Context(), runID)
			if err != nil {
				response.InternalError(c, "Failed to list payroll items")
				return
			}
			response.OK(c, items)
		})
		protected.POST("/payroll/cycles/:id/approve", auth.AdminOnly(), payrollHandler.ApproveCycle)

		protected.POST("/payroll/cycles/:id/lock", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			if err := a.Queries.LockPayrollCycle(c.Request.Context(), store.LockPayrollCycleParams{
				ID:        id,
				CompanyID: companyID,
				LockedBy:  &userID,
			}); err != nil {
				response.InternalError(c, "Failed to lock cycle")
				return
			}
			response.OK(c, map[string]bool{"locked": true})
		})

		protected.POST("/payroll/cycles/:id/unlock", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			if err := a.Queries.UnlockPayrollCycle(c.Request.Context(), store.UnlockPayrollCycleParams{
				ID:        id,
				CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to unlock cycle")
				return
			}
			response.OK(c, map[string]bool{"locked": false})
		})

		protected.GET("/payroll/runs/:id/anomalies", auth.AdminOnly(), func(c *gin.Context) {
			runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid run ID")
				return
			}
			companyID := auth.GetCompanyID(c)
			calculator := payroll.NewCalculator(a.Queries, a.Pool, a.Logger)
			report, err := calculator.DetectAnomalies(c.Request.Context(), runID, companyID)
			if err != nil {
				response.InternalError(c, fmt.Sprintf("Anomaly detection failed: %s", err.Error()))
				return
			}
			response.OK(c, report)
		})
		protected.GET("/payroll/cycles/:id/anomalies", auth.AdminOnly(), func(c *gin.Context) {
			cycleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid cycle ID")
				return
			}
			companyID := auth.GetCompanyID(c)
			runID, err := a.Queries.GetLatestCompletedRunForCycle(c.Request.Context(), store.GetLatestCompletedRunForCycleParams{
				CycleID:   cycleID,
				CompanyID: companyID,
			})
			if err != nil {
				response.OK(c, map[string]any{"anomalies": []any{}, "total_items": 0})
				return
			}
			calculator := payroll.NewCalculator(a.Queries, a.Pool, a.Logger)
			report, err := calculator.DetectAnomalies(c.Request.Context(), runID, companyID)
			if err != nil {
				response.InternalError(c, fmt.Sprintf("Anomaly detection failed: %s", err.Error()))
				return
			}
			response.OK(c, report)
		})
		protected.GET("/payroll/payslips", payrollHandler.ListPayslips)
		protected.GET("/payroll/payslips/:id", payrollHandler.GetPayslip)
		protected.GET("/payroll/payslips/:id/pdf", payrollHandler.DownloadPayslipPDF)

		// Compliance & Tax Tables
		protected.GET("/compliance/sss-table", auth.AdminOnly(), complianceHandler.ListSSSTable)
		protected.GET("/compliance/philhealth-table", auth.AdminOnly(), complianceHandler.ListPhilHealthTable)
		protected.GET("/compliance/pagibig-table", auth.AdminOnly(), complianceHandler.ListPagIBIGTable)
		protected.GET("/compliance/bir-tax-table", auth.AdminOnly(), complianceHandler.ListBIRTaxTable)
		protected.GET("/compliance/government-forms", auth.AdminOnly(), complianceHandler.ListGovernmentForms)
		protected.POST("/compliance/government-forms", auth.AdminOnly(), complianceHandler.CreateGovernmentForm)
		protected.POST("/compliance/government-forms/generate", auth.AdminOnly(), complianceHandler.GenerateFormHandler)

		// Tax Filing & Remittance
		protected.GET("/tax-filings", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
			status := c.Query("status")
			filingType := c.Query("type")

			filings, err := a.Queries.ListTaxFilings(c.Request.Context(), store.ListTaxFilingsParams{
				CompanyID:  companyID,
				Status:     status,
				FilingType: filingType,
				PeriodYear: int32(year),
			})
			if err != nil {
				response.InternalError(c, "Failed to list tax filings")
				return
			}
			summary, _ := a.Queries.GetFilingSummary(c.Request.Context(), store.GetFilingSummaryParams{
				CompanyID:  companyID,
				PeriodYear: int32(year),
			})
			response.OK(c, gin.H{"data": filings, "summary": summary, "year": year})
		})

		protected.POST("/tax-filings", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				FilingType    string  `json:"filing_type" binding:"required"`
				PeriodType    string  `json:"period_type" binding:"required"`
				PeriodYear    int32   `json:"period_year" binding:"required"`
				PeriodMonth   *int32  `json:"period_month"`
				PeriodQuarter *int32  `json:"period_quarter"`
				DueDate       string  `json:"due_date" binding:"required"`
				Amount        float64 `json:"amount"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			dueDate, err := time.Parse("2006-01-02", req.DueDate)
			if err != nil {
				response.BadRequest(c, "Invalid due date")
				return
			}
			var amountNum pgtype.Numeric
			_ = amountNum.Scan(fmt.Sprintf("%.2f", req.Amount))
			filing, err := a.Queries.CreateTaxFiling(c.Request.Context(), store.CreateTaxFilingParams{
				CompanyID:     companyID,
				FilingType:    req.FilingType,
				PeriodType:    req.PeriodType,
				PeriodYear:    req.PeriodYear,
				PeriodMonth:   req.PeriodMonth,
				PeriodQuarter: req.PeriodQuarter,
				DueDate:       dueDate,
				Amount:        amountNum,
				Status:        "pending",
			})
			if err != nil {
				response.InternalError(c, "Failed to create tax filing")
				return
			}
			response.Created(c, filing)
		})

		protected.PUT("/tax-filings/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Status      string `json:"status" binding:"required"`
				ReferenceNo string `json:"reference_no"`
				Notes       string `json:"notes"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			filing, err := a.Queries.UpdateTaxFilingStatus(c.Request.Context(), store.UpdateTaxFilingStatusParams{
				ID:          id,
				CompanyID:   companyID,
				Status:      req.Status,
				FiledBy:     &userID,
				ReferenceNo: &req.ReferenceNo,
				Notes:       &req.Notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to update filing")
				return
			}
			response.OK(c, filing)
		})

		protected.GET("/tax-filings/overdue", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			filings, err := a.Queries.ListOverdueFilings(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list overdue filings")
				return
			}
			response.OK(c, filings)
		})

		protected.GET("/tax-filings/upcoming", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			filings, err := a.Queries.ListUpcomingFilings(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list upcoming filings")
				return
			}
			response.OK(c, filings)
		})

		protected.POST("/tax-filings/generate-annual", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				Year int32 `json:"year" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Year is required")
				return
			}
			err := a.Queries.GenerateAnnualFilings(c.Request.Context(), store.GenerateAnnualFilingsParams{
				CompanyID:  companyID,
				PeriodYear: req.Year,
			})
			if err != nil {
				response.InternalError(c, "Failed to generate annual filings")
				return
			}
			response.OK(c, gin.H{"message": "Annual filings generated", "year": req.Year})
		})

		protected.GET("/tax-filings/remittances", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
			records, err := a.Queries.ListRemittanceRecords(c.Request.Context(), store.ListRemittanceRecordsParams{
				CompanyID:  companyID,
				PeriodYear: int32(year),
			})
			if err != nil {
				response.InternalError(c, "Failed to list remittance records")
				return
			}
			response.OK(c, records)
		})

		// Disciplinary Management
		protected.POST("/disciplinary/incidents", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			var req struct {
				EmployeeID    int64  `json:"employee_id" binding:"required"`
				IncidentDate  string `json:"incident_date" binding:"required"`
				Category      string `json:"category" binding:"required"`
				Severity      string `json:"severity" binding:"required"`
				Description   string `json:"description" binding:"required"`
				Witnesses     string `json:"witnesses"`
				EvidenceNotes string `json:"evidence_notes"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			incDate, err := time.Parse("2006-01-02", req.IncidentDate)
			if err != nil {
				response.BadRequest(c, "Invalid date format")
				return
			}
			incident, err := a.Queries.CreateDisciplinaryIncident(c.Request.Context(), store.CreateDisciplinaryIncidentParams{
				CompanyID:     companyID,
				EmployeeID:    req.EmployeeID,
				ReportedBy:    &userID,
				IncidentDate:  incDate,
				Category:      req.Category,
				Severity:      req.Severity,
				Description:   req.Description,
				Witnesses:     &req.Witnesses,
				EvidenceNotes: &req.EvidenceNotes,
			})
			if err != nil {
				response.InternalError(c, "Failed to create incident")
				return
			}
			response.Created(c, incident)
		})

		protected.GET("/disciplinary/incidents", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			empIDStr := c.Query("employee_id")
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 {
				page = 1
			}

			var empID int64
			if empIDStr != "" {
				empID, _ = strconv.ParseInt(empIDStr, 10, 64)
			}

			incidents, err := a.Queries.ListDisciplinaryIncidents(c.Request.Context(), store.ListDisciplinaryIncidentsParams{
				CompanyID:  companyID,
				Status:     status,
				EmployeeID: empID,
				Lim:        int32(limit),
				Off:        int32((page - 1) * limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list incidents")
				return
			}
			count, err := a.Queries.CountDisciplinaryIncidents(c.Request.Context(), store.CountDisciplinaryIncidentsParams{
				CompanyID:  companyID,
				Status:     status,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to count incidents")
				return
			}
			response.OK(c, gin.H{"data": incidents, "total": count, "page": page, "limit": limit})
		})

		protected.GET("/disciplinary/incidents/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			incident, err := a.Queries.GetDisciplinaryIncident(c.Request.Context(), store.GetDisciplinaryIncidentParams{
				ID:        id,
				CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Incident not found")
				return
			}
			actions, err := a.Queries.ListActionsByIncident(c.Request.Context(), store.ListActionsByIncidentParams{
				IncidentID: &id,
				CompanyID:  companyID,
			})
			if err != nil {
				actions = []store.ListActionsByIncidentRow{}
			}
			response.OK(c, gin.H{"incident": incident, "actions": actions})
		})

		protected.PUT("/disciplinary/incidents/:id/status", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Status          string `json:"status" binding:"required"`
				ResolutionNotes string `json:"resolution_notes"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			incident, err := a.Queries.UpdateIncidentStatus(c.Request.Context(), store.UpdateIncidentStatusParams{
				ID:              id,
				CompanyID:       companyID,
				Status:          req.Status,
				ResolutionNotes: &req.ResolutionNotes,
				ResolvedBy:      &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to update incident")
				return
			}
			response.OK(c, incident)
		})

		protected.POST("/disciplinary/actions", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			var req struct {
				EmployeeID     int64  `json:"employee_id" binding:"required"`
				IncidentID     *int64 `json:"incident_id"`
				ActionType     string `json:"action_type" binding:"required"`
				ActionDate     string `json:"action_date" binding:"required"`
				Description    string `json:"description" binding:"required"`
				SuspensionDays *int32 `json:"suspension_days"`
				EffectiveDate  string `json:"effective_date"`
				EndDate        string `json:"end_date"`
				Notes          string `json:"notes"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			actDate, err := time.Parse("2006-01-02", req.ActionDate)
			if err != nil {
				response.BadRequest(c, "Invalid action date")
				return
			}
			var effDate, endDatePg pgtype.Date
			if req.EffectiveDate != "" {
				t, _ := time.Parse("2006-01-02", req.EffectiveDate)
				effDate = pgtype.Date{Time: t, Valid: true}
			}
			if req.EndDate != "" {
				t, _ := time.Parse("2006-01-02", req.EndDate)
				endDatePg = pgtype.Date{Time: t, Valid: true}
			}

			action, err := a.Queries.CreateDisciplinaryAction(c.Request.Context(), store.CreateDisciplinaryActionParams{
				CompanyID:      companyID,
				EmployeeID:     req.EmployeeID,
				IncidentID:     req.IncidentID,
				ActionType:     req.ActionType,
				ActionDate:     actDate,
				IssuedBy:       userID,
				Description:    req.Description,
				SuspensionDays: req.SuspensionDays,
				EffectiveDate:  effDate,
				EndDate:        endDatePg,
				Notes:          &req.Notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to create disciplinary action")
				return
			}
			response.Created(c, action)
		})

		protected.GET("/disciplinary/actions", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			empIDStr := c.Query("employee_id")
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 {
				page = 1
			}

			var empID int64
			if empIDStr != "" {
				empID, _ = strconv.ParseInt(empIDStr, 10, 64)
			}

			actions, err := a.Queries.ListDisciplinaryActions(c.Request.Context(), store.ListDisciplinaryActionsParams{
				CompanyID:  companyID,
				EmployeeID: empID,
				Lim:        int32(limit),
				Off:        int32((page - 1) * limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list actions")
				return
			}
			count, err := a.Queries.CountDisciplinaryActions(c.Request.Context(), store.CountDisciplinaryActionsParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to count actions")
				return
			}
			response.OK(c, gin.H{"data": actions, "total": count, "page": page, "limit": limit})
		})

		protected.POST("/disciplinary/actions/:id/acknowledge", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			action, err := a.Queries.AcknowledgeDisciplinaryAction(c.Request.Context(), store.AcknowledgeDisciplinaryActionParams{
				ID:        id,
				CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to acknowledge action")
				return
			}
			response.OK(c, action)
		})

		protected.POST("/disciplinary/actions/:id/appeal", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Reason string `json:"reason" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Appeal reason is required")
				return
			}
			action, err := a.Queries.AppealDisciplinaryAction(c.Request.Context(), store.AppealDisciplinaryActionParams{
				ID:           id,
				CompanyID:    companyID,
				AppealReason: &req.Reason,
			})
			if err != nil {
				response.InternalError(c, "Failed to submit appeal")
				return
			}
			response.OK(c, action)
		})

		protected.POST("/disciplinary/actions/:id/resolve-appeal", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Status     string `json:"status" binding:"required"` // appeal_denied, appeal_granted
				Resolution string `json:"resolution"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			action, err := a.Queries.ResolveAppeal(c.Request.Context(), store.ResolveAppealParams{
				ID:               id,
				CompanyID:        companyID,
				AppealStatus:     &req.Status,
				AppealResolution: &req.Resolution,
				AppealResolvedBy: &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to resolve appeal")
				return
			}
			response.OK(c, action)
		})

		protected.GET("/disciplinary/employee/:id/summary", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			incSummary, err := a.Queries.GetEmployeeDisciplinarySummary(c.Request.Context(), store.GetEmployeeDisciplinarySummaryParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to get summary")
				return
			}
			actCounts, err := a.Queries.GetEmployeeActionCounts(c.Request.Context(), store.GetEmployeeActionCountsParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to get action counts")
				return
			}
			response.OK(c, gin.H{"incidents": incSummary, "actions": actCounts})
		})

		// DTR (Daily Time Record) Report
		protected.GET("/reports/dtr", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			startStr := c.Query("start")
			endStr := c.Query("end")
			employeeIDStr := c.Query("employee_id")

			if startStr == "" || endStr == "" {
				response.BadRequest(c, "start and end dates are required")
				return
			}
			startDate, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				response.BadRequest(c, "Invalid start date")
				return
			}
			endDate, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				response.BadRequest(c, "Invalid end date")
				return
			}
			endDate = endDate.AddDate(0, 0, 1) // inclusive end
			startTz := pgtype.Timestamptz{Time: startDate, Valid: true}
			endTz := pgtype.Timestamptz{Time: endDate, Valid: true}

			if employeeIDStr != "" {
				empID, _ := strconv.ParseInt(employeeIDStr, 10, 64)
				records, err := a.Queries.GetDTR(c.Request.Context(), store.GetDTRParams{
					CompanyID:   companyID,
					EmployeeID:  empID,
					ClockInAt:   startTz,
					ClockInAt_2: endTz,
				})
				if err != nil {
					response.InternalError(c, "Failed to get DTR")
					return
				}
				response.OK(c, records)
			} else {
				records, err := a.Queries.GetDTRAllEmployees(c.Request.Context(), store.GetDTRAllEmployeesParams{
					CompanyID:   companyID,
					ClockInAt:   startTz,
					ClockInAt_2: endTz,
				})
				if err != nil {
					response.InternalError(c, "Failed to get DTR")
					return
				}
				response.OK(c, records)
			}
		})

		protected.GET("/reports/dtr/csv", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			startStr := c.Query("start")
			endStr := c.Query("end")
			if startStr == "" || endStr == "" {
				response.BadRequest(c, "start and end dates are required")
				return
			}
			startDate, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				response.BadRequest(c, "Invalid start date")
				return
			}
			endDate, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				response.BadRequest(c, "Invalid end date")
				return
			}
			endDate = endDate.AddDate(0, 0, 1)
			startTz := pgtype.Timestamptz{Time: startDate, Valid: true}
			endTz := pgtype.Timestamptz{Time: endDate, Valid: true}

			records, err := a.Queries.GetDTRAllEmployees(c.Request.Context(), store.GetDTRAllEmployeesParams{
				CompanyID:   companyID,
				ClockInAt:   startTz,
				ClockInAt_2: endTz,
			})
			if err != nil {
				response.InternalError(c, "Failed to get DTR")
				return
			}

			c.Header("Content-Type", "text/csv")
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=dtr_%s_%s.csv", startStr, c.Query("end")))

			var buf bytes.Buffer
			buf.WriteString("Employee No,Name,Department,Position,Date,Clock In,Clock Out,Work Hours,OT Hours,Late (min),Undertime (min),Status\n")
			for _, r := range records {
				clockIn := ""
				date := ""
				if r.ClockInAt.Valid {
					clockIn = r.ClockInAt.Time.Format("15:04")
					date = r.ClockInAt.Time.Format("2006-01-02")
				}
				clockOut := ""
				if r.ClockOutAt.Valid {
					clockOut = r.ClockOutAt.Time.Format("15:04")
				}
				wh, _ := r.WorkHours.Float64Value()
				oh, _ := r.OvertimeHours.Float64Value()
				var late, ut int32
				if r.LateMinutes != nil {
					late = *r.LateMinutes
				}
				if r.UndertimeMinutes != nil {
					ut = *r.UndertimeMinutes
				}
				buf.WriteString(fmt.Sprintf("%s,\"%s %s\",%s,%s,%s,%s,%s,%.2f,%.2f,%d,%d,%s\n",
					r.EmployeeNo, r.FirstName, r.LastName,
					r.DepartmentName, r.PositionName,
					date, clockIn, clockOut,
					wh.Float64, oh.Float64,
					late, ut,
					r.Status,
				))
			}
			c.Data(http.StatusOK, "text/csv", buf.Bytes())
		})

		// DOLE Employee Register Report
		protected.GET("/reports/dole-register", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)

			comp, err := a.Queries.GetCompanyByID(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get company")
				return
			}

			emps, err := a.Queries.ListEmployeesForDOLERegister(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list employees")
				return
			}

			pdfBytes, err := generateDOLERegisterPDF(comp, emps)
			if err != nil {
				a.Logger.Error("failed to generate DOLE register PDF", "error", err)
				response.InternalError(c, "Failed to generate PDF")
				return
			}

			fileName := fmt.Sprintf("DOLE_Register_%s.pdf", time.Now().Format("20060102"))
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
			c.Data(200, "application/pdf", pdfBytes)
		})

		// Attendance Report
		protected.GET("/attendance/report", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			startStr := c.Query("start")
			endStr := c.Query("end")
			if startStr == "" || endStr == "" {
				response.BadRequest(c, "start and end dates are required")
				return
			}
			startDate, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				response.BadRequest(c, "Invalid start date format")
				return
			}
			endDate, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				response.BadRequest(c, "Invalid end date format")
				return
			}
			endDate = endDate.AddDate(0, 0, 1) // exclusive end
			report, err := a.Queries.GetAttendanceReport(c.Request.Context(), store.GetAttendanceReportParams{
				CompanyID:   companyID,
				ClockInAt:   pgtype.Timestamptz{Time: startDate, Valid: true},
				ClockInAt_2: pgtype.Timestamptz{Time: endDate, Valid: true},
			})
			if err != nil {
				a.Logger.Error("failed to get attendance report", "error", err)
				response.InternalError(c, "Failed to generate report")
				return
			}
			response.OK(c, report)
		})

		// Final Pay
		protected.GET("/final-pay", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 50 }
			items, err := a.Queries.ListFinalPays(c.Request.Context(), store.ListFinalPaysParams{
				CompanyID: companyID,
				Limit:     int32(limit),
				Offset:    int32((page - 1) * limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list final pays")
				return
			}
			response.OK(c, items)
		})
		protected.GET("/final-pay/:employee_id", auth.AdminOnly(), func(c *gin.Context) {
			empID, _ := strconv.ParseInt(c.Param("employee_id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			fp, err := a.Queries.GetFinalPay(c.Request.Context(), store.GetFinalPayParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.NotFound(c, "No final pay record found")
				return
			}
			response.OK(c, fp)
		})
		protected.POST("/final-pay", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				EmployeeID            int64   `json:"employee_id" binding:"required"`
				SeparationDate        string  `json:"separation_date" binding:"required"`
				SeparationReason      string  `json:"separation_reason" binding:"required"`
				UnpaidSalary          float64 `json:"unpaid_salary"`
				Prorated13th          float64 `json:"prorated_13th"`
				UnusedLeaveConversion float64 `json:"unused_leave_conversion"`
				SeparationPay         float64 `json:"separation_pay"`
				TaxRefund             float64 `json:"tax_refund"`
				OtherDeductions       float64 `json:"other_deductions"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			sepDate, _ := time.Parse("2006-01-02", req.SeparationDate)

			toNum := func(v float64) pgtype.Numeric {
				var n pgtype.Numeric
				_ = n.Scan(fmt.Sprintf("%.2f", v))
				return n
			}

			total := req.UnpaidSalary + req.Prorated13th + req.UnusedLeaveConversion +
				req.SeparationPay + req.TaxRefund - req.OtherDeductions

			fp, err := a.Queries.CreateFinalPay(c.Request.Context(), store.CreateFinalPayParams{
				CompanyID:             companyID,
				EmployeeID:            req.EmployeeID,
				SeparationDate:        sepDate,
				SeparationReason:      req.SeparationReason,
				UnpaidSalary:          toNum(req.UnpaidSalary),
				Prorated13th:          toNum(req.Prorated13th),
				UnusedLeaveConversion: toNum(req.UnusedLeaveConversion),
				SeparationPay:         toNum(req.SeparationPay),
				TaxRefund:             toNum(req.TaxRefund),
				OtherDeductions:       toNum(req.OtherDeductions),
				TotalFinalPay:         toNum(total),
				Payload:               []byte("{}"),
			})
			if err != nil {
				a.Logger.Error("failed to create final pay", "error", err)
				response.InternalError(c, "Failed to create final pay")
				return
			}
			response.Created(c, fp)
		})
		protected.PUT("/final-pay/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct { Status string `json:"status" binding:"required"` }
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			fp, err := a.Queries.UpdateFinalPayStatus(c.Request.Context(), store.UpdateFinalPayStatusParams{
				ID: id, CompanyID: companyID, Status: req.Status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update final pay status")
				return
			}
			response.OK(c, fp)
		})

		// Benefits Administration
		protected.GET("/benefits/plans", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			plans, err := a.Queries.ListBenefitPlans(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list benefit plans")
				return
			}
			response.OK(c, plans)
		})

		protected.GET("/benefits/plans/:id", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			planID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			plan, err := a.Queries.GetBenefitPlan(c.Request.Context(), store.GetBenefitPlanParams{
				ID: planID, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Plan not found")
				return
			}
			response.OK(c, plan)
		})

		protected.POST("/benefits/plans", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				Name              string  `json:"name" binding:"required"`
				Category          string  `json:"category" binding:"required"`
				Description       string  `json:"description"`
				Provider          string  `json:"provider"`
				EmployerShare     float64 `json:"employer_share"`
				EmployeeShare     float64 `json:"employee_share"`
				CoverageAmount    float64 `json:"coverage_amount"`
				EligibilityType   string  `json:"eligibility_type"`
				EligibilityMonths int32   `json:"eligibility_months"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			var erShare, eeShare, covAmt pgtype.Numeric
			_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
			_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
			_ = covAmt.Scan(fmt.Sprintf("%.2f", req.CoverageAmount))
			eligType := req.EligibilityType
			if eligType == "" {
				eligType = "all"
			}
			var desc, prov *string
			if req.Description != "" {
				desc = &req.Description
			}
			if req.Provider != "" {
				prov = &req.Provider
			}
			plan, err := a.Queries.CreateBenefitPlan(c.Request.Context(), store.CreateBenefitPlanParams{
				CompanyID:         companyID,
				Name:              req.Name,
				Category:          req.Category,
				Description:       desc,
				Provider:          prov,
				EmployerShare:     erShare,
				EmployeeShare:     eeShare,
				CoverageAmount:    covAmt,
				EligibilityType:   eligType,
				EligibilityMonths: req.EligibilityMonths,
			})
			if err != nil {
				response.InternalError(c, "Failed to create benefit plan")
				return
			}
			response.Created(c, plan)
		})

		protected.PUT("/benefits/plans/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			planID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Name              string  `json:"name" binding:"required"`
				Category          string  `json:"category" binding:"required"`
				Description       string  `json:"description"`
				Provider          string  `json:"provider"`
				EmployerShare     float64 `json:"employer_share"`
				EmployeeShare     float64 `json:"employee_share"`
				CoverageAmount    float64 `json:"coverage_amount"`
				EligibilityType   string  `json:"eligibility_type"`
				EligibilityMonths int32   `json:"eligibility_months"`
				IsActive          bool    `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			var erShare, eeShare, covAmt pgtype.Numeric
			_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
			_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
			_ = covAmt.Scan(fmt.Sprintf("%.2f", req.CoverageAmount))
			var desc, prov *string
			if req.Description != "" {
				desc = &req.Description
			}
			if req.Provider != "" {
				prov = &req.Provider
			}
			plan, err := a.Queries.UpdateBenefitPlan(c.Request.Context(), store.UpdateBenefitPlanParams{
				ID:                planID,
				CompanyID:         companyID,
				Name:              req.Name,
				Category:          req.Category,
				Description:       desc,
				Provider:          prov,
				EmployerShare:     erShare,
				EmployeeShare:     eeShare,
				CoverageAmount:    covAmt,
				EligibilityType:   req.EligibilityType,
				EligibilityMonths: req.EligibilityMonths,
				IsActive:          req.IsActive,
			})
			if err != nil {
				response.InternalError(c, "Failed to update benefit plan")
				return
			}
			response.OK(c, plan)
		})

		protected.GET("/benefits/summary", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			summary, err := a.Queries.GetBenefitsSummary(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get benefits summary")
				return
			}
			response.OK(c, summary)
		})

		// Benefit Enrollments
		protected.GET("/benefits/enrollments", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
			enrollments, err := a.Queries.ListBenefitEnrollments(c.Request.Context(), store.ListBenefitEnrollmentsParams{
				CompanyID:  companyID,
				Status:     status,
				EmployeeID: employeeID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list enrollments")
				return
			}
			response.OK(c, enrollments)
		})

		protected.GET("/benefits/my-enrollments", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			enrollments, err := a.Queries.ListMyEnrollments(c.Request.Context(), store.ListMyEnrollmentsParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list enrollments")
				return
			}
			response.OK(c, enrollments)
		})

		protected.POST("/benefits/enrollments", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				EmployeeID    int64   `json:"employee_id" binding:"required"`
				PlanID        int64   `json:"plan_id" binding:"required"`
				EffectiveDate string  `json:"effective_date" binding:"required"`
				EmployerShare float64 `json:"employer_share"`
				EmployeeShare float64 `json:"employee_share"`
				Notes         string  `json:"notes"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			effDate, err := time.Parse("2006-01-02", req.EffectiveDate)
			if err != nil {
				response.BadRequest(c, "Invalid date format")
				return
			}
			var erShare, eeShare pgtype.Numeric
			_ = erShare.Scan(fmt.Sprintf("%.2f", req.EmployerShare))
			_ = eeShare.Scan(fmt.Sprintf("%.2f", req.EmployeeShare))
			var notes *string
			if req.Notes != "" {
				notes = &req.Notes
			}
			enrollment, err := a.Queries.CreateBenefitEnrollment(c.Request.Context(), store.CreateBenefitEnrollmentParams{
				CompanyID:     companyID,
				EmployeeID:    req.EmployeeID,
				PlanID:        req.PlanID,
				Status:        "active",
				EffectiveDate: effDate,
				EmployerShare: erShare,
				EmployeeShare: eeShare,
				Notes:         notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to create enrollment")
				return
			}
			response.Created(c, enrollment)
		})

		protected.POST("/benefits/enrollments/:id/cancel", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			enrollmentID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			enrollment, err := a.Queries.CancelBenefitEnrollment(c.Request.Context(), store.CancelBenefitEnrollmentParams{
				ID: enrollmentID, CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to cancel enrollment")
				return
			}
			response.OK(c, enrollment)
		})

		// Benefit Dependents
		protected.GET("/benefits/enrollments/:id/dependents", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			enrollmentID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			deps, err := a.Queries.ListBenefitDependents(c.Request.Context(), store.ListBenefitDependentsParams{
				EnrollmentID: enrollmentID, CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list dependents")
				return
			}
			response.OK(c, deps)
		})

		protected.POST("/benefits/enrollments/:id/dependents", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			employeeID := emp.ID
			enrollmentID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Name         string `json:"name" binding:"required"`
				Relationship string `json:"relationship" binding:"required"`
				BirthDate    string `json:"birth_date"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			var birthDate pgtype.Date
			if req.BirthDate != "" {
				t, err := time.Parse("2006-01-02", req.BirthDate)
				if err != nil {
					response.BadRequest(c, "Invalid birth date")
					return
				}
				birthDate = pgtype.Date{Time: t, Valid: true}
			}
			dep, err := a.Queries.CreateBenefitDependent(c.Request.Context(), store.CreateBenefitDependentParams{
				CompanyID:    companyID,
				EmployeeID:   employeeID,
				EnrollmentID: enrollmentID,
				Name:         req.Name,
				Relationship: req.Relationship,
				BirthDate:    birthDate,
			})
			if err != nil {
				response.InternalError(c, "Failed to add dependent")
				return
			}
			response.Created(c, dep)
		})

		protected.DELETE("/benefits/dependents/:id", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			depID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			err := a.Queries.DeleteBenefitDependent(c.Request.Context(), store.DeleteBenefitDependentParams{
				ID: depID, CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to delete dependent")
				return
			}
			response.OK(c, gin.H{"message": "Dependent deleted"})
		})

		// Benefit Claims
		protected.GET("/benefits/claims", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 50 }
			claims, err := a.Queries.ListBenefitClaims(c.Request.Context(), store.ListBenefitClaimsParams{
				CompanyID:  companyID,
				Status:     status,
				EmployeeID: employeeID,
				Off:        int32((page - 1) * limit),
				Lim:        int32(limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list claims")
				return
			}
			total, _ := a.Queries.CountBenefitClaims(c.Request.Context(), store.CountBenefitClaimsParams{
				CompanyID:  companyID,
				Status:     status,
				EmployeeID: employeeID,
			})
			response.OK(c, gin.H{"items": claims, "total": total, "page": page, "limit": limit})
		})

		protected.POST("/benefits/claims", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			employeeID := emp.ID
			var req struct {
				EnrollmentID int64   `json:"enrollment_id" binding:"required"`
				ClaimDate    string  `json:"claim_date" binding:"required"`
				Amount       float64 `json:"amount" binding:"required"`
				Description  string  `json:"description" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			claimDate, err := time.Parse("2006-01-02", req.ClaimDate)
			if err != nil {
				response.BadRequest(c, "Invalid date format")
				return
			}
			var amount pgtype.Numeric
			_ = amount.Scan(fmt.Sprintf("%.2f", req.Amount))
			claim, err := a.Queries.CreateBenefitClaim(c.Request.Context(), store.CreateBenefitClaimParams{
				CompanyID:    companyID,
				EmployeeID:   employeeID,
				EnrollmentID: req.EnrollmentID,
				ClaimDate:    claimDate,
				Amount:       amount,
				Description:  req.Description,
			})
			if err != nil {
				response.InternalError(c, "Failed to create claim")
				return
			}
			response.Created(c, claim)
		})

		protected.POST("/benefits/claims/:id/approve", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			claimID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			claim, err := a.Queries.ApproveBenefitClaim(c.Request.Context(), store.ApproveBenefitClaimParams{
				ID: claimID, CompanyID: companyID, ApprovedBy: &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to approve claim")
				return
			}
			response.OK(c, claim)
		})

		protected.POST("/benefits/claims/:id/reject", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			claimID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&req)
			var reason *string
			if req.Reason != "" {
				reason = &req.Reason
			}
			claim, err := a.Queries.RejectBenefitClaim(c.Request.Context(), store.RejectBenefitClaimParams{
				ID: claimID, CompanyID: companyID, RejectionReason: reason,
			})
			if err != nil {
				response.InternalError(c, "Failed to reject claim")
				return
			}
			response.OK(c, claim)
		})

		// 201 File Document Management
		protected.GET("/201file/categories", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			_ = a.Queries.EnsureDefaultCategories(c.Request.Context(), companyID)
			cats, err := a.Queries.ListDocumentCategories(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list categories")
				return
			}
			response.OK(c, cats)
		})

		protected.POST("/201file/categories", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				Name        string `json:"name" binding:"required"`
				Slug        string `json:"slug" binding:"required"`
				Description string `json:"description"`
				SortOrder   int32  `json:"sort_order"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			cat, err := a.Queries.CreateDocumentCategory(c.Request.Context(), store.CreateDocumentCategoryParams{
				CompanyID:   companyID,
				Name:        req.Name,
				Slug:        req.Slug,
				Description: &req.Description,
				SortOrder:   req.SortOrder,
			})
			if err != nil {
				response.InternalError(c, "Failed to create category")
				return
			}
			response.Created(c, cat)
		})

		protected.GET("/201file/employee/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			categoryID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
			status := c.Query("status")
			docs, err := a.Queries.List201Documents(c.Request.Context(), store.List201DocumentsParams{
				CompanyID:  companyID,
				EmployeeID: empID,
				CategoryID: categoryID,
				Status:     status,
			})
			if err != nil {
				response.InternalError(c, "Failed to list documents")
				return
			}
			response.OK(c, docs)
		})

		protected.GET("/201file/employee/:id/stats", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			stats, err := a.Queries.GetEmployee201Stats(c.Request.Context(), store.GetEmployee201StatsParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to get stats")
				return
			}
			response.OK(c, stats)
		})

		protected.POST("/201file/employee/:id/upload", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

			file, header, err := c.Request.FormFile("file")
			if err != nil {
				response.BadRequest(c, "File is required")
				return
			}
			defer file.Close()

			title := c.PostForm("title")
			docType := c.PostForm("doc_type")
			if docType == "" {
				docType = "general"
			}
			categoryIDStr := c.PostForm("category_id")

			uploadDir := fmt.Sprintf("uploads/201file/%d/%d", companyID, empID)
			if err := os.MkdirAll(uploadDir, 0755); err != nil {
				response.InternalError(c, "Failed to create upload directory")
				return
			}

			fileName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), header.Filename)
			filePath := filepath.Join(uploadDir, fileName)

			out, err := os.Create(filePath)
			if err != nil {
				response.InternalError(c, "Failed to save file")
				return
			}
			defer out.Close()

			written, err := io.Copy(out, file)
			if err != nil {
				response.InternalError(c, "Failed to write file")
				return
			}

			mimeType := header.Header.Get("Content-Type")
			var expiryDate pgtype.Date
			if ed := c.PostForm("expiry_date"); ed != "" {
				if parsed, err := time.Parse("2006-01-02", ed); err == nil {
					expiryDate = pgtype.Date{Time: parsed, Valid: true}
				}
			}

			var catID *int64
			if cid, err := strconv.ParseInt(categoryIDStr, 10, 64); err == nil && cid > 0 {
				catID = &cid
			}

			var titlePtr *string
			if title != "" {
				titlePtr = &title
			}

			var notes *string
			if n := c.PostForm("notes"); n != "" {
				notes = &n
			}

			doc, err := a.Queries.Upload201Document(c.Request.Context(), store.Upload201DocumentParams{
				CompanyID:  companyID,
				EmployeeID: empID,
				CategoryID: catID,
				Title:      titlePtr,
				DocType:    docType,
				FileName:   header.Filename,
				FilePath:   filePath,
				FileSize:   written,
				MimeType:   &mimeType,
				Version:    1,
				ExpiryDate: expiryDate,
				UploadedBy: &userID,
				Notes:      notes,
			})
			if err != nil {
				response.InternalError(c, "Failed to save document record")
				return
			}
			response.Created(c, doc)
		})

		protected.GET("/201file/document/:doc_id/download", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			docID, err := uuid.Parse(c.Param("doc_id"))
			if err != nil {
				response.BadRequest(c, "Invalid document ID")
				return
			}
			doc, err := a.Queries.Get201Document(c.Request.Context(), store.Get201DocumentParams{
				ID: docID, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Document not found")
				return
			}
			c.FileAttachment(doc.FilePath, doc.FileName)
		})

		protected.PUT("/201file/document/:doc_id", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			docID, err := uuid.Parse(c.Param("doc_id"))
			if err != nil {
				response.BadRequest(c, "Invalid document ID")
				return
			}
			var req struct {
				Title      string `json:"title"`
				CategoryID *int64 `json:"category_id"`
				ExpiryDate string `json:"expiry_date"`
				Notes      string `json:"notes"`
				Status     string `json:"status"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			var expiryDate pgtype.Date
			if req.ExpiryDate != "" {
				if parsed, err := time.Parse("2006-01-02", req.ExpiryDate); err == nil {
					expiryDate = pgtype.Date{Time: parsed, Valid: true}
				}
			}
			var titlePtr, notesPtr *string
			if req.Title != "" {
				titlePtr = &req.Title
			}
			if req.Notes != "" {
				notesPtr = &req.Notes
			}
			status := req.Status
			if status == "" {
				status = "active"
			}
			doc, err := a.Queries.Update201Document(c.Request.Context(), store.Update201DocumentParams{
				ID:         docID,
				CompanyID:  companyID,
				Title:      titlePtr,
				CategoryID: req.CategoryID,
				ExpiryDate: expiryDate,
				Notes:      notesPtr,
				Status:     status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update document")
				return
			}
			response.OK(c, doc)
		})

		protected.DELETE("/201file/document/:doc_id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			docID, err := uuid.Parse(c.Param("doc_id"))
			if err != nil {
				response.BadRequest(c, "Invalid document ID")
				return
			}
			doc, err := a.Queries.Get201Document(c.Request.Context(), store.Get201DocumentParams{
				ID: docID, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Document not found")
				return
			}
			_ = os.Remove(doc.FilePath)
			if err := a.Queries.Delete201Document(c.Request.Context(), store.Delete201DocumentParams{
				ID: docID, CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to delete document")
				return
			}
			response.OK(c, gin.H{"message": "Document deleted"})
		})

		protected.GET("/201file/expiring", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			docs, err := a.Queries.List201ExpiringDocuments(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list expiring documents")
				return
			}
			response.OK(c, docs)
		})

		protected.GET("/201file/employee/:id/compliance", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			empID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			checklist, err := a.Queries.GetComplianceChecklist(c.Request.Context(), store.GetComplianceChecklistParams{
				CompanyID:  companyID,
				EmployeeID: empID,
			})
			if err != nil {
				response.InternalError(c, "Failed to get compliance checklist")
				return
			}
			response.OK(c, checklist)
		})

		// Document Requirements
		protected.GET("/201file/requirements", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			reqs, err := a.Queries.ListDocumentRequirements(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list requirements")
				return
			}
			response.OK(c, reqs)
		})

		protected.POST("/201file/requirements", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				CategoryID   int64  `json:"category_id" binding:"required"`
				DocumentName string `json:"document_name" binding:"required"`
				IsRequired   bool   `json:"is_required"`
				AppliesTo    string `json:"applies_to"`
				ExpiryMonths *int32 `json:"expiry_months"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			appliesTo := req.AppliesTo
			if appliesTo == "" {
				appliesTo = "all"
			}
			r, err := a.Queries.CreateDocumentRequirement(c.Request.Context(), store.CreateDocumentRequirementParams{
				CompanyID:    companyID,
				CategoryID:   req.CategoryID,
				DocumentName: req.DocumentName,
				IsRequired:   req.IsRequired,
				AppliesTo:    appliesTo,
				ExpiryMonths: req.ExpiryMonths,
			})
			if err != nil {
				response.InternalError(c, "Failed to create requirement")
				return
			}
			response.Created(c, r)
		})

		protected.DELETE("/201file/requirements/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			reqID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			if err := a.Queries.DeleteDocumentRequirement(c.Request.Context(), store.DeleteDocumentRequirementParams{
				ID: reqID, CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to delete requirement")
				return
			}
			response.OK(c, gin.H{"message": "Requirement deleted"})
		})

		// Company Policies & Acknowledgments
		protected.GET("/policies", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			policies, err := a.Queries.ListPolicies(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list policies")
				return
			}
			response.OK(c, policies)
		})

		protected.GET("/policies/:id", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			policy, err := a.Queries.GetPolicy(c.Request.Context(), store.GetPolicyParams{
				ID: id, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Policy not found")
				return
			}
			response.OK(c, policy)
		})

		protected.POST("/policies", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			var req struct {
				Title                  string `json:"title" binding:"required"`
				Content                string `json:"content" binding:"required"`
				Category               string `json:"category"`
				EffectiveDate          string `json:"effective_date"`
				RequiresAcknowledgment bool   `json:"requires_acknowledgment"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			category := req.Category
			if category == "" {
				category = "general"
			}
			effDate := time.Now()
			if req.EffectiveDate != "" {
				if parsed, err := time.Parse("2006-01-02", req.EffectiveDate); err == nil {
					effDate = parsed
				}
			}
			policy, err := a.Queries.CreatePolicy(c.Request.Context(), store.CreatePolicyParams{
				CompanyID:              companyID,
				Title:                  req.Title,
				Content:                req.Content,
				Category:               category,
				Version:                1,
				EffectiveDate:          effDate,
				RequiresAcknowledgment: req.RequiresAcknowledgment,
				CreatedBy:              &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to create policy")
				return
			}
			response.Created(c, policy)
		})

		protected.PUT("/policies/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Title                  string `json:"title" binding:"required"`
				Content                string `json:"content" binding:"required"`
				Category               string `json:"category"`
				Version                int32  `json:"version"`
				EffectiveDate          string `json:"effective_date"`
				RequiresAcknowledgment bool   `json:"requires_acknowledgment"`
				IsActive               bool   `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			effDate := time.Now()
			if req.EffectiveDate != "" {
				if parsed, err := time.Parse("2006-01-02", req.EffectiveDate); err == nil {
					effDate = parsed
				}
			}
			policy, err := a.Queries.UpdatePolicy(c.Request.Context(), store.UpdatePolicyParams{
				ID:                     id,
				CompanyID:              companyID,
				Title:                  req.Title,
				Content:                req.Content,
				Category:               req.Category,
				Version:                req.Version,
				EffectiveDate:          effDate,
				RequiresAcknowledgment: req.RequiresAcknowledgment,
				IsActive:               req.IsActive,
			})
			if err != nil {
				response.InternalError(c, "Failed to update policy")
				return
			}
			response.OK(c, policy)
		})

		protected.DELETE("/policies/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			_, err := a.Queries.DeactivatePolicy(c.Request.Context(), store.DeactivatePolicyParams{
				ID: id, CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to deactivate policy")
				return
			}
			response.OK(c, gin.H{"message": "Policy deactivated"})
		})

		protected.POST("/policies/:id/acknowledge", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			ipAddr := c.ClientIP()
			ack, err := a.Queries.AcknowledgePolicy(c.Request.Context(), store.AcknowledgePolicyParams{
				CompanyID:  companyID,
				PolicyID:   id,
				EmployeeID: emp.ID,
				IpAddress:  &ipAddr,
			})
			if err != nil {
				response.InternalError(c, "Failed to acknowledge policy")
				return
			}
			response.OK(c, ack)
		})

		protected.GET("/policies/:id/acknowledgments", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			acks, err := a.Queries.ListPolicyAcknowledgments(c.Request.Context(), store.ListPolicyAcknowledgmentsParams{
				PolicyID: id, CompanyID: companyID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list acknowledgments")
				return
			}
			response.OK(c, acks)
		})

		protected.GET("/policies/:id/stats", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			stats, err := a.Queries.GetPolicyAckStats(c.Request.Context(), store.GetPolicyAckStatsParams{
				CompanyID: companyID, PolicyID: id,
			})
			if err != nil {
				response.InternalError(c, "Failed to get stats")
				return
			}
			response.OK(c, stats)
		})

		protected.GET("/policies/pending", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.OK(c, []any{})
				return
			}
			policies, err := a.Queries.ListUnacknowledgedPolicies(c.Request.Context(), store.ListUnacknowledgedPoliciesParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list pending policies")
				return
			}
			response.OK(c, policies)
		})

		// Grievance Management
		protected.GET("/grievances/summary", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			summary, err := a.Queries.GetGrievanceSummary(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get grievance summary")
				return
			}
			response.OK(c, summary)
		})

		protected.GET("/grievances", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			category := c.Query("category")
			employeeID, _ := strconv.ParseInt(c.Query("employee_id"), 10, 64)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 50 }
			cases, err := a.Queries.ListGrievances(c.Request.Context(), store.ListGrievancesParams{
				CompanyID:  companyID,
				Status:     status,
				Category:   category,
				EmployeeID: employeeID,
				Off:        int32((page - 1) * limit),
				Lim:        int32(limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list grievances")
				return
			}
			total, _ := a.Queries.CountGrievances(c.Request.Context(), store.CountGrievancesParams{
				CompanyID:  companyID,
				Status:     status,
				Category:   category,
				EmployeeID: employeeID,
			})
			response.OK(c, gin.H{"items": cases, "total": total, "page": page, "limit": limit})
		})

		protected.GET("/grievances/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			g, err := a.Queries.GetGrievance(c.Request.Context(), store.GetGrievanceParams{
				ID: id, CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Grievance not found")
				return
			}
			response.OK(c, g)
		})

		protected.POST("/grievances", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			var req struct {
				Category    string `json:"category" binding:"required"`
				Subject     string `json:"subject" binding:"required"`
				Description string `json:"description" binding:"required"`
				Severity    string `json:"severity"`
				IsAnonymous bool   `json:"is_anonymous"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			severity := req.Severity
			if severity == "" {
				severity = "medium"
			}
			nextNum, _ := a.Queries.NextGrievanceCaseNumber(c.Request.Context(), companyID)
			caseNumber := fmt.Sprintf("GRV-%04d", nextNum)
			g, err := a.Queries.CreateGrievance(c.Request.Context(), store.CreateGrievanceParams{
				CompanyID:   companyID,
				EmployeeID:  emp.ID,
				CaseNumber:  caseNumber,
				Category:    req.Category,
				Subject:     req.Subject,
				Description: req.Description,
				Severity:    severity,
				IsAnonymous: req.IsAnonymous,
			})
			if err != nil {
				response.InternalError(c, "Failed to create grievance")
				return
			}
			response.Created(c, g)
		})

		protected.GET("/grievances/my", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			cases, err := a.Queries.ListMyGrievances(c.Request.Context(), store.ListMyGrievancesParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list grievances")
				return
			}
			response.OK(c, cases)
		})

		protected.PUT("/grievances/:id/status", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Status string `json:"status" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			g, err := a.Queries.UpdateGrievanceStatus(c.Request.Context(), store.UpdateGrievanceStatusParams{
				ID: id, CompanyID: companyID, Status: req.Status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update status")
				return
			}
			response.OK(c, g)
		})

		protected.POST("/grievances/:id/assign", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				AssignedTo int64 `json:"assigned_to" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			g, err := a.Queries.AssignGrievance(c.Request.Context(), store.AssignGrievanceParams{
				ID: id, CompanyID: companyID, AssignedTo: &req.AssignedTo,
			})
			if err != nil {
				response.InternalError(c, "Failed to assign grievance")
				return
			}
			response.OK(c, g)
		})

		protected.POST("/grievances/:id/resolve", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Resolution string `json:"resolution" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			g, err := a.Queries.ResolveGrievance(c.Request.Context(), store.ResolveGrievanceParams{
				ID: id, CompanyID: companyID, Resolution: &req.Resolution,
			})
			if err != nil {
				response.InternalError(c, "Failed to resolve grievance")
				return
			}
			response.OK(c, g)
		})

		protected.POST("/grievances/:id/withdraw", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID, UserID: &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			g, err := a.Queries.WithdrawGrievance(c.Request.Context(), store.WithdrawGrievanceParams{
				ID: id, CompanyID: companyID, EmployeeID: emp.ID,
			})
			if err != nil {
				response.InternalError(c, "Failed to withdraw grievance")
				return
			}
			response.OK(c, g)
		})

		protected.GET("/grievances/:id/comments", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			comments, err := a.Queries.ListGrievanceComments(c.Request.Context(), id)
			if err != nil {
				response.InternalError(c, "Failed to list comments")
				return
			}
			response.OK(c, comments)
		})

		protected.POST("/grievances/:id/comments", auth.ManagerOrAbove(), func(c *gin.Context) {
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Comment    string `json:"comment" binding:"required"`
				IsInternal bool   `json:"is_internal"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request: "+err.Error())
				return
			}
			comment, err := a.Queries.AddGrievanceComment(c.Request.Context(), store.AddGrievanceCommentParams{
				GrievanceID: id,
				UserID:      userID,
				Comment:     req.Comment,
				IsInternal:  req.IsInternal,
			})
			if err != nil {
				response.InternalError(c, "Failed to add comment")
				return
			}
			response.Created(c, comment)
		})

		// Expense Reimbursement
		protected.GET("/expenses/categories", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			cats, err := a.Queries.ListExpenseCategories(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list expense categories")
				return
			}
			response.OK(c, cats)
		})

		protected.POST("/expenses/categories", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				Name            string  `json:"name" binding:"required"`
				Description     *string `json:"description"`
				MaxAmount       float64 `json:"max_amount"`
				RequiresReceipt bool    `json:"requires_receipt"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			var maxAmt pgtype.Numeric
			if req.MaxAmount > 0 {
				_ = maxAmt.Scan(fmt.Sprintf("%.2f", req.MaxAmount))
			}
			cat, err := a.Queries.CreateExpenseCategory(c.Request.Context(), store.CreateExpenseCategoryParams{
				CompanyID:       companyID,
				Name:            req.Name,
				Description:     req.Description,
				MaxAmount:       maxAmt,
				RequiresReceipt: req.RequiresReceipt,
			})
			if err != nil {
				response.InternalError(c, "Failed to create expense category")
				return
			}
			response.Created(c, cat)
		})

		protected.PUT("/expenses/categories/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Name            string  `json:"name"`
				Description     *string `json:"description"`
				MaxAmount       float64 `json:"max_amount"`
				RequiresReceipt bool    `json:"requires_receipt"`
				IsActive        bool    `json:"is_active"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			var maxAmt pgtype.Numeric
			if req.MaxAmount > 0 {
				_ = maxAmt.Scan(fmt.Sprintf("%.2f", req.MaxAmount))
			}
			cat, err := a.Queries.UpdateExpenseCategory(c.Request.Context(), store.UpdateExpenseCategoryParams{
				ID:              id,
				CompanyID:       companyID,
				Name:            req.Name,
				Description:     req.Description,
				MaxAmount:       maxAmt,
				RequiresReceipt: req.RequiresReceipt,
				IsActive:        req.IsActive,
			})
			if err != nil {
				response.InternalError(c, "Failed to update expense category")
				return
			}
			response.OK(c, cat)
		})

		protected.GET("/expenses/summary", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			summary, err := a.Queries.GetExpenseSummary(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to get expense summary")
				return
			}
			response.OK(c, summary)
		})

		protected.GET("/expenses", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			pg := pagination.Parse(c)
			statusFilter := c.DefaultQuery("status", "")
			var employeeIDFilter int64
			if eid := c.Query("employee_id"); eid != "" {
				employeeIDFilter, _ = strconv.ParseInt(eid, 10, 64)
			}
			claims, err := a.Queries.ListExpenseClaims(c.Request.Context(), store.ListExpenseClaimsParams{
				CompanyID:  companyID,
				Status:     statusFilter,
				EmployeeID: employeeIDFilter,
				Lim:        int32(pg.Limit),
				Off:        int32(pg.Offset),
			})
			if err != nil {
				response.InternalError(c, "Failed to list expense claims")
				return
			}
			count, _ := a.Queries.CountExpenseClaims(c.Request.Context(), store.CountExpenseClaimsParams{
				CompanyID:  companyID,
				Status:     statusFilter,
				EmployeeID: employeeIDFilter,
			})
			response.Paginated(c, claims, count, pg.Page, pg.Limit)
		})

		protected.GET("/expenses/my", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID,
				UserID:    &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			claims, err := a.Queries.ListMyExpenseClaims(c.Request.Context(), store.ListMyExpenseClaimsParams{
				CompanyID:  companyID,
				EmployeeID: emp.ID,
			})
			if err != nil {
				response.InternalError(c, "Failed to list expense claims")
				return
			}
			response.OK(c, claims)
		})

		protected.GET("/expenses/:id", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			claim, err := a.Queries.GetExpenseClaim(c.Request.Context(), store.GetExpenseClaimParams{
				ID:        id,
				CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Expense claim not found")
				return
			}
			response.OK(c, claim)
		})

		protected.POST("/expenses", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID,
				UserID:    &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			var req struct {
				CategoryID  int64   `json:"category_id" binding:"required"`
				Description string  `json:"description" binding:"required"`
				Amount      float64 `json:"amount" binding:"required"`
				Currency    string  `json:"currency"`
				ExpenseDate string  `json:"expense_date" binding:"required"`
				ReceiptPath *string `json:"receipt_path"`
				Notes       *string `json:"notes"`
				Submit      bool    `json:"submit"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			expDate, err := time.Parse("2006-01-02", req.ExpenseDate)
			if err != nil {
				response.BadRequest(c, "Invalid expense_date format")
				return
			}
			nextNum, _ := a.Queries.NextExpenseClaimNumber(c.Request.Context(), companyID)
			claimNumber := fmt.Sprintf("EXP-%06d", nextNum)
			currency := req.Currency
			if currency == "" {
				currency = "PHP"
			}
			status := "draft"
			if req.Submit {
				status = "submitted"
			}
			var amount pgtype.Numeric
			_ = amount.Scan(fmt.Sprintf("%.2f", req.Amount))
			claim, err := a.Queries.CreateExpenseClaim(c.Request.Context(), store.CreateExpenseClaimParams{
				CompanyID:   companyID,
				EmployeeID:  emp.ID,
				ClaimNumber: claimNumber,
				CategoryID:  req.CategoryID,
				Description: req.Description,
				Amount:      amount,
				Currency:    currency,
				ExpenseDate: expDate,
				ReceiptPath: req.ReceiptPath,
				Status:      status,
				Notes:       req.Notes,
			})
			if err != nil {
				a.Logger.Error("failed to create expense claim", "error", err)
				response.InternalError(c, "Failed to create expense claim")
				return
			}
			response.Created(c, claim)
		})

		protected.POST("/expenses/:id/submit", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID,
				UserID:    &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			claim, err := a.Queries.SubmitExpenseClaim(c.Request.Context(), store.SubmitExpenseClaimParams{
				ID:         id,
				CompanyID:  companyID,
				EmployeeID: emp.ID,
			})
			if err != nil {
				response.BadRequest(c, "Failed to submit expense claim")
				return
			}
			response.OK(c, claim)
		})

		protected.POST("/expenses/:id/approve", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID,
				UserID:    &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			claim, err := a.Queries.ApproveExpenseClaim(c.Request.Context(), store.ApproveExpenseClaimParams{
				ID:         id,
				CompanyID:  companyID,
				ApproverID: &emp.ID,
			})
			if err != nil {
				response.BadRequest(c, "Failed to approve expense claim")
				return
			}
			response.OK(c, claim)
		})

		protected.POST("/expenses/:id/reject", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Reason string `json:"reason"`
			}
			_ = c.ShouldBindJSON(&req)
			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				CompanyID: companyID,
				UserID:    &userID,
			})
			if err != nil {
				response.BadRequest(c, "Employee profile not found")
				return
			}
			claim, err := a.Queries.RejectExpenseClaim(c.Request.Context(), store.RejectExpenseClaimParams{
				ID:              id,
				CompanyID:       companyID,
				ApproverID:      &emp.ID,
				RejectionReason: &req.Reason,
			})
			if err != nil {
				response.BadRequest(c, "Failed to reject expense claim")
				return
			}
			response.OK(c, claim)
		})

		protected.POST("/expenses/:id/mark-paid", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Reference string `json:"reference"`
			}
			_ = c.ShouldBindJSON(&req)
			claim, err := a.Queries.MarkExpenseClaimPaid(c.Request.Context(), store.MarkExpenseClaimPaidParams{
				ID:            id,
				CompanyID:     companyID,
				PaidReference: &req.Reference,
			})
			if err != nil {
				response.BadRequest(c, "Failed to mark expense as paid")
				return
			}
			response.OK(c, claim)
		})

		// Training & Certification
		protected.GET("/trainings", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 50 }
			items, err := a.Queries.ListTrainings(c.Request.Context(), store.ListTrainingsParams{
				CompanyID: companyID,
				Limit:     int32(limit),
				Offset:    int32((page - 1) * limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list trainings")
				return
			}
			response.OK(c, items)
		})
		protected.POST("/trainings", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Title           string  `json:"title" binding:"required"`
				Description     *string `json:"description"`
				Trainer         *string `json:"trainer"`
				TrainingType    string  `json:"training_type"`
				StartDate       string  `json:"start_date" binding:"required"`
				EndDate         *string `json:"end_date"`
				MaxParticipants *int32  `json:"max_participants"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			startDate, _ := time.Parse("2006-01-02", req.StartDate)
			var endDate pgtype.Date
			if req.EndDate != nil {
				parsed, _ := time.Parse("2006-01-02", *req.EndDate)
				endDate = pgtype.Date{Time: parsed, Valid: true}
			}
			if req.TrainingType == "" { req.TrainingType = "internal" }
			training, err := a.Queries.CreateTraining(c.Request.Context(), store.CreateTrainingParams{
				CompanyID:       companyID,
				Title:           req.Title,
				Description:     req.Description,
				Trainer:         req.Trainer,
				TrainingType:    req.TrainingType,
				StartDate:       startDate,
				EndDate:         endDate,
				MaxParticipants: req.MaxParticipants,
				CreatedBy:       &userID,
			})
			if err != nil {
				response.InternalError(c, "Failed to create training")
				return
			}
			response.Created(c, training)
		})
		protected.PUT("/trainings/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct { Status string `json:"status" binding:"required"` }
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			training, err := a.Queries.UpdateTrainingStatus(c.Request.Context(), store.UpdateTrainingStatusParams{
				ID: id, CompanyID: companyID, Status: req.Status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update training")
				return
			}
			response.OK(c, training)
		})
		protected.GET("/trainings/:id/participants", func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			items, err := a.Queries.ListTrainingParticipants(c.Request.Context(), id)
			if err != nil {
				response.InternalError(c, "Failed to list participants")
				return
			}
			response.OK(c, items)
		})
		protected.POST("/trainings/:id/participants", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct { EmployeeID int64 `json:"employee_id" binding:"required"` }
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			p, err := a.Queries.AddTrainingParticipant(c.Request.Context(), store.AddTrainingParticipantParams{
				TrainingID: id, EmployeeID: req.EmployeeID,
			})
			if err != nil {
				response.InternalError(c, "Failed to add participant")
				return
			}
			response.Created(c, p)
		})

		protected.GET("/certifications", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 50 }
			var empID int64
			if eid := c.Query("employee_id"); eid != "" {
				empID, _ = strconv.ParseInt(eid, 10, 64)
			}
			items, err := a.Queries.ListCertifications(c.Request.Context(), store.ListCertificationsParams{
				CompanyID: companyID,
				Column2:   empID,
				Limit:     int32(limit),
				Offset:    int32((page - 1) * limit),
			})
			if err != nil {
				response.InternalError(c, "Failed to list certifications")
				return
			}
			response.OK(c, items)
		})
		protected.POST("/certifications", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				EmployeeID   int64   `json:"employee_id" binding:"required"`
				Name         string  `json:"name" binding:"required"`
				IssuingBody  *string `json:"issuing_body"`
				CredentialID *string `json:"credential_id"`
				IssueDate    string  `json:"issue_date" binding:"required"`
				ExpiryDate   *string `json:"expiry_date"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			issueDate, _ := time.Parse("2006-01-02", req.IssueDate)
			var expiryDate pgtype.Date
			if req.ExpiryDate != nil {
				parsed, _ := time.Parse("2006-01-02", *req.ExpiryDate)
				expiryDate = pgtype.Date{Time: parsed, Valid: true}
			}
			cert, err := a.Queries.CreateCertification(c.Request.Context(), store.CreateCertificationParams{
				CompanyID:    companyID,
				EmployeeID:   req.EmployeeID,
				Name:         req.Name,
				IssuingBody:  req.IssuingBody,
				CredentialID: req.CredentialID,
				IssueDate:    issueDate,
				ExpiryDate:   expiryDate,
			})
			if err != nil {
				response.InternalError(c, "Failed to create certification")
				return
			}
			response.Created(c, cert)
		})
		protected.DELETE("/certifications/:id", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			if err := a.Queries.DeleteCertification(c.Request.Context(), store.DeleteCertificationParams{
				ID: id, CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to delete certification")
				return
			}
			response.OK(c, nil)
		})
		protected.GET("/certifications/expiring", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			items, err := a.Queries.ListExpiringCertifications(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list expiring certifications")
				return
			}
			response.OK(c, items)
		})

		// Clearance / Resignation
		protected.POST("/clearance", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)
			var req struct {
				EmployeeID     int64  `json:"employee_id" binding:"required"`
				ResignationDate string `json:"resignation_date" binding:"required"`
				LastWorkingDay  string `json:"last_working_day" binding:"required"`
				Reason         string `json:"reason"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			resignDate, err := time.Parse("2006-01-02", req.ResignationDate)
			if err != nil {
				response.BadRequest(c, "Invalid resignation date")
				return
			}
			lastDay, err := time.Parse("2006-01-02", req.LastWorkingDay)
			if err != nil {
				response.BadRequest(c, "Invalid last working day")
				return
			}
			cr, err := a.Queries.CreateClearanceRequest(c.Request.Context(), store.CreateClearanceRequestParams{
				CompanyID:       companyID,
				EmployeeID:      req.EmployeeID,
				ResignationDate: resignDate,
				LastWorkingDay:  lastDay,
				Reason:          &req.Reason,
				SubmittedBy:     userID,
			})
			if err != nil {
				a.Logger.Error("failed to create clearance request", "error", err)
				response.InternalError(c, "Failed to create clearance request")
				return
			}

			// Auto-create clearance items from template
			templates, _ := a.Queries.ListClearanceTemplates(c.Request.Context(), companyID)
			if len(templates) == 0 {
				// Default items if no template configured
				defaults := []struct{ dept, item string }{
					{"IT", "Return laptop/equipment"},
					{"IT", "Revoke system access"},
					{"IT", "Email account deactivation"},
					{"HR", "Exit interview completed"},
					{"HR", "ID card returned"},
					{"HR", "Final pay computation"},
					{"HR", "Certificate of Employment"},
					{"Finance", "Cash advance settlement"},
					{"Finance", "Company credit card returned"},
					{"Admin", "Office keys returned"},
					{"Admin", "Parking card returned"},
					{"Direct Manager", "Knowledge transfer completed"},
					{"Direct Manager", "Pending tasks handover"},
				}
				for _, d := range defaults {
					_, _ = a.Queries.CreateClearanceItem(c.Request.Context(), store.CreateClearanceItemParams{
						ClearanceID: cr.ID,
						Department:  d.dept,
						ItemName:    d.item,
					})
				}
			} else {
				for _, t := range templates {
					_, _ = a.Queries.CreateClearanceItem(c.Request.Context(), store.CreateClearanceItemParams{
						ClearanceID: cr.ID,
						Department:  t.Department,
						ItemName:    t.ItemName,
					})
				}
			}

			response.Created(c, cr)
		})

		protected.GET("/clearance", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			status := c.Query("status")
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
			offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

			items, err := a.Queries.ListClearanceRequests(c.Request.Context(), store.ListClearanceRequestsParams{
				CompanyID: companyID,
				Column2:   status,
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
			if err != nil {
				response.InternalError(c, "Failed to list clearance requests")
				return
			}
			count, _ := a.Queries.CountClearanceRequests(c.Request.Context(), store.CountClearanceRequestsParams{
				CompanyID: companyID,
				Column2:   status,
			})
			response.Paginated(c, items, count, offset/limit+1, limit)
		})

		protected.GET("/clearance/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			cr, err := a.Queries.GetClearanceRequest(c.Request.Context(), store.GetClearanceRequestParams{
				ID:        id,
				CompanyID: companyID,
			})
			if err != nil {
				response.NotFound(c, "Clearance request not found")
				return
			}
			items, err := a.Queries.ListClearanceItems(c.Request.Context(), id)
			if err != nil {
				response.InternalError(c, "Failed to list clearance items")
				return
			}
			response.OK(c, map[string]any{
				"request": cr,
				"items":   items,
			})
		})

		protected.PUT("/clearance/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			var req struct {
				Status string `json:"status" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			cr, err := a.Queries.UpdateClearanceStatus(c.Request.Context(), store.UpdateClearanceStatusParams{
				ID:        id,
				CompanyID: companyID,
				Status:    req.Status,
			})
			if err != nil {
				response.InternalError(c, "Failed to update clearance status")
				return
			}
			response.OK(c, cr)
		})

		protected.PUT("/clearance/items/:id", auth.ManagerOrAbove(), func(c *gin.Context) {
			userID := auth.GetUserID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			var req struct {
				Status  string  `json:"status" binding:"required"`
				Remarks *string `json:"remarks"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			item, err := a.Queries.UpdateClearanceItem(c.Request.Context(), store.UpdateClearanceItemParams{
				ID:        id,
				Status:    req.Status,
				ClearedBy: &userID,
				Remarks:   req.Remarks,
			})
			if err != nil {
				response.InternalError(c, "Failed to update clearance item")
				return
			}
			response.OK(c, item)
		})

		// Clearance Templates
		protected.GET("/clearance/templates", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			templates, err := a.Queries.ListClearanceTemplates(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list templates")
				return
			}
			response.OK(c, templates)
		})

		protected.POST("/clearance/templates", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			var req struct {
				Department string `json:"department" binding:"required"`
				ItemName   string `json:"item_name" binding:"required"`
				SortOrder  int32  `json:"sort_order"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Invalid request")
				return
			}
			tmpl, err := a.Queries.CreateClearanceTemplate(c.Request.Context(), store.CreateClearanceTemplateParams{
				CompanyID:  companyID,
				Department: req.Department,
				ItemName:   req.ItemName,
				SortOrder:  req.SortOrder,
			})
			if err != nil {
				response.InternalError(c, "Failed to create template")
				return
			}
			response.Created(c, tmpl)
		})

		protected.DELETE("/clearance/templates/:id", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid ID")
				return
			}
			if err := a.Queries.DeleteClearanceTemplate(c.Request.Context(), store.DeleteClearanceTemplateParams{
				ID:        id,
				CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to delete template")
				return
			}
			response.OK(c, nil)
		})

		// CSV Export
		protected.GET("/export/payroll/:id/csv", auth.AdminOnly(), complianceHandler.ExportPayrollCSV)
		protected.GET("/export/payroll/:id/bank-file", auth.AdminOnly(), complianceHandler.ExportPayrollBankFile)
		protected.GET("/export/employees/csv", auth.AdminOnly(), complianceHandler.ExportEmployeesCSV)
		protected.GET("/export/attendance/csv", auth.AdminOnly(), importExportHandler.ExportAttendanceCSV)
		protected.GET("/export/leave-balances/csv", auth.AdminOnly(), importExportHandler.ExportLeaveBalancesCSV)

		// CSV Import
		protected.POST("/import/employees/csv", auth.AdminOnly(), importExportHandler.ImportEmployeesCSV)
		protected.POST("/import/employees/preview", auth.AdminOnly(), importExportHandler.PreviewImportCSV)

		// Salary Management
		protected.GET("/salary/structures", auth.AdminOnly(), complianceHandler.ListSalaryStructures)
		protected.POST("/salary/structures", auth.AdminOnly(), complianceHandler.CreateSalaryStructure)
		protected.GET("/salary/components", auth.AdminOnly(), complianceHandler.ListSalaryComponents)
		protected.POST("/salary/components", auth.AdminOnly(), complianceHandler.CreateSalaryComponent)

		// Approvals
		protected.GET("/approvals/pending", auth.ManagerOrAbove(), func(c *gin.Context) {
			userID := auth.GetUserID(c)
			companyID := auth.GetCompanyID(c)

			emp, err := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})
			if err != nil {
				response.OK(c, []any{})
				return
			}

			approvals, err := a.Queries.ListPendingApprovals(c.Request.Context(), emp.ID)
			if err != nil {
				response.InternalError(c, "Failed to list approvals")
				return
			}
			response.OK(c, approvals)
		})
		protected.POST("/approvals/:id/approve", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid approval ID")
				return
			}

			userID := auth.GetUserID(c)
			companyID := auth.GetCompanyID(c)

			emp, _ := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})

			var req struct {
				Comments *string `json:"comments"`
			}
			_ = c.ShouldBindJSON(&req)

			if err := a.Queries.ApproveWorkflow(c.Request.Context(), store.ApproveWorkflowParams{
				ID:         id,
				ApproverID: emp.ID,
				Comments:   req.Comments,
			}); err != nil {
				response.NotFound(c, "Approval not found or already processed")
				return
			}
			response.OK(c, gin.H{"message": "Approved"})
		})
		protected.POST("/approvals/:id/reject", auth.ManagerOrAbove(), func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				response.BadRequest(c, "Invalid approval ID")
				return
			}

			userID := auth.GetUserID(c)
			companyID := auth.GetCompanyID(c)

			emp, _ := a.Queries.GetEmployeeByUserID(c.Request.Context(), store.GetEmployeeByUserIDParams{
				UserID:    &userID,
				CompanyID: companyID,
			})

			var req struct {
				Comments *string `json:"comments"`
			}
			_ = c.ShouldBindJSON(&req)

			if err := a.Queries.RejectWorkflow(c.Request.Context(), store.RejectWorkflowParams{
				ID:         id,
				ApproverID: emp.ID,
				Comments:   req.Comments,
			}); err != nil {
				response.NotFound(c, "Approval not found or already processed")
				return
			}
			response.OK(c, gin.H{"message": "Rejected"})
		})

		// AI Assistant
		if aiHandler != nil {
			aiHandler.RegisterRoutes(protected)
		}

		// Holidays
		protected.GET("/holidays", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
			year, _ := strconv.ParseInt(yearStr, 10, 32)
			holidays, err := a.Queries.ListHolidays(c.Request.Context(), store.ListHolidaysParams{
				CompanyID: companyID,
				Year:      int32(year),
			})
			if err != nil {
				response.InternalError(c, "Failed to list holidays")
				return
			}
			response.OK(c, holidays)
		})
		protected.POST("/holidays", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Name        string `json:"name" binding:"required"`
				HolidayDate string `json:"holiday_date" binding:"required"`
				HolidayType string `json:"holiday_type" binding:"required"`
				IsNationwide *bool `json:"is_nationwide"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			date, err := time.Parse("2006-01-02", req.HolidayDate)
			if err != nil {
				response.BadRequest(c, "Invalid date format")
				return
			}
			isNationwide := true
			if req.IsNationwide != nil {
				isNationwide = *req.IsNationwide
			}
			holiday, err := a.Queries.CreateHoliday(c.Request.Context(), store.CreateHolidayParams{
				CompanyID:    companyID,
				Name:         req.Name,
				HolidayDate:  date,
				HolidayType:  req.HolidayType,
				Year:         int32(date.Year()),
				IsNationwide: isNationwide,
			})
			if err != nil {
				response.InternalError(c, "Failed to create holiday")
				return
			}
			response.Created(c, holiday)
		})
		protected.DELETE("/holidays/:id", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			if err := a.Queries.DeleteHoliday(c.Request.Context(), store.DeleteHolidayParams{
				ID: id, CompanyID: companyID,
			}); err != nil {
				response.NotFound(c, "Holiday not found")
				return
			}
			response.OK(c, gin.H{"message": "Deleted"})
		})

		// 13th Month Pay
		protected.GET("/payroll/13th-month", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			yearStr := c.DefaultQuery("year", fmt.Sprintf("%d", time.Now().Year()))
			year, _ := strconv.ParseInt(yearStr, 10, 32)
			records, err := a.Queries.List13thMonthPay(c.Request.Context(), store.List13thMonthPayParams{
				CompanyID: companyID,
				Year:      int32(year),
			})
			if err != nil {
				response.InternalError(c, "Failed to list 13th month pay")
				return
			}
			response.OK(c, records)
		})
		protected.POST("/payroll/13th-month/calculate", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Year int32 `json:"year" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Year is required")
				return
			}
			companyID := auth.GetCompanyID(c)
			calculator := payroll.NewCalculator(a.Queries, a.Pool, a.Logger)
			results, err := calculator.Calculate13thMonthPay(c.Request.Context(), companyID, req.Year)
			if err != nil {
				response.InternalError(c, fmt.Sprintf("Calculation failed: %s", err.Error()))
				return
			}
			response.OK(c, results)
		})

		// AI Smart Suggestions
		protected.GET("/suggestions", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			today := time.Now()

			type suggestion struct {
				Type        string      `json:"type"`
				Priority    string      `json:"priority"`
				Title       string      `json:"title"`
				Description string      `json:"description"`
				Count       int         `json:"count,omitempty"`
				Items       interface{} `json:"items,omitempty"`
			}

			var suggestions []suggestion

			// 1. Regularization due
			regDue, _ := a.Queries.ListEmployeesDueForRegularization(c.Request.Context(), store.ListEmployeesDueForRegularizationParams{
				CompanyID: companyID,
				Column2:   today,
			})
			if len(regDue) > 0 {
				suggestions = append(suggestions, suggestion{
					Type:        "regularization",
					Priority:    "high",
					Title:       fmt.Sprintf("%d employee(s) due for regularization", len(regDue)),
					Description: "These probationary employees are approaching or past their regularization date.",
					Count:       len(regDue),
					Items:       regDue,
				})
			}

			// 2. Expiring contracts
			expiring, _ := a.Queries.ListExpiringContracts(c.Request.Context(), store.ListExpiringContractsParams{
				CompanyID: companyID,
				Column2:   today,
			})
			if len(expiring) > 0 {
				suggestions = append(suggestions, suggestion{
					Type:        "contract_expiry",
					Priority:    "high",
					Title:       fmt.Sprintf("%d contract(s) expiring within 60 days", len(expiring)),
					Description: "Review these contractual employees for renewal or separation.",
					Count:       len(expiring),
					Items:       expiring,
				})
			}

			// 3. Upcoming birthdays
			birthdays, _ := a.Queries.ListUpcomingBirthdays(c.Request.Context(), store.ListUpcomingBirthdaysParams{
				CompanyID: companyID,
				Column2:   today,
			})
			if len(birthdays) > 0 {
				suggestions = append(suggestions, suggestion{
					Type:        "birthday",
					Priority:    "low",
					Title:       fmt.Sprintf("%d upcoming birthday(s) in the next 30 days", len(birthdays)),
					Description: "Send greetings to celebrate your team members.",
					Count:       len(birthdays),
					Items:       birthdays,
				})
			}

			// 4. Pending onboarding tasks
			pendingTasks, _ := a.Queries.ListPendingOnboardingTasks(c.Request.Context(), companyID)
			if len(pendingTasks) > 0 {
				suggestions = append(suggestions, suggestion{
					Type:        "onboarding",
					Priority:    "medium",
					Title:       fmt.Sprintf("%d pending onboarding task(s)", len(pendingTasks)),
					Description: "Complete these onboarding tasks to ensure smooth employee integration.",
					Count:       len(pendingTasks),
					Items:       pendingTasks,
				})
			}

			// 5. Employees without salary records
			noSalaryCount, _ := a.Queries.CountEmployeesWithNoSalary(c.Request.Context(), store.CountEmployeesWithNoSalaryParams{
				CompanyID:     companyID,
				EffectiveFrom: today,
			})
			if noSalaryCount > 0 {
				suggestions = append(suggestions, suggestion{
					Type:        "missing_salary",
					Priority:    "high",
					Title:       fmt.Sprintf("%d employee(s) have no salary record", noSalaryCount),
					Description: "Assign salary to these employees to include them in payroll.",
					Count:       int(noSalaryCount),
				})
			}

			response.OK(c, suggestions)
		})

		// User Management (admin)
		protected.GET("/users", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
			if page < 1 {
				page = 1
			}
			offset := (page - 1) * pageSize
			users, err := a.Queries.ListUsersByCompany(c.Request.Context(), store.ListUsersByCompanyParams{
				CompanyID: companyID,
				Limit:     int32(pageSize),
				Offset:    int32(offset),
			})
			if err != nil {
				response.InternalError(c, "Failed to list users")
				return
			}
			total, _ := a.Queries.CountUsersByCompany(c.Request.Context(), companyID)
			// Strip password hashes
			type safeUser struct {
				ID          int64       `json:"id"`
				Email       string      `json:"email"`
				FirstName   string      `json:"first_name"`
				LastName    string      `json:"last_name"`
				Role        string      `json:"role"`
				Status      string      `json:"status"`
				AvatarUrl   *string     `json:"avatar_url"`
				Locale      string      `json:"locale"`
				LastLoginAt interface{} `json:"last_login_at"`
				CreatedAt   interface{} `json:"created_at"`
			}
			safeUsers := make([]safeUser, len(users))
			for i, u := range users {
				safeUsers[i] = safeUser{
					ID: u.ID, Email: u.Email, FirstName: u.FirstName, LastName: u.LastName,
					Role: u.Role, Status: u.Status, AvatarUrl: u.AvatarUrl, Locale: u.Locale,
					LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt,
				}
			}
			response.OK(c, gin.H{"users": safeUsers, "total": total})
		})
		protected.PUT("/users/:id/role", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Role string `json:"role" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Role is required")
				return
			}
			allowed := map[string]bool{"admin": true, "manager": true, "employee": true}
			if !allowed[req.Role] {
				response.BadRequest(c, "Invalid role")
				return
			}
			if err := a.Queries.UpdateUserRole(c.Request.Context(), store.UpdateUserRoleParams{
				ID: id, Role: req.Role,
			}); err != nil {
				response.InternalError(c, "Failed to update role")
				return
			}
			response.OK(c, gin.H{"message": "Role updated"})
		})
		protected.PUT("/users/:id/status", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			var req struct {
				Status string `json:"status" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Status is required")
				return
			}
			allowed := map[string]bool{"active": true, "inactive": true, "suspended": true}
			if !allowed[req.Status] {
				response.BadRequest(c, "Invalid status")
				return
			}
			if err := a.Queries.UpdateUserStatus(c.Request.Context(), store.UpdateUserStatusParams{
				ID: id, Status: req.Status,
			}); err != nil {
				response.InternalError(c, "Failed to update status")
				return
			}
			response.OK(c, gin.H{"message": "Status updated"})
		})
		protected.POST("/users/:id/reset-password", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			var req struct {
				Password string `json:"password" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "Password is required")
				return
			}
			if len(req.Password) < 8 {
				response.BadRequest(c, "Password must be at least 8 characters")
				return
			}
			hash, err := auth.HashPassword(req.Password)
			if err != nil {
				response.InternalError(c, "Failed to hash password")
				return
			}
			if err := a.Queries.AdminResetPassword(c.Request.Context(), store.AdminResetPasswordParams{
				ID: id, PasswordHash: hash, CompanyID: companyID,
			}); err != nil {
				response.InternalError(c, "Failed to reset password")
				return
			}
			response.OK(c, gin.H{"message": "Password reset"})
		})

		// Announcements
		protected.GET("/announcements", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			announcements, err := a.Queries.ListAnnouncements(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list announcements")
				return
			}
			response.OK(c, announcements)
		})
		protected.GET("/announcements/all", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			announcements, err := a.Queries.ListAllAnnouncements(c.Request.Context(), companyID)
			if err != nil {
				response.InternalError(c, "Failed to list announcements")
				return
			}
			response.OK(c, announcements)
		})
		protected.POST("/announcements", auth.AdminOnly(), func(c *gin.Context) {
			var req struct {
				Title             string   `json:"title" binding:"required"`
				Content           string   `json:"content" binding:"required"`
				Priority          string   `json:"priority"`
				TargetRoles       []string `json:"target_roles"`
				TargetDepartments []int64  `json:"target_departments"`
				PublishedAt       *string  `json:"published_at"`
				ExpiresAt         *string  `json:"expires_at"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			companyID := auth.GetCompanyID(c)
			userID := auth.GetUserID(c)

			priority := req.Priority
			if priority == "" {
				priority = "normal"
			}

			var publishedAt interface{}
			if req.PublishedAt != nil {
				if t, err := time.Parse(time.RFC3339, *req.PublishedAt); err == nil {
					publishedAt = t
				}
			}

			var expiresAt pgtype.Timestamptz
			if req.ExpiresAt != nil {
				if t, err := time.Parse(time.RFC3339, *req.ExpiresAt); err == nil {
					expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}

			ann, err := a.Queries.CreateAnnouncement(c.Request.Context(), store.CreateAnnouncementParams{
				CompanyID:         companyID,
				Title:             req.Title,
				Content:           req.Content,
				Priority:          priority,
				TargetRoles:       req.TargetRoles,
				TargetDepartments: req.TargetDepartments,
				Column7:           publishedAt,
				ExpiresAt:         expiresAt,
				CreatedBy:         &userID,
			})
			if err != nil {
				a.Logger.Error("failed to create announcement", "error", err)
				response.InternalError(c, "Failed to create announcement")
				return
			}
			response.Created(c, ann)
		})
		protected.DELETE("/announcements/:id", auth.AdminOnly(), func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			companyID := auth.GetCompanyID(c)
			if err := a.Queries.DeleteAnnouncement(c.Request.Context(), store.DeleteAnnouncementParams{
				ID: id, CompanyID: companyID,
			}); err != nil {
				response.NotFound(c, "Announcement not found")
				return
			}
			response.OK(c, gin.H{"message": "Deleted"})
		})

		// Dashboard
		protected.GET("/dashboard/stats", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)

			totalEmployees, _ := a.Queries.CountEmployees(c.Request.Context(), store.CountEmployeesParams{
				CompanyID: companyID,
			})

			attendanceSummary, _ := a.Queries.GetTodayAttendanceSummary(c.Request.Context(), companyID)
			var presentToday int64
			for _, s := range attendanceSummary {
				presentToday += s.Count
			}

			pendingLeaves, _ := a.Queries.CountLeaveRequests(c.Request.Context(), store.CountLeaveRequestsParams{
				CompanyID: companyID,
				Column3:   "pending",
			})

			pendingOT, _ := a.Queries.CountOvertimeRequests(c.Request.Context(), store.CountOvertimeRequestsParams{
				CompanyID: companyID,
				Column3:   "pending",
			})

			response.OK(c, gin.H{
				"total_employees":  totalEmployees,
				"present_today":    presentToday,
				"pending_leaves":   pendingLeaves,
				"pending_overtime": pendingOT,
			})
		})
		protected.GET("/dashboard/attendance", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			summary, _ := a.Queries.GetTodayAttendanceSummary(c.Request.Context(), companyID)
			response.OK(c, summary)
		})
		protected.GET("/dashboard/department-distribution", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			rows, err := a.Pool.Query(c.Request.Context(), `
				SELECT d.name, COUNT(e.id) as count
				FROM employees e
				JOIN departments d ON d.id = e.department_id
				WHERE e.company_id = $1 AND e.status = 'active'
				GROUP BY d.name
				ORDER BY count DESC
			`, companyID)
			if err != nil {
				response.OK(c, []any{})
				return
			}
			defer rows.Close()
			var result []gin.H
			for rows.Next() {
				var name string
				var count int64
				if err := rows.Scan(&name, &count); err == nil {
					result = append(result, gin.H{"name": name, "count": count})
				}
			}
			response.OK(c, result)
		})
		protected.GET("/dashboard/payroll-trend", auth.AdminOnly(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			rows, err := a.Pool.Query(c.Request.Context(), `
				SELECT pc.name, pr.total_gross, pr.total_deductions, pr.total_net, pr.total_employees
				FROM payroll_runs pr
				JOIN payroll_cycles pc ON pc.id = pr.cycle_id
				WHERE pr.company_id = $1 AND pr.status = 'completed'
				ORDER BY pc.period_start DESC
				LIMIT 12
			`, companyID)
			if err != nil {
				response.OK(c, []any{})
				return
			}
			defer rows.Close()
			var result []gin.H
			for rows.Next() {
				var name string
				var gross, deductions, net pgtype.Numeric
				var employees int32
				if err := rows.Scan(&name, &gross, &deductions, &net, &employees); err == nil {
					result = append(result, gin.H{
						"name":       name,
						"gross":      numericToFloat(gross),
						"deductions": numericToFloat(deductions),
						"net":        numericToFloat(net),
						"employees":  employees,
					})
				}
			}
			// Reverse to chronological order
			for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
				result[i], result[j] = result[j], result[i]
			}
			response.OK(c, result)
		})
		protected.GET("/dashboard/leave-summary", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			rows, err := a.Pool.Query(c.Request.Context(), `
				SELECT lt.name, COUNT(lr.id) as count
				FROM leave_requests lr
				JOIN leave_types lt ON lt.id = lr.leave_type_id
				WHERE lr.company_id = $1
				  AND lr.status = 'approved'
				  AND EXTRACT(YEAR FROM lr.start_date) = EXTRACT(YEAR FROM NOW())
				GROUP BY lt.name
				ORDER BY count DESC
			`, companyID)
			if err != nil {
				response.OK(c, []any{})
				return
			}
			defer rows.Close()
			var result []gin.H
			for rows.Next() {
				var name string
				var count int64
				if err := rows.Scan(&name, &count); err == nil {
					result = append(result, gin.H{"name": name, "count": count})
				}
			}
			response.OK(c, result)
		})

		// Pending Action Items (admin/manager dashboard)
		protected.GET("/dashboard/action-items", auth.ManagerOrAbove(), func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			ctx := c.Request.Context()

			type ActionItem struct {
				Category string `json:"category"`
				Label    string `json:"label"`
				Count    int64  `json:"count"`
				Route    string `json:"route"`
			}
			var items []ActionItem

			// Pending leave requests
			pendingLeaves, _ := a.Queries.CountLeaveRequests(ctx, store.CountLeaveRequestsParams{
				CompanyID: companyID,
				Column3:   "pending",
			})
			if pendingLeaves > 0 {
				items = append(items, ActionItem{"approvals", "Pending Leave Requests", pendingLeaves, "/approvals"})
			}

			// Pending OT requests
			pendingOT, _ := a.Queries.CountOvertimeRequests(ctx, store.CountOvertimeRequestsParams{
				CompanyID: companyID,
				Column3:   "pending",
			})
			if pendingOT > 0 {
				items = append(items, ActionItem{"approvals", "Pending Overtime Requests", pendingOT, "/approvals"})
			}

			// Pending loans
			var pendingLoans int64
			_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM loans WHERE company_id = $1 AND status = 'pending'`, companyID).Scan(&pendingLoans)
			if pendingLoans > 0 {
				items = append(items, ActionItem{"approvals", "Pending Loan Applications", pendingLoans, "/loans"})
			}

			// Payroll cycles in draft/computed
			var draftPayroll int64
			_ = a.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payroll_cycles WHERE company_id = $1 AND status IN ('draft', 'computed')`, companyID).Scan(&draftPayroll)
			if draftPayroll > 0 {
				items = append(items, ActionItem{"payroll", "Payroll Cycles Pending", draftPayroll, "/payroll"})
			}

			// Employees without salary
			var noSalary int64
			_ = a.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM employees e
				WHERE e.company_id = $1 AND e.status = 'active'
				AND NOT EXISTS (SELECT 1 FROM employee_salaries es WHERE es.employee_id = e.id)
			`, companyID).Scan(&noSalary)
			if noSalary > 0 {
				items = append(items, ActionItem{"data_gaps", "Employees Without Salary", noSalary, "/employees"})
			}

			// Expiring documents (next 30 days)
			var expiringDocs int64
			_ = a.Pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM employee_documents
				WHERE company_id = $1 AND expiry_date IS NOT NULL
				AND expiry_date BETWEEN NOW()::date AND (NOW() + INTERVAL '30 days')::date
			`, companyID).Scan(&expiringDocs)
			if expiringDocs > 0 {
				items = append(items, ActionItem{"compliance", "Documents Expiring Soon", expiringDocs, "/employees"})
			}

			response.OK(c, items)
		})

		// Upcoming Birthdays & Anniversaries
		protected.GET("/dashboard/celebrations", func(c *gin.Context) {
			companyID := auth.GetCompanyID(c)
			daysAhead := 7

			// Birthdays: match month+day within next N days
			bRows, err := a.Pool.Query(c.Request.Context(), `
				SELECT id, employee_no, first_name, last_name, birth_date
				FROM employees
				WHERE company_id = $1 AND status = 'active' AND birth_date IS NOT NULL
				  AND (
				    (EXTRACT(MONTH FROM birth_date) = EXTRACT(MONTH FROM CURRENT_DATE)
				     AND EXTRACT(DAY FROM birth_date) BETWEEN EXTRACT(DAY FROM CURRENT_DATE) AND EXTRACT(DAY FROM CURRENT_DATE) + $2)
				    OR
				    (EXTRACT(MONTH FROM birth_date) = EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
				     AND EXTRACT(DAY FROM birth_date) <= EXTRACT(DAY FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
				     AND EXTRACT(MONTH FROM CURRENT_DATE) != EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL))
				  )
				ORDER BY EXTRACT(MONTH FROM birth_date), EXTRACT(DAY FROM birth_date)
				LIMIT 20
			`, companyID, daysAhead)
			var birthdays []gin.H
			if err == nil {
				defer bRows.Close()
				for bRows.Next() {
					var id int64
					var empNo, firstName, lastName string
					var birthDate pgtype.Date
					if err := bRows.Scan(&id, &empNo, &firstName, &lastName, &birthDate); err == nil {
						bd := ""
						if birthDate.Valid {
							bd = birthDate.Time.Format("01-02")
						}
						birthdays = append(birthdays, gin.H{
							"id": id, "employee_no": empNo,
							"name": firstName + " " + lastName,
							"date": bd,
						})
					}
				}
			}

			// Work Anniversaries: hired on same month+day, at least 1 year ago
			aRows, err := a.Pool.Query(c.Request.Context(), `
				SELECT id, employee_no, first_name, last_name, hire_date,
				       EXTRACT(YEAR FROM AGE(CURRENT_DATE, hire_date))::int as years
				FROM employees
				WHERE company_id = $1 AND status = 'active'
				  AND EXTRACT(YEAR FROM AGE(CURRENT_DATE, hire_date)) >= 1
				  AND (
				    (EXTRACT(MONTH FROM hire_date) = EXTRACT(MONTH FROM CURRENT_DATE)
				     AND EXTRACT(DAY FROM hire_date) BETWEEN EXTRACT(DAY FROM CURRENT_DATE) AND EXTRACT(DAY FROM CURRENT_DATE) + $2)
				    OR
				    (EXTRACT(MONTH FROM hire_date) = EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
				     AND EXTRACT(DAY FROM hire_date) <= EXTRACT(DAY FROM CURRENT_DATE + ($2 || ' days')::INTERVAL)
				     AND EXTRACT(MONTH FROM CURRENT_DATE) != EXTRACT(MONTH FROM CURRENT_DATE + ($2 || ' days')::INTERVAL))
				  )
				ORDER BY EXTRACT(MONTH FROM hire_date), EXTRACT(DAY FROM hire_date)
				LIMIT 20
			`, companyID, daysAhead)
			var anniversaries []gin.H
			if err == nil {
				defer aRows.Close()
				for aRows.Next() {
					var id int64
					var empNo, firstName, lastName string
					var hireDate time.Time
					var years int
					if err := aRows.Scan(&id, &empNo, &firstName, &lastName, &hireDate, &years); err == nil {
						anniversaries = append(anniversaries, gin.H{
							"id": id, "employee_no": empNo,
							"name":  firstName + " " + lastName,
							"date":  hireDate.Format("01-02"),
							"years": years,
						})
					}
				}
			}

			response.OK(c, gin.H{
				"birthdays":     birthdays,
				"anniversaries": anniversaries,
			})
		})
	}
}

func (a *App) Run() error {
	addr := fmt.Sprintf("%s:%s", a.Cfg.Server.Host, a.Cfg.Server.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.Logger.Info("server starting", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		a.Logger.Info("received signal, shutting down", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	a.Pool.Close()
	a.Redis.Close()
	a.Logger.Info("server stopped")
	return nil
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return f.Float64
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

func generateCOEPDF(comp store.Company, emp store.GetEmployeeForCOERow) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(25, 25, 25)

	companyName := comp.Name
	if comp.LegalName != nil && *comp.LegalName != "" {
		companyName = *comp.LegalName
	}

	// Company header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(160, 10, companyName, "", 1, "C", false, 0, "")

	if comp.Address != nil {
		pdf.SetFont("Arial", "", 10)
		addr := *comp.Address
		if comp.City != nil {
			addr += ", " + *comp.City
		}
		if comp.Province != nil {
			addr += ", " + *comp.Province
		}
		pdf.CellFormat(160, 5, addr, "", 1, "C", false, 0, "")
	}
	if comp.Tin != nil {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(160, 5, "TIN: "+*comp.Tin, "", 1, "C", false, 0, "")
	}

	pdf.Ln(15)

	// Title
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "CERTIFICATE OF EMPLOYMENT", "", 1, "C", false, 0, "")
	pdf.Ln(10)

	// Date
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(5)

	// Salutation
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 7, "TO WHOM IT MAY CONCERN:", "", 1, "L", false, 0, "")
	pdf.Ln(5)

	// Employee name
	fullName := strings.TrimSpace(emp.FirstName + " " + emp.LastName)
	if emp.MiddleName != nil && *emp.MiddleName != "" {
		fullName = strings.TrimSpace(emp.FirstName + " " + *emp.MiddleName + " " + emp.LastName)
	}

	// Body paragraph
	pdf.SetFont("Arial", "", 11)
	body := fmt.Sprintf(
		"This is to certify that %s has been employed with %s since %s",
		fullName, companyName, emp.HireDate.Format("January 02, 2006"),
	)
	if emp.Status == "active" {
		body += " up to the present."
	} else {
		body += "."
	}
	pdf.MultiCell(160, 6, body, "", "L", false)
	pdf.Ln(3)

	// Position / Department
	if emp.PositionTitle != "" || emp.DepartmentName != "" {
		var detail string
		if emp.PositionTitle != "" && emp.DepartmentName != "" {
			detail = fmt.Sprintf(
				"During the period of employment, %s held the position of %s under the %s department.",
				fullName, emp.PositionTitle, emp.DepartmentName,
			)
		} else if emp.PositionTitle != "" {
			detail = fmt.Sprintf("During the period of employment, %s held the position of %s.", fullName, emp.PositionTitle)
		} else {
			detail = fmt.Sprintf("During the period of employment, %s was assigned to the %s department.", fullName, emp.DepartmentName)
		}
		pdf.MultiCell(160, 6, detail, "", "L", false)
		pdf.Ln(3)
	}

	// Employment type
	empType := emp.EmploymentType
	if len(empType) > 0 {
		empType = strings.ToUpper(empType[:1]) + empType[1:]
	}
	pdf.MultiCell(160, 6, fmt.Sprintf("Employment type: %s.", empType), "", "L", false)
	pdf.Ln(3)

	// Closing
	pdf.MultiCell(160, 6, "This certificate is issued upon request for whatever legal purpose it may serve.", "", "L", false)

	pdf.Ln(25)

	// Signature
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Authorized Signatory", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")

	// Footer
	pdf.Ln(15)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(160, 5, "This is a system-generated document.", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func generateLetterPDF(comp store.Company, emp store.GetEmployeeForCOERow, letterType, subject, body, violations, deadline string, salary float64) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(25, 25, 25)

	companyName := comp.Name
	if comp.LegalName != nil && *comp.LegalName != "" {
		companyName = *comp.LegalName
	}

	fullName := strings.TrimSpace(emp.FirstName + " " + emp.LastName)
	if emp.MiddleName != nil && *emp.MiddleName != "" {
		fullName = strings.TrimSpace(emp.FirstName + " " + *emp.MiddleName + " " + emp.LastName)
	}

	// Company header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(160, 10, companyName, "", 1, "C", false, 0, "")
	if comp.Address != nil {
		pdf.SetFont("Arial", "", 10)
		addr := *comp.Address
		if comp.City != nil {
			addr += ", " + *comp.City
		}
		if comp.Province != nil {
			addr += ", " + *comp.Province
		}
		pdf.CellFormat(160, 5, addr, "", 1, "C", false, 0, "")
	}
	if comp.Tin != nil {
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(160, 5, "TIN: "+*comp.Tin, "", 1, "C", false, 0, "")
	}
	pdf.Ln(10)

	switch letterType {
	case "nte":
		generateNTE(pdf, companyName, fullName, emp, subject, violations, deadline)
	case "coec":
		generateCOEC(pdf, companyName, fullName, emp, salary)
	case "clearance":
		generateClearance(pdf, companyName, fullName, emp)
	case "memo":
		generateMemo(pdf, companyName, fullName, emp, subject, body)
	default:
		return nil, fmt.Errorf("unsupported letter type: %s", letterType)
	}

	// Footer
	pdf.Ln(15)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(160, 5, "This is a system-generated document.", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func generateNTE(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, subject, violations, deadline string) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "NOTICE TO EXPLAIN", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Date
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// To
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "To:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, fullName, "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "Dept:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, emp.DepartmentName, "", 1, "L", false, 0, "")
	pdf.Ln(5)

	if subject != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(160, 7, "RE: "+subject, "", 1, "L", false, 0, "")
		pdf.Ln(3)
	}

	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(160, 6, "Dear "+emp.FirstName+",", "", "L", false)
	pdf.Ln(3)

	intro := "This is to formally notify you that you are being required to explain the following matter(s) which may constitute a violation of company policy:"
	pdf.MultiCell(160, 6, intro, "", "L", false)
	pdf.Ln(3)

	if violations != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(160, 6, "Alleged Violation(s):", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(160, 6, violations, "", "L", false)
		pdf.Ln(3)
	}

	pdf.SetFont("Arial", "", 11)
	responseText := "You are hereby given the opportunity to explain your side in writing."
	if deadline != "" {
		responseText += " Please submit your written explanation on or before " + deadline + "."
	} else {
		responseText += " Please submit your written explanation within five (5) calendar days from receipt of this notice."
	}
	pdf.MultiCell(160, 6, responseText, "", "L", false)
	pdf.Ln(3)

	pdf.MultiCell(160, 6, "Failure to respond within the given period shall be construed as a waiver of your right to be heard, and management will proceed to resolve the matter based on available evidence.", "", "L", false)

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Human Resources Department", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")
}

func generateCOEC(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, salary float64) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "CERTIFICATE OF EMPLOYMENT", "", 1, "C", false, 0, "")
	pdf.CellFormat(160, 8, "WITH COMPENSATION", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(3)

	pdf.CellFormat(160, 7, "TO WHOM IT MAY CONCERN:", "", 1, "L", false, 0, "")
	pdf.Ln(5)

	body := fmt.Sprintf(
		"This is to certify that %s has been employed with %s since %s",
		fullName, companyName, emp.HireDate.Format("January 02, 2006"),
	)
	if emp.Status == "active" {
		body += " up to the present."
	} else {
		body += "."
	}
	pdf.MultiCell(160, 6, body, "", "L", false)
	pdf.Ln(3)

	if emp.PositionTitle != "" {
		pdf.MultiCell(160, 6, fmt.Sprintf("Position: %s", emp.PositionTitle), "", "L", false)
	}
	if emp.DepartmentName != "" {
		pdf.MultiCell(160, 6, fmt.Sprintf("Department: %s", emp.DepartmentName), "", "L", false)
	}
	pdf.Ln(3)

	if salary > 0 {
		salaryStr := fmt.Sprintf("PHP %.2f", salary)
		pdf.MultiCell(160, 6, fmt.Sprintf("Current monthly compensation: %s", salaryStr), "", "L", false)
		pdf.Ln(3)
	}

	pdf.MultiCell(160, 6, "This certificate is issued upon request for whatever legal purpose it may serve.", "", "L", false)

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Authorized Signatory", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")
}

func generateClearance(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "EMPLOYEE CLEARANCE CERTIFICATE", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "R", false, 0, "")
	pdf.Ln(3)

	pdf.MultiCell(160, 6, fmt.Sprintf(
		"This is to certify that %s (Employee No. %s) has been cleared of all accountabilities and obligations with %s.",
		fullName, emp.EmployeeNo, companyName,
	), "", "L", false)
	pdf.Ln(3)

	// Clearance checklist
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 8, "Clearance Checklist:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)

	items := []string{
		"Company Property (ID, equipment, keys)",
		"Outstanding Cash Advances / Loans",
		"Pending Work Assignments",
		"IT Accounts & Access Deactivation",
		"Final Pay Computation",
	}
	for _, item := range items {
		pdf.CellFormat(8, 6, "", "1", 0, "C", false, 0, "")
		pdf.CellFormat(152, 6, "  "+item, "", 1, "L", false, 0, "")
	}

	pdf.Ln(10)

	// Signatures
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(80, 6, "________________________________", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "________________________________", "", 1, "C", false, 0, "")
	pdf.CellFormat(80, 6, "Department Head", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "HR Department", "", 1, "C", false, 0, "")

	pdf.Ln(10)
	pdf.CellFormat(80, 6, "________________________________", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "________________________________", "", 1, "C", false, 0, "")
	pdf.CellFormat(80, 6, "Finance / Accounting", "", 0, "C", false, 0, "")
	pdf.CellFormat(80, 6, "IT Department", "", 1, "C", false, 0, "")
}

func generateMemo(pdf *fpdf.Fpdf, companyName, fullName string, emp store.GetEmployeeForCOERow, subject, body string) {
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(160, 10, "MEMORANDUM", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(160, 6, "Date: "+time.Now().Format("January 02, 2006"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "To:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, fullName+" ("+emp.EmployeeNo+")", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(20, 6, "From:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(140, 6, "Human Resources Department", "", 1, "L", false, 0, "")

	if subject != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(20, 6, "RE:", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(140, 6, subject, "", 1, "L", false, 0, "")
	}

	pdf.Ln(5)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(25, pdf.GetY(), 185, pdf.GetY())
	pdf.Ln(5)

	if body != "" {
		pdf.SetFont("Arial", "", 11)
		pdf.MultiCell(160, 6, body, "", "L", false)
	}

	pdf.Ln(20)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, "Human Resources Department", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 5, companyName, "", 1, "L", false, 0, "")

	pdf.Ln(15)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(160, 6, "Acknowledged by:", "", 1, "L", false, 0, "")
	pdf.Ln(10)
	pdf.CellFormat(160, 6, "________________________________", "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 6, fullName, "", 1, "L", false, 0, "")
	pdf.CellFormat(160, 5, "Date: _______________", "", 1, "L", false, 0, "")
}

func generateDOLERegisterPDF(comp store.Company, emps []store.ListEmployeesForDOLERegisterRow) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "Legal", "")
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()

	companyName := comp.Name

	// Title
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, companyName, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 7, "DOLE Employee Register", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("As of %s", time.Now().Format("January 2, 2006")), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Column widths (Legal landscape ~356mm usable)
	colWidths := []float64{15, 25, 40, 15, 20, 18, 18, 20, 30, 30, 22, 22, 22, 22}
	headers := []string{"No.", "Emp No", "Name", "Sex", "Birth Date", "Civil St.", "Nationality", "Hire Date", "Department", "Position", "TIN", "SSS", "PhilHealth", "Pag-IBIG"}

	// Header row
	pdf.SetFont("Arial", "B", 7)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Data rows
	pdf.SetFont("Arial", "", 6.5)
	pdf.SetFillColor(245, 245, 245)
	for idx, emp := range emps {
		fill := idx%2 == 1

		// Build full name
		name := emp.LastName + ", " + emp.FirstName
		if emp.MiddleName != nil && *emp.MiddleName != "" {
			name += " " + string((*emp.MiddleName)[0]) + "."
		}

		gender := ""
		if emp.Gender != nil {
			g := strings.ToUpper(*emp.Gender)
			if len(g) > 0 {
				gender = string(g[0])
			}
		}

		birthDate := ""
		if emp.BirthDate.Valid {
			birthDate = emp.BirthDate.Time.Format("01/02/2006")
		}

		civilStatus := ""
		if emp.CivilStatus != nil {
			civilStatus = *emp.CivilStatus
		}

		nationality := ""
		if emp.Nationality != nil {
			nationality = *emp.Nationality
		}

		row := []string{
			fmt.Sprintf("%d", idx+1),
			emp.EmployeeNo,
			name,
			gender,
			birthDate,
			civilStatus,
			nationality,
			emp.HireDate.Format("01/02/2006"),
			emp.DepartmentName,
			emp.PositionTitle,
			emp.Tin,
			emp.SssNo,
			emp.PhilhealthNo,
			emp.PagibigNo,
		}

		for i, val := range row {
			pdf.CellFormat(colWidths[i], 5, val, "1", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
	}

	// Summary
	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Employees: %d", len(emps)), "", 1, "L", false, 0, "")

	pdf.Ln(15)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(100, 6, "Prepared by: ________________________________", "", 0, "L", false, 0, "")
	pdf.CellFormat(100, 6, "Noted by: ________________________________", "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
