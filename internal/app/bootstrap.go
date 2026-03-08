package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tonypk/aigonhr/internal/ai"
	"github.com/tonypk/aigonhr/internal/ai/provider"
	"github.com/tonypk/aigonhr/internal/analytics"
	"github.com/tonypk/aigonhr/internal/announcement"
	"github.com/tonypk/aigonhr/internal/approval"
	"github.com/tonypk/aigonhr/internal/attendance"
	"github.com/tonypk/aigonhr/internal/audit"
	"github.com/tonypk/aigonhr/internal/auth"
	"github.com/tonypk/aigonhr/internal/benefits"
	"github.com/tonypk/aigonhr/internal/clearance"
	"github.com/tonypk/aigonhr/internal/company"
	"github.com/tonypk/aigonhr/internal/compliance"
	"github.com/tonypk/aigonhr/internal/config"
	"github.com/tonypk/aigonhr/internal/dashboard"
	"github.com/tonypk/aigonhr/internal/disciplinary"
	"github.com/tonypk/aigonhr/internal/docfile"
	"github.com/tonypk/aigonhr/internal/email"
	"github.com/tonypk/aigonhr/internal/employee"
	"github.com/tonypk/aigonhr/internal/expense"
	"github.com/tonypk/aigonhr/internal/finalpay"
	"github.com/tonypk/aigonhr/internal/grievance"
	"github.com/tonypk/aigonhr/internal/holiday"
	"github.com/tonypk/aigonhr/internal/importexport"
	"github.com/tonypk/aigonhr/internal/knowledge"
	"github.com/tonypk/aigonhr/internal/leave"
	"github.com/tonypk/aigonhr/internal/loan"
	"github.com/tonypk/aigonhr/internal/middleware"
	"github.com/tonypk/aigonhr/internal/milestone"
	"github.com/tonypk/aigonhr/internal/notification"
	"github.com/tonypk/aigonhr/internal/onboarding"
	"github.com/tonypk/aigonhr/internal/overtime"
	"github.com/tonypk/aigonhr/internal/payroll"
	"github.com/tonypk/aigonhr/internal/performance"
	"github.com/tonypk/aigonhr/internal/policy"
	"github.com/tonypk/aigonhr/internal/ratelimit"
	"github.com/tonypk/aigonhr/internal/report"
	"github.com/tonypk/aigonhr/internal/selfservice"
	"github.com/tonypk/aigonhr/internal/store"
	"github.com/tonypk/aigonhr/internal/training"
)

type App struct {
	Cfg     *config.Config
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Queries *store.Queries
	Router  *gin.Engine
	Logger  *slog.Logger
	Email   *email.Sender
	Limiter *ratelimit.Limiter
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
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.SecurityHeaders())

	// CORS — origins from environment variable
	origins := strings.Split(cfg.CORS.AllowOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.MaxMultipartMemory = 10 << 20 // 10MB

	// Rate limiter
	limiter := ratelimit.New(rdb, ratelimit.Config{
		Enabled:     cfg.RateLimit.Enabled,
		LoginRate:   cfg.RateLimit.LoginRate,
		LoginWindow: cfg.RateLimit.LoginWindow,
		APIRate:     cfg.RateLimit.APIRate,
		APIWindow:   cfg.RateLimit.APIWindow,
	})

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
		Limiter: limiter,
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

	// Handlers
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
	benefitsHandler := benefits.NewHandler(a.Queries, a.Pool, a.Logger)
	docfileHandler := docfile.NewHandler(a.Queries, a.Pool, a.Logger)
	policyHandler := policy.NewHandler(a.Queries, a.Pool, a.Logger)
	disciplinaryHandler := disciplinary.NewHandler(a.Queries, a.Pool, a.Logger)
	grievanceHandler := grievance.NewHandler(a.Queries, a.Pool, a.Logger)
	expenseHandler := expense.NewHandler(a.Queries, a.Pool, a.Logger)
	trainingHandler := training.NewHandler(a.Queries, a.Pool, a.Logger)
	clearanceHandler := clearance.NewHandler(a.Queries, a.Pool, a.Logger)
	milestoneHandler := milestone.NewHandler(a.Queries, a.Pool, a.Logger)
	reportHandler := report.NewHandler(a.Queries, a.Pool, a.Logger)
	finalpayHandler := finalpay.NewHandler(a.Queries, a.Pool, a.Logger)
	approvalHandler := approval.NewHandler(a.Queries, a.Pool, a.Logger)
	holidayHandler := holiday.NewHandler(a.Queries, a.Pool, a.Logger)
	announcementHandler := announcement.NewHandler(a.Queries, a.Pool, a.Logger)
	dashboardHandler := dashboard.NewHandler(a.Queries, a.Pool, a.Logger)

	// AI service (optional)
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

	protected := api.Group("")
	protected.Use(auth.JWTMiddleware(jwtSvc))
	protected.Use(a.Limiter.APIMiddleware())

	// Register all routes (login rate limiter applied inside auth routes)
	authHandler.RegisterRoutes(api, protected, a.Limiter.LoginMiddleware())
	companyHandler.RegisterRoutes(protected)
	employeeHandler.RegisterRoutes(protected)
	attendanceHandler.RegisterRoutes(protected)
	leaveHandler.RegisterRoutes(protected)
	overtimeHandler.RegisterRoutes(protected)
	payrollHandler.RegisterRoutes(protected)
	complianceHandler.RegisterRoutes(protected)
	onboardingHandler.RegisterRoutes(protected)
	performanceHandler.RegisterRoutes(protected)
	selfServiceHandler.RegisterRoutes(protected)
	analyticsHandler.RegisterRoutes(protected)
	loanHandler.RegisterRoutes(protected)
	notificationHandler.RegisterRoutes(protected)
	knowledgeHandler.RegisterRoutes(protected)
	auditHandler.RegisterRoutes(protected)
	importExportHandler.RegisterRoutes(protected)
	benefitsHandler.RegisterRoutes(protected)
	docfileHandler.RegisterRoutes(protected)
	policyHandler.RegisterRoutes(protected)
	disciplinaryHandler.RegisterRoutes(protected)
	grievanceHandler.RegisterRoutes(protected)
	expenseHandler.RegisterRoutes(protected)
	trainingHandler.RegisterRoutes(protected)
	clearanceHandler.RegisterRoutes(protected)
	milestoneHandler.RegisterRoutes(protected)
	reportHandler.RegisterRoutes(protected)
	finalpayHandler.RegisterRoutes(protected)
	approvalHandler.RegisterRoutes(protected)
	holidayHandler.RegisterRoutes(protected)
	announcementHandler.RegisterRoutes(protected)
	dashboardHandler.RegisterRoutes(protected)

	if aiHandler != nil {
		aiHandler.RegisterRoutes(protected)
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
