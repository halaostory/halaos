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

	"github.com/halaostory/halaos/internal/ai"
	"github.com/halaostory/halaos/internal/ai/agent"
	"github.com/halaostory/halaos/internal/ai/byok"
	aicontext "github.com/halaostory/halaos/internal/ai/context"
	"github.com/halaostory/halaos/internal/ai/draft"
	"github.com/halaostory/halaos/internal/ai/provider"
	"github.com/halaostory/halaos/internal/billing"
	"github.com/halaostory/halaos/internal/bot"
	bottelegram "github.com/halaostory/halaos/internal/bot/telegram"
	"github.com/halaostory/halaos/internal/integration"
	"github.com/halaostory/halaos/internal/integration/connector"
	connectorslack "github.com/halaostory/halaos/internal/integration/connector/slack"
	connectorgoogle "github.com/halaostory/halaos/internal/integration/connector/google"
	connectorgithub "github.com/halaostory/halaos/internal/integration/connector/github"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/analytics"
	"github.com/halaostory/halaos/internal/orgintel"
	"github.com/halaostory/halaos/internal/announcement"
	"github.com/halaostory/halaos/internal/approval"
	"github.com/halaostory/halaos/internal/attendance"
	"github.com/halaostory/halaos/internal/breaks"
	"github.com/halaostory/halaos/internal/audit"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/benefits"
	"github.com/halaostory/halaos/internal/clearance"
	"github.com/halaostory/halaos/internal/company"
	"github.com/halaostory/halaos/internal/compliance"
	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/dashboard"
	"github.com/halaostory/halaos/internal/disciplinary"
	"github.com/halaostory/halaos/internal/docfile"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/employee"
	"github.com/halaostory/halaos/internal/expense"
	"github.com/halaostory/halaos/internal/finalpay"
	"github.com/halaostory/halaos/internal/grievance"
	"github.com/halaostory/halaos/internal/holiday"
	"github.com/halaostory/halaos/internal/importexport"
	"github.com/halaostory/halaos/internal/knowledge"
	"github.com/halaostory/halaos/internal/leave"
	"github.com/halaostory/halaos/internal/loan"
	"github.com/halaostory/halaos/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/halaostory/halaos/internal/milestone"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/onboarding"
	onboardingChecklist "github.com/halaostory/halaos/internal/onboarding_checklist"
	"github.com/halaostory/halaos/internal/overtime"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/performance"
	"github.com/halaostory/halaos/internal/policy"
	"github.com/halaostory/halaos/internal/ratelimit"
	"github.com/halaostory/halaos/internal/report"
	"github.com/halaostory/halaos/internal/selfservice"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/training"
	"github.com/halaostory/halaos/internal/pulse"
	"github.com/halaostory/halaos/internal/hrrequest"
	"github.com/halaostory/halaos/internal/recognition"
	"github.com/halaostory/halaos/internal/nps"
	"github.com/halaostory/halaos/internal/referral"
	"github.com/halaostory/halaos/internal/virtualoffice"
	"github.com/halaostory/halaos/internal/workflow"
)

type App struct {
	Cfg        *config.Config
	Pool       *pgxpool.Pool
	Redis      *redis.Client
	Queries    *store.Queries
	Router     *gin.Engine
	Logger     *slog.Logger
	Email      *email.Sender
	Resend     *email.Service
	Limiter    *ratelimit.Limiter
	BotManager *bot.BotManager
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
	router.Use(middleware.PrometheusMetrics())

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

	// Email sender (optional, SMTP for HR notifications)
	emailSender := email.NewSender(cfg.SMTP, logger)
	if emailSender != nil {
		logger.Info("email notifications enabled (SMTP)")
	} else {
		logger.Info("email notifications disabled (SMTP not configured)")
	}

	// Resend email service (for registration verification)
	resendSvc := email.NewService(cfg.Resend.APIKey, cfg.Resend.From, cfg.Resend.BaseURL, logger)
	if resendSvc.IsEnabled() {
		logger.Info("resend email service enabled")
	} else {
		logger.Info("resend email service disabled (RESEND_API_KEY not set)")
	}

	app := &App{
		Cfg:     cfg,
		Pool:    pool,
		Redis:   rdb,
		Queries: queries,
		Router:  router,
		Logger:  logger,
		Email:   emailSender,
		Resend:  resendSvc,
		Limiter: limiter,
	}

	app.setupRoutes()
	return app, nil
}

