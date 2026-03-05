package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/attendance"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/company"
	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/employee"
	"github.com/tonypk/aigonhr/internal/leave"
	"github.com/tonypk/aigonhr/internal/overtime"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/store"
)

type App struct {
	Cfg     *config.Config
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Queries *store.Queries
	Router  *gin.Engine
	Logger  *slog.Logger
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
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.MaxMultipartMemory = 10 << 20 // 10MB

	app := &App{
		Cfg:     cfg,
		Pool:    pool,
		Redis:   rdb,
		Queries: queries,
		Router:  router,
		Logger:  logger,
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
		// User
		protected.GET("/me", authHandler.Me)

		// Company
		protected.GET("/company", companyHandler.GetCompany)
		protected.PUT("/company", auth.AdminOnly(), companyHandler.UpdateCompany)

		// Departments
		protected.GET("/departments", companyHandler.ListDepartments)
		protected.POST("/departments", auth.AdminOnly(), companyHandler.CreateDepartment)
		protected.PUT("/departments/:id", auth.AdminOnly(), companyHandler.UpdateDepartment)

		// Positions
		protected.GET("/positions", companyHandler.ListPositions)
		protected.POST("/positions", auth.AdminOnly(), companyHandler.CreatePosition)
		protected.PUT("/positions/:id", auth.AdminOnly(), companyHandler.UpdatePosition)

		// Employees
		protected.GET("/employees", auth.ManagerOrAbove(), employeeHandler.ListEmployees)
		protected.POST("/employees", auth.AdminOnly(), employeeHandler.CreateEmployee)
		protected.GET("/employees/:id", employeeHandler.GetEmployee)
		protected.PUT("/employees/:id", auth.AdminOnly(), employeeHandler.UpdateEmployee)
		protected.GET("/employees/:id/profile", employeeHandler.GetProfile)
		protected.PUT("/employees/:id/profile", auth.AdminOnly(), employeeHandler.UpdateProfile)
		protected.GET("/employees/:id/documents", employeeHandler.ListDocuments)
		protected.POST("/employees/:id/documents", auth.AdminOnly(), employeeHandler.UploadDocument)

		// Attendance
		protected.POST("/attendance/clock-in", attendanceHandler.ClockIn)
		protected.POST("/attendance/clock-out", attendanceHandler.ClockOut)
		protected.GET("/attendance/records", attendanceHandler.ListRecords)
		protected.GET("/attendance/summary", attendanceHandler.GetSummary)
		protected.GET("/attendance/shifts", attendanceHandler.ListShifts)
		protected.POST("/attendance/shifts", auth.AdminOnly(), attendanceHandler.CreateShift)
		protected.PUT("/attendance/shifts/:id", auth.AdminOnly(), attendanceHandler.UpdateShift)
		protected.POST("/attendance/schedules", auth.AdminOnly(), attendanceHandler.AssignSchedule)

		// Leave
		protected.GET("/leaves/types", leaveHandler.ListTypes)
		protected.POST("/leaves/types", auth.AdminOnly(), leaveHandler.CreateType)
		protected.GET("/leaves/balances", leaveHandler.GetBalances)
		protected.POST("/leaves/requests", leaveHandler.CreateRequest)
		protected.GET("/leaves/requests", leaveHandler.ListRequests)
		protected.POST("/leaves/requests/:id/approve", auth.ManagerOrAbove(), leaveHandler.ApproveRequest)
		protected.POST("/leaves/requests/:id/reject", auth.ManagerOrAbove(), leaveHandler.RejectRequest)
		protected.POST("/leaves/requests/:id/cancel", leaveHandler.CancelRequest)

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
		protected.POST("/payroll/cycles/:id/approve", auth.AdminOnly(), payrollHandler.ApproveCycle)
		protected.GET("/payslips", payrollHandler.ListPayslips)
		protected.GET("/payslips/:id", payrollHandler.GetPayslip)

		// Dashboard
		protected.GET("/dashboard/stats", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "dashboard stats placeholder"}})
		})
	}
}

func (a *App) Run() error {
	addr := fmt.Sprintf("%s:%s", a.Cfg.Server.Host, a.Cfg.Server.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
