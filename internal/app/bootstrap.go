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
	"github.com/halaostory/halaos/internal/alerts"
	"github.com/halaostory/halaos/internal/analytics"
	"github.com/halaostory/halaos/internal/announcement"
	"github.com/halaostory/halaos/internal/approval"
	"github.com/halaostory/halaos/internal/attendance"
	"github.com/halaostory/halaos/internal/audit"
	"github.com/halaostory/halaos/internal/auth"
	"github.com/halaostory/halaos/internal/benefits"
	"github.com/halaostory/halaos/internal/billing"
	"github.com/halaostory/halaos/internal/bot"
	bottelegram "github.com/halaostory/halaos/internal/bot/telegram"
	"github.com/halaostory/halaos/internal/breaks"
	"github.com/halaostory/halaos/internal/clearance"
	"github.com/halaostory/halaos/internal/company"
	"github.com/halaostory/halaos/internal/compliance"
	"github.com/halaostory/halaos/internal/config"
	"github.com/halaostory/halaos/internal/contractors"
	"github.com/halaostory/halaos/internal/dashboard"
	"github.com/halaostory/halaos/internal/devices"
	"github.com/halaostory/halaos/internal/disciplinary"
	"github.com/halaostory/halaos/internal/docfile"
	"github.com/halaostory/halaos/internal/edition"
	"github.com/halaostory/halaos/internal/email"
	"github.com/halaostory/halaos/internal/employee"
	"github.com/halaostory/halaos/internal/expense"
	"github.com/halaostory/halaos/internal/finalpay"
	"github.com/halaostory/halaos/internal/glsync"
	"github.com/halaostory/halaos/internal/govfiling"
	"github.com/halaostory/halaos/internal/grievance"
	"github.com/halaostory/halaos/internal/holiday"
	"github.com/halaostory/halaos/internal/home"
	"github.com/halaostory/halaos/internal/hrrequest"
	"github.com/halaostory/halaos/internal/importexport"
	"github.com/halaostory/halaos/internal/integration"
	"github.com/halaostory/halaos/internal/integration/connector"
	connectorgithub "github.com/halaostory/halaos/internal/integration/connector/github"
	connectorgoogle "github.com/halaostory/halaos/internal/integration/connector/google"
	connectorokta "github.com/halaostory/halaos/internal/integration/connector/okta"
	connectorslack "github.com/halaostory/halaos/internal/integration/connector/slack"
	"github.com/halaostory/halaos/internal/integration/crypto"
	"github.com/halaostory/halaos/internal/knowledge"
	"github.com/halaostory/halaos/internal/leave"
	"github.com/halaostory/halaos/internal/leavepolicy"
	"github.com/halaostory/halaos/internal/lifecycle"
	"github.com/halaostory/halaos/internal/loan"
	"github.com/halaostory/halaos/internal/middleware"
	"github.com/halaostory/halaos/internal/milestone"
	"github.com/halaostory/halaos/internal/notification"
	"github.com/halaostory/halaos/internal/onboarding"
	onboardingChecklist "github.com/halaostory/halaos/internal/onboarding_checklist"
	"github.com/halaostory/halaos/internal/orgintel"
	"github.com/halaostory/halaos/internal/overtime"
	"github.com/halaostory/halaos/internal/payouts"
	"github.com/halaostory/halaos/internal/payroll"
	"github.com/halaostory/halaos/internal/performance"
	"github.com/halaostory/halaos/internal/policy"
	"github.com/halaostory/halaos/internal/pulse"
	"github.com/halaostory/halaos/internal/ratelimit"
	"github.com/halaostory/halaos/internal/recognition"
	"github.com/halaostory/halaos/internal/report"
	"github.com/halaostory/halaos/internal/routines"
	"github.com/halaostory/halaos/internal/selfonboard"
	"github.com/halaostory/halaos/internal/selfservice"
	"github.com/halaostory/halaos/internal/store"
	"github.com/halaostory/halaos/internal/training"
	"github.com/halaostory/halaos/internal/workflow"
	"github.com/halaostory/halaos/internal/workplace"
	"github.com/halaostory/halaos/pkg/response"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	Cfg        *config.Config
	Pool       *pgxpool.Pool
	Redis      *redis.Client
	Queries    *store.Queries
	Router     *gin.Engine
	Logger     *slog.Logger
	Email      *email.Sender
	Edition    edition.Config
	sink       *gin.RouterGroup // routes of modules not shipped in this edition go here (never served)
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

	// Serve only the uploads that are meant to be public (shown in <img> tags).
	// Employee documents and 201 files live under ./uploads too but are only
	// downloadable through authenticated, company-scoped handlers.
	registerPublicUploads(router, "./uploads")

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
		Edition: edition.Load(),
		sink:    &gin.New().RouterGroup,
	}
	logger.Info("product edition", "edition", app.Edition.Edition, "product", app.Edition.ProductName, "modules", app.Edition.Modules)

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
	lifecycleHandler := lifecycle.NewHandler(a.Queries, a.Pool, a.Logger)
	lifecycleHandler.Service().SetAccountingEmitter(acctEmitter)
	acctDispatcher := integration.NewAccountingDispatcher(a.Queries, a.Logger)
	go acctDispatcher.Run(context.Background())

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
	// Public product configuration (edition, brand, modules, markets) for the web app.
	api.GET("/edition", func(c *gin.Context) { response.OK(c, a.Edition) })
	authHandler.SetEdition(a.Edition)
	authHandler.SetOIDC(auth.OIDCConfig{AppURL: a.Cfg.Resend.BaseURL, RedirectURL: os.Getenv("OIDC_REDIRECT_URL")})
	authHandler.RegisterRoutes(api, protected, a.Limiter.LoginMiddleware())
	companyHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	employeeHandler.RegisterRoutes(a.mod(edition.ModPeople, protected))
	lifecycleHandler.RegisterRoutes(a.mod(edition.ModLifecycle, protected))
	attendanceHandler.RegisterRoutes(a.mod(edition.ModAttendance, protected))
	breaksHandler.RegisterRoutes(a.mod(edition.ModAttendance, protected))
	leaveHandler.RegisterRoutes(a.mod(edition.ModTimeOff, protected))
	overtimeHandler.RegisterRoutes(a.mod(edition.ModTimeOff, protected))
	payrollHandler.RegisterRoutes(a.mod(edition.ModPayroll, protected))
	complianceHandler.RegisterRoutes(a.mod(edition.ModCompliance, protected))
	onboardingHandler.RegisterRoutes(a.mod(edition.ModLifecycle, protected))
	onboardingChecklistHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	performanceHandler.RegisterRoutes(a.mod(edition.ModTalent, protected))
	selfServiceHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	analyticsHandler.RegisterRoutes(a.mod(edition.ModIntelligence, protected))
	loanHandler.RegisterRoutes(a.mod(edition.ModPayroll, protected))
	notificationHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	knowledgeHandler.RegisterRoutes(a.mod(edition.ModIntelligence, protected))
	auditHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	importExportHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	benefitsHandler.RegisterRoutes(a.mod(edition.ModBenefits, protected))
	docfileHandler.RegisterRoutes(a.mod(edition.ModPeople, protected))
	policyHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	disciplinaryHandler.RegisterRoutes(a.mod(edition.ModTalent, protected))
	grievanceHandler.RegisterRoutes(a.mod(edition.ModTalent, protected))
	expenseHandler.RegisterRoutes(a.mod(edition.ModExpenses, protected))
	trainingHandler.RegisterRoutes(a.mod(edition.ModTalent, protected))
	clearanceHandler.RegisterRoutes(a.mod(edition.ModLifecycle, protected))
	milestoneHandler.RegisterRoutes(a.mod(edition.ModLifecycle, protected))
	reportHandler.RegisterRoutes(a.mod(edition.ModPayroll, protected))
	finalpayHandler.RegisterRoutes(a.mod(edition.ModPayroll, protected))
	approvalHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	holidayHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	announcementHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	dashboardHandler.RegisterRoutes(a.mod(edition.ModCore, protected))
	acctHandler.RegisterRoutes(a.mod(edition.ModPayroll, protected))
	workflowHandler.RegisterRoutes(a.mod(edition.ModIntelligence, protected))
	pulseHandler.RegisterRoutes(a.mod(edition.ModEngagement, protected))
	recognitionHandler.RegisterRoutes(a.mod(edition.ModEngagement, protected))
	hrrequestHandler.RegisterRoutes(a.mod(edition.ModCore, protected))

	billingHandler.RegisterRoutes(a.mod(edition.ModCore, protected))

	// Government filing agent (prepare → review → approve → submit)
	govSvc := govfiling.NewService(a.Queries, a.Pool, a.Logger, govfiling.ServiceConfigFromEnv(a.Cfg.Integration.EncryptionKey, aiProvider, a.Email, a.Logger))
	govHandler := govfiling.NewHandler(govSvc, a.Queries, a.Logger)
	govHandler.RegisterRoutes(a.mod(edition.ModCompliance, protected))

	// Agent routines (New hire, Offboarding, Pre-payroll check, Filing autopilot)
	routineEngine := &routines.Engine{
		Service:    routines.NewService(a.Queries, a.Logger),
		Gov:        govSvc,
		Calculator: payroll.NewCalculator(a.Queries, a.Pool, a.Logger),
		Enabled:    a.Edition.Enabled,
	}
	if aiProvider != nil {
		routineEngine.AI = aiProvider
	}
	govSvc.SetPolicy(routineEngine.Policy)
	lifecycleHandler.Service().SetRoutines(routineEngine)

	// Workplaces (work locations; policies can target them)
	workplace.NewHandler(a.Queries).RegisterRoutes(a.mod(edition.ModPeople, protected))

	// Direct journal push to Xero / QuickBooks Online
	glsync.NewFromEnv(a.Queries, a.Cfg.Integration.EncryptionKey, a.Cfg.Resend.BaseURL, a.Logger).
		RegisterRoutes(a.mod(edition.ModPayroll, protected), a.mod(edition.ModPayroll, api))

	// Net pay disbursement through a licensed payout partner (Xendit)
	payouts.NewFromEnv(a.Queries, a.Pool, a.Logger).RegisterRoutes(a.mod(edition.ModPayroll, protected), a.mod(edition.ModPayroll, api))

	// Rule-based time-off policies (by department, employment type, gender, tenure)
	leavepolicy.NewService(a.Queries, a.Logger).RegisterRoutes(a.mod(edition.ModTimeOff, protected))

	// Slack alerts (agent needs you, notices, filings due, payroll ready)
	alertSvc := alerts.NewServiceFromKey(a.Queries, a.Cfg.Integration.EncryptionKey, a.Logger, a.Cfg.Resend.BaseURL)
	alertSvc.RegisterRoutes(a.mod(edition.ModCore, protected))
	routineEngine.SetNotifier(alertSvc)
	govSvc.SetNotifier(alertSvc)
	routines.NewHandler(routineEngine).RegisterRoutes(a.mod(edition.ModAI, protected))

	// Home dashboard (to-dos, upcoming payroll/filings, agency registrations, agent activity)
	home.NewHandler(a.Queries, a.Logger, a.Edition.Enabled).RegisterRoutes(a.mod(edition.ModCore, protected))

	// New-hire self-onboarding portal + e-signatures (public token routes are rate limited)
	selfOnboardHandler := selfonboard.NewHandler(selfonboard.NewService(a.Queries, a.Logger, a.Email, aiProvider, a.Cfg.Resend.BaseURL), a.Queries, a.Logger)
	selfOnboardHandler.RegisterRoutes(a.mod(edition.ModESign, protected))
	selfOnboardHandler.RegisterPublic(a.mod(edition.ModESign, api), a.Limiter.APIMiddleware(), a.Limiter.LoginMiddleware())
	lifecycleHandler.Service().SetOnboarding(selfOnboardHandler.Service())
	if aiProvider != nil {
		lifecycleHandler.SetAI(aiProvider)
	}

	// Device inventory + MDM actions
	deviceSvc := devices.NewServiceFromKey(a.Queries, a.Logger, a.Cfg.Integration.EncryptionKey)
	devices.NewHandler(deviceSvc, a.Queries).RegisterRoutes(a.mod(edition.ModIT, protected))
	lifecycleHandler.Service().SetDevices(deviceSvc)

	// Contractors & freelancer payments (EWT, BIR 2307)
	contractorsHandler := contractors.NewHandler(a.Queries, a.Logger, a.Edition.ProductName)
	contractorsHandler.SetWise(contractors.WiseFromEnv())
	contractorsHandler.RegisterRoutes(a.mod(edition.ModContractors, protected))

	// Org Intelligence
	var briefGen *orgintel.BriefingGenerator
	if aiProvider != nil {
		briefGen = orgintel.NewBriefingGenerator(a.Queries, aiProvider, a.Logger)
	}
	orgIntelHandler := orgintel.NewHandler(a.Queries, a.Pool, a.Logger, briefGen)
	orgIntelHandler.RegisterRoutes(a.mod(edition.ModIntelligence, protected))

	a.Edition.AIAssistant = aiHandler != nil && a.Edition.Enabled(edition.ModAI)
	if aiHandler != nil {
		aiHandler.RegisterRoutes(a.mod(edition.ModAI, protected))
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
			connRegistry.Register("okta", func(creds connector.Credentials) (connector.Connector, error) {
				return connectorokta.New(creds)
			})

			integrationSvc := integration.NewService(a.Queries, connRegistry, encryptor, a.Logger)
			provisioningSvc := integration.NewProvisioningService(a.Queries, a.Logger)
			integrationHandler := integration.NewHandler(integrationSvc, provisioningSvc, a.Queries)
			integrationHandler.RegisterRoutes(a.mod(edition.ModIT, protected))

			// Start provisioning worker
			if a.Edition.Enabled(edition.ModIT) {
				provWorker := integration.NewProvisioningWorker(a.Queries, integrationSvc, a.Logger)
				go provWorker.Run(context.Background(), 30*time.Second)
			}

			a.Logger.Info("integration hub enabled", "providers", connRegistry.Providers())

			// BYOK key management (shares same encryptor)
			byokHandler := byok.NewHandler(a.Queries, encryptor)
			byokHandler.RegisterRoutes(a.mod(edition.ModAI, protected))
			a.Logger.Info("BYOK key management enabled")
		}
	} else {
		a.Logger.Info("integration hub disabled (INTEGRATION_ENCRYPTION_KEY not set)")
	}

	// Bot link management (always available for link code generation)
	botLinker := bot.NewLinker(a.Queries, a.Logger)

	// Telegram bot (optional) — managed by BotManager with hot reload
	if executor != nil && a.Edition.Enabled(edition.ModAI) {
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
	botLinkHandler.RegisterRoutes(a.mod(edition.ModAI, protected))
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

// mod returns g when the module ships in this edition; otherwise a router
// group on a detached engine, so the module's routes are never served.
func (a *App) mod(module string, g *gin.RouterGroup) *gin.RouterGroup {
	if a.Edition.Enabled(module) {
		return g
	}
	return a.sink
}