func (a *App) setupRoutes() {
	// Health check with detailed status
	a.Router.GET("/health", func(c *gin.Context) {
		ctx := c.Request.Context()
		health := gin.H{"status": "healthy"}

		// Database
		if err := a.Pool.Ping(ctx); err != nil {
			health["status"] = "unhealthy"
			health["db"] = gin.H{"status": "down", "error": err.Error()}
			c.JSON(http.StatusServiceUnavailable, health)
			return
		}
		stat := a.Pool.Stat()
		health["db"] = gin.H{
			"status":       "up",
			"total_conns":  stat.TotalConns(),
			"idle_conns":   stat.IdleConns(),
			"active_conns": stat.AcquiredConns(),
			"max_conns":    stat.MaxConns(),
		}

		// Redis
		if err := a.Redis.Ping(ctx).Err(); err != nil {
			health["redis"] = gin.H{"status": "down", "error": err.Error()}
		} else {
			health["redis"] = gin.H{"status": "up"}
		}

		c.JSON(http.StatusOK, health)
	})

	// Prometheus metrics endpoint
	a.Router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	jwtSvc := auth.NewJWTService(a.Cfg.JWT.Secret, a.Cfg.JWT.Expiry, a.Cfg.JWT.RefreshExpiry)

	// SSO service (needed by both auth handler and accounting handler)
	acctSSO := integration.NewSSOService(a.Cfg.Integration.JWTSecret)

	// Handlers
	authHandler := auth.NewHandler(a.Queries, a.Pool, jwtSvc, a.Resend, a.Logger, a.Redis, acctSSO)
	companyHandler := company.NewHandler(a.Queries, a.Pool, a.Logger)
	employeeHandler := employee.NewHandler(a.Queries, a.Pool, a.Logger)
	attendanceHandler := attendance.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)
	breaksHandler := breaks.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)
	leaveHandler := leave.NewHandler(a.Queries, a.Pool, a.Logger, a.Email)
	overtimeHandler := overtime.NewHandler(a.Queries, a.Pool, a.Logger)
	payrollHandler := payroll.NewHandler(a.Queries, a.Pool, a.Logger)

	// Wire accounting integration (outbox + dispatcher)
	acctEmitter := integration.NewAccountingEmitter(a.Queries, a.Logger)
	payrollHandler.SetAccountingEmitter(acctEmitter)
	employeeHandler.SetAccountingEmitter(acctEmitter)
	acctDispatcher := integration.NewAccountingDispatcher(a.Queries, a.Logger)
	go acctDispatcher.Run(context.Background())

	// Wire brain integration (outbox + dispatcher)
	brainDispatcher := integration.NewBrainDispatcher(a.Queries, a.Logger)
	go brainDispatcher.Run(context.Background())

	acctHandler := integration.NewAccountingHandler(a.Queries, acctSSO, a.Logger)
	complianceHandler := compliance.NewHandler(a.Queries, a.Pool, a.Logger)
	onboardingHandler := onboarding.NewHandler(a.Queries, a.Pool, a.Logger)
	onboardingChecklistHandler := onboardingChecklist.NewHandler(a.Queries, a.Pool, a.Logger)
	performanceHandler := performance.NewHandler(a.Queries, a.Pool, a.Logger)
	selfServiceHandler := selfservice.NewHandler(a.Queries, a.Pool, a.Logger)
	analyticsHandler := analytics.NewHandler(a.Queries, a.Pool, a.Logger)
	loanHandler := loan.NewHandler(a.Queries, a.Pool, a.Logger, a.Email)
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
	workflowHandler := workflow.NewHandler(a.Queries, a.Pool, a.Logger)
	pulseHandler := pulse.NewHandler(a.Queries, a.Pool, a.Logger)
	recognitionHandler := recognition.NewHandler(a.Queries, a.Pool, a.Logger)
	hrrequestHandler := hrrequest.NewHandler(a.Queries, a.Pool, a.Logger)
	virtualOfficeHandler := virtualoffice.NewHandler(a.Queries, a.Pool, a.Logger, a.Redis)

	// Billing service
	billingSvc := billing.NewService(a.Queries, a.Logger)
	billingHandler := billing.NewHandler(billingSvc, a.Queries, a.Logger)

	// AI service (optional — supports Anthropic or OpenAI)
	var aiHandler *ai.Handler
	var executor *agent.Executor
	var draftSvc *draft.Service
	var aiProvider provider.Provider
	if a.Cfg.AI.Enabled {
		switch {
		case a.Cfg.AI.MiniMaxKey != "":
			aiProvider = provider.NewMiniMax(a.Cfg.AI.MiniMaxKey, "")
			a.Logger.Info("AI provider: MiniMax")
		case a.Cfg.AI.AnthropicKey != "":
			aiProvider = provider.NewAnthropic(a.Cfg.AI.AnthropicKey, "")
			a.Logger.Info("AI provider: Anthropic")
		case a.Cfg.AI.OpenAIKey != "":
			aiProvider = provider.NewOpenAI(a.Cfg.AI.OpenAIKey, "")
			a.Logger.Info("AI provider: OpenAI")
		}
		if aiProvider != nil {
			aiService := ai.NewService(aiProvider, a.Queries, a.Pool, a.Logger)
			toolRegistry := ai.NewToolRegistry(a.Queries, a.Pool)
			agentRegistry := agent.NewRegistry(a.Queries, a.Logger)
			draftSvc = draft.NewService(a.Queries, a.Logger)
			contextBld := aicontext.NewBuilder(a.Queries)
			executor = agent.NewExecutor(aiProvider, toolRegistry, billingSvc, agentRegistry, a.Queries, a.Logger, draftSvc, contextBld)

			// BYOK resolver: allows per-company/user API keys
			if a.Cfg.Integration.EncryptionKey != "" {
				if byokEncryptor, err := crypto.NewCredentialEncryptor(a.Cfg.Integration.EncryptionKey); err == nil {
					byokResolver := byok.NewResolver(a.Queries, byokEncryptor, aiProvider,
						a.Cfg.AI.AnthropicKey, a.Cfg.AI.OpenAIKey, a.Cfg.AI.GeminiKey, a.Logger)
					executor.SetResolver(func(ctx context.Context, companyID, userID int64) provider.Provider {
						rp := byokResolver.Resolve(ctx, companyID, userID)
						return rp.Provider
					})
					a.Logger.Info("BYOK key resolver enabled")
				}
			}

			draftHandler := draft.NewHandler(draftSvc, toolRegistry)
			aiHandler = ai.NewHandler(aiService, executor, agentRegistry, toolRegistry, a.Queries, draftHandler)
			a.Logger.Info("AI assistant enabled", "provider", aiProvider.Name(), "agents", len(agentRegistry.List(context.Background())))
		} else {
			a.Logger.Info("AI assistant disabled (no API key configured)")
		}
	} else {
		a.Logger.Info("AI assistant disabled (AI_ENABLED=false)")
	}

	// Wire AI provider into approval handler for smart context
	if aiProvider != nil {
		approvalHandler.SetAIProvider(aiProvider)
	}

	api := a.Router.Group("/api/v1")

	// Public endpoints (no auth required)
	api.POST("/contact", a.Limiter.LoginMiddleware(), a.handleContactForm)

	protected := api.Group("")
	protected.Use(auth.APIKeyMiddleware(a.Queries))
	protected.Use(auth.JWTMiddleware(jwtSvc))
	protected.Use(a.Limiter.APIMiddleware())

	// Register all routes (login rate limiter applied inside auth routes)
	authHandler.RegisterRoutes(api, protected, a.Limiter.LoginMiddleware())
	companyHandler.RegisterRoutes(protected)
	employeeHandler.RegisterRoutes(protected)
	attendanceHandler.RegisterRoutes(protected)
	breaksHandler.RegisterRoutes(protected)
	leaveHandler.RegisterRoutes(protected)
	overtimeHandler.RegisterRoutes(protected)
	payrollHandler.RegisterRoutes(protected)
	complianceHandler.RegisterRoutes(protected)
	onboardingHandler.RegisterRoutes(protected)
	onboardingChecklistHandler.RegisterRoutes(protected)
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
	acctHandler.RegisterRoutes(protected)
	workflowHandler.RegisterRoutes(protected)
	pulseHandler.RegisterRoutes(protected)
	recognitionHandler.RegisterRoutes(protected)
	hrrequestHandler.RegisterRoutes(protected)
	virtualOfficeHandler.RegisterRoutes(protected)

	billingHandler.RegisterRoutes(protected)

	referralHandler := referral.NewHandler(a.Queries, a.Logger)
	referralHandler.RegisterRoutes(protected)

	npsHandler := nps.NewHandler(a.Queries, a.Logger)
	npsHandler.RegisterRoutes(protected)

	// Org Intelligence
	var briefGen *orgintel.BriefingGenerator
	if aiProvider != nil {
		briefGen = orgintel.NewBriefingGenerator(a.Queries, aiProvider, a.Logger)
	}
	orgIntelHandler := orgintel.NewHandler(a.Queries, a.Pool, a.Logger, briefGen)
	orgIntelHandler.RegisterRoutes(protected)

	if aiHandler != nil {
		aiHandler.RegisterRoutes(protected)
	}

	// Integration Hub
	if a.Cfg.Integration.EncryptionKey != "" {
		encryptor, err := crypto.NewCredentialEncryptor(a.Cfg.Integration.EncryptionKey)
		if err != nil {
			a.Logger.Error("failed to init integration encryptor", "error", err)
		} else {
			connRegistry := connector.NewRegistry()
			connRegistry.Register("slack", func(creds connector.Credentials) (connector.Connector, error) {
				return connectorslack.New(creds)
			})
			connRegistry.Register("google", func(creds connector.Credentials) (connector.Connector, error) {
				return connectorgoogle.New(creds)
			})
			connRegistry.Register("github", func(creds connector.Credentials) (connector.Connector, error) {
				return connectorgithub.New(creds)
			})

			integrationSvc := integration.NewService(a.Queries, connRegistry, encryptor, a.Logger)
			provisioningSvc := integration.NewProvisioningService(a.Queries, a.Logger)
			integrationHandler := integration.NewHandler(integrationSvc, provisioningSvc, a.Queries)
			integrationHandler.RegisterRoutes(protected)

			// Start provisioning worker
			provWorker := integration.NewProvisioningWorker(a.Queries, integrationSvc, a.Logger)
			go provWorker.Run(context.Background(), 30*time.Second)

			a.Logger.Info("integration hub enabled", "providers", connRegistry.Providers())

			// BYOK key management (shares same encryptor)
			byokHandler := byok.NewHandler(a.Queries, encryptor)
			byokHandler.RegisterRoutes(protected)
			a.Logger.Info("BYOK key management enabled")
		}
	} else {
		a.Logger.Info("integration hub disabled (INTEGRATION_ENCRYPTION_KEY not set)")
	}

	// Bot link management (always available for link code generation)
	botLinker := bot.NewLinker(a.Queries, a.Logger)

	// Telegram bot (optional) — managed by BotManager with hot reload
	if executor != nil {
		botSessionMgr := bot.NewSessionManager(a.Queries, a.Logger)
		botRateLimiter := bot.NewRateLimiter(a.Redis, 20, 1*time.Minute)
		dispatcher := bot.NewDispatcher(botLinker, botSessionMgr, executor, draftSvc, a.Queries, botRateLimiter, a.Logger)

		botFactory := func(ctx context.Context, token string) {
			tgBot := bottelegram.New(token, dispatcher, a.Logger)
			tgBot.Run(ctx)
		}
		a.BotManager = bot.NewBotManager(botFactory, a.Queries, &a.Cfg.Bot, a.Logger)
		a.BotManager.StartAll()
	} else {
		a.Logger.Info("telegram bot disabled (AI provider not available)")
	}

	botLinkHandler := bot.NewLinkHandler(botLinker, a.Queries, a.Logger, a.BotManager, &a.Cfg.Bot)
	botLinkHandler.RegisterRoutes(protected)
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

	if a.BotManager != nil {
		a.BotManager.StopAll()
	}
	a.Pool.Close()
	a.Redis.Close()
	a.Logger.Info("server stopped")
	return nil
}

func (a *App) handleContactForm(c *gin.Context) {
	var req struct {
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
		Company   string `json:"company"`
		Subject   string `json:"subject" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"message": err.Error()}})
		return
	}

	if a.Resend != nil && a.Resend.IsEnabled() {
		if err := a.Resend.SendContactForm(
			"hello@halaos.com",
			req.FirstName, req.LastName, req.Email,
			req.Company, req.Subject, req.Message,
		); err != nil {
			a.Logger.Error("contact form email failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"message": "Failed to send message"}})
			return
		}
	} else {
		a.Logger.Info("contact form received (email not configured)",
			"from", req.Email, "subject", req.Subject)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "Message sent successfully"}})
}
