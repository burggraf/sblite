// internal/server/server.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/markb/sblite/internal/admin"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/functions"
	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/oauth"
	"github.com/markb/sblite/internal/realtime"
	"github.com/markb/sblite/internal/rest"
	"github.com/markb/sblite/internal/rls"
	"github.com/markb/sblite/internal/rpc"
	"github.com/markb/sblite/internal/schema"
	"github.com/markb/sblite/internal/storage"
	"github.com/markb/sblite/internal/vector"
	"golang.org/x/crypto/acme/autocert"
)

type Server struct {
	db               *db.DB
	router           *chi.Mux
	authService      *auth.Service
	rlsService       *rls.Service
	rlsEnforcer      *rls.Enforcer
	restHandler      *rest.Handler
	mailConfig       *mail.Config
	mailer           mail.Mailer
	catchMailer      *mail.CatchMailer
	emailService     *mail.EmailService
	adminHandler     *admin.Handler
	schema           *schema.Schema
	dashboardHandler *dashboard.Handler

	// OAuth fields
	oauthRegistry       *oauth.Registry
	oauthStateStore     *oauth.StateStore
	allowedRedirectURLs []string
	baseURL             string

	// Storage fields
	storageService *storage.Service
	storageHandler *storage.Handler

	// Functions fields
	jwtSecret        string
	functionsService *functions.Service
	functionsHandler *functions.Handler
	functionsEnabled bool

	// RPC fields
	rpcStore       *rpc.Store
	rpcExecutor    *rpc.Executor
	rpcHandler     *rpc.Handler
	rpcInterceptor *rpc.Interceptor

	// Vector search
	vectorSearcher *vector.Searcher

	// Realtime fields
	realtimeService *realtime.Service

	// HTTP server for graceful shutdown
	httpServer *http.Server

	// HTTPS fields
	httpsServer  *http.Server
	httpRedirect *http.Server
	autocertMgr  *autocert.Manager

	// Dashboard store for auth settings
	dashboardStore *dashboard.Store
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	JWTSecret     string
	MailConfig    *mail.Config
	MigrationsDir string
	StoragePath   string         // Path for local file storage (deprecated, use StorageConfig)
	StorageConfig *storage.Config // Full storage configuration
}

func New(database *db.DB, jwtSecret string, mailConfig *mail.Config, migrationsDir string, storagePath string) *Server {
	if storagePath == "" {
		storagePath = "./storage"
	}
	return NewWithConfig(database, ServerConfig{
		JWTSecret:     jwtSecret,
		MailConfig:    mailConfig,
		MigrationsDir: migrationsDir,
		StoragePath:   storagePath,
	})
}

func NewWithConfig(database *db.DB, cfg ServerConfig) *Server {
	rlsService := rls.NewService(database)
	rlsEnforcer := rls.NewEnforcer(rlsService)

	// Use default config if nil
	if cfg.MailConfig == nil {
		cfg.MailConfig = mail.DefaultConfig()
	}

	// Initialize schema first (needed by REST handler)
	schemaInstance := schema.New(database.DB)

	s := &Server{
		db:              database,
		router:          chi.NewRouter(),
		authService:     auth.NewService(database, cfg.JWTSecret),
		rlsService:      rlsService,
		rlsEnforcer:     rlsEnforcer,
		restHandler:     rest.NewHandler(database, rlsEnforcer, schemaInstance),
		mailConfig:      cfg.MailConfig,
		schema:          schemaInstance,
		jwtSecret:       cfg.JWTSecret,
		oauthRegistry:   oauth.NewRegistry(),
		oauthStateStore: oauth.NewStateStore(database.DB),
	}

	// Initialize admin handler (uses schema)
	s.adminHandler = admin.NewHandler(s.db, s.schema)

	// Initialize RPC components
	s.rpcStore = rpc.NewStore(database.DB)
	s.rpcExecutor = rpc.NewExecutor(database.DB, s.rpcStore)
	s.rpcHandler = rpc.NewHandler(s.rpcExecutor, s.rpcStore)
	s.rpcInterceptor = rpc.NewInterceptor(s.rpcStore)

	// Initialize vector searcher and wire to RPC handler
	s.vectorSearcher = vector.NewSearcher(database.DB, rlsEnforcer, schemaInstance)
	s.rpcHandler.SetVectorSearcher(s.vectorSearcher)

	// Initialize dashboard handler
	s.dashboardHandler = dashboard.NewHandler(database.DB, cfg.MigrationsDir)
	s.dashboardHandler.SetJWTSecret(cfg.JWTSecret)
	s.dashboardStore = s.dashboardHandler.GetStore()
	// Set RPC interceptor and executor on dashboard handler
	s.dashboardHandler.SetRPCInterceptor(s.rpcInterceptor)
	s.dashboardHandler.SetRPCExecutor(s.rpcExecutor)

	// Apply persisted settings from dashboard (e.g., SiteURL, mail mode)
	// This must happen before initMail() so dashboard settings are loaded
	s.applyPersistedSettings()

	// Initialize mail services (after persisted settings are loaded)
	s.initMail()

	// Set catch mailer on dashboard handler for mail viewer (if in catch mode)
	s.dashboardHandler.SetCatchMailer(s.catchMailer)

	// Set up callback to update mail config when SiteURL changes via dashboard
	s.dashboardHandler.SetOnSiteURLChange(func(siteURL string) {
		s.mailConfig.SiteURL = siteURL
	})

	// Initialize storage service
	var storageCfg storage.Config
	if cfg.StorageConfig != nil {
		storageCfg = *cfg.StorageConfig
	} else {
		// Backward compatibility: use StoragePath for local storage
		storagePath := cfg.StoragePath
		if storagePath == "" {
			storagePath = "./storage"
		}
		storageCfg = storage.Config{
			Backend:   "local",
			LocalPath: storagePath,
		}
	}
	storageService, err := storage.NewService(database.DB, storageCfg)
	if err == nil {
		s.storageService = storageService
		s.storageHandler = storage.NewHandler(storageService)
		// Pass RLS service and enforcer to storage handler for RLS policy enforcement
		s.storageHandler.SetRLSEnforcer(rlsService, rlsEnforcer)
		// Pass JWT secret for signed URL generation
		s.storageHandler.SetJWTSecret(cfg.JWTSecret)
		// Enable TUS resumable uploads
		uploadsDir := storageCfg.LocalPath
		if uploadsDir == "" {
			uploadsDir = "./storage"
		}
		uploadsDir += "/.uploads"
		s.storageHandler.EnableTUS(uploadsDir)
		// Set storage service on dashboard handler for management UI
		s.dashboardHandler.SetStorageService(storageService)
		// Register storage reload callback for hot-reload via dashboard
		s.dashboardHandler.SetStorageReloadFunc(func(cfg *dashboard.StorageConfig) error {
			storageCfg := storage.Config{
				Backend:          cfg.Backend,
				LocalPath:        cfg.LocalPath,
				S3Endpoint:       cfg.S3Endpoint,
				S3Region:         cfg.S3Region,
				S3Bucket:         cfg.S3Bucket,
				S3AccessKey:      cfg.S3AccessKey,
				S3SecretKey:      cfg.S3SecretKey,
				S3ForcePathStyle: cfg.S3ForcePathStyle,
			}
			return storageService.Reconfigure(storageCfg)
		})
	} else {
		log.Warn("failed to initialize storage service", "error", err.Error())
	}

	// Register mail reload callback
	s.dashboardHandler.SetMailReloadFunc(func(cfg *dashboard.MailConfig) error {
		return s.ReloadMail(cfg)
	})

	s.setupRoutes()
	return s
}

// SetDashboardConfig sets the dashboard server configuration for display in settings.
func (s *Server) SetDashboardConfig(cfg *dashboard.ServerConfig) {
	s.dashboardHandler.SetServerConfig(cfg)
}

// applyPersistedSettings applies settings persisted in the dashboard to the server config.
// This is called after the dashboard handler is initialized.
func (s *Server) applyPersistedSettings() {
	// Apply persisted SiteURL if configured
	if siteURL := s.dashboardHandler.GetSiteURL(); siteURL != "" {
		s.mailConfig.SiteURL = siteURL
	}

	// Apply persisted mail settings if configured
	if mode := s.dashboardHandler.GetMailMode(); mode != "" {
		s.mailConfig.Mode = mode
	}
	if from := s.dashboardHandler.GetMailFrom(); from != "" {
		s.mailConfig.From = from
	}
	host, port, user, pass := s.dashboardHandler.GetSMTPConfig()
	if host != "" {
		s.mailConfig.SMTPHost = host
	}
	if port != 587 || s.mailConfig.SMTPPort == 0 {
		s.mailConfig.SMTPPort = port
	}
	if user != "" {
		s.mailConfig.SMTPUser = user
	}
	if pass != "" {
		s.mailConfig.SMTPPass = pass
	}
}

// initMail initializes the mail services based on configuration.
func (s *Server) initMail() {
	switch s.mailConfig.Mode {
	case mail.ModeCatch:
		s.catchMailer = mail.NewCatchMailer(s.db)
		s.mailer = s.catchMailer
	case mail.ModeSMTP:
		s.catchMailer = nil // Clear catch mailer when not in catch mode
		smtpConfig := mail.SMTPConfig{
			Host: s.mailConfig.SMTPHost,
			Port: s.mailConfig.SMTPPort,
			User: s.mailConfig.SMTPUser,
			Pass: s.mailConfig.SMTPPass,
		}
		s.mailer = mail.NewSMTPMailer(smtpConfig)
	default:
		// Default to log mode
		s.catchMailer = nil // Clear catch mailer when not in catch mode
		s.mailer = mail.NewLogMailer(nil)
	}

	// Create template and email services
	templates := mail.NewTemplateService(s.db)
	s.emailService = mail.NewEmailService(s.mailer, templates, s.mailConfig)
}

// ReloadMail recreates the mailer with new configuration.
// Called by dashboard when mail settings change.
func (s *Server) ReloadMail(cfg *dashboard.MailConfig) error {
	// Update mail config
	s.mailConfig.Mode = cfg.Mode
	s.mailConfig.From = cfg.From
	s.mailConfig.SMTPHost = cfg.SMTPHost
	s.mailConfig.SMTPPort = cfg.SMTPPort
	s.mailConfig.SMTPUser = cfg.SMTPUser
	s.mailConfig.SMTPPass = cfg.SMTPPass

	// Reinitialize mailer
	s.initMail()

	// Update dashboard handler with catch mailer state (may be nil if not in catch mode)
	s.dashboardHandler.SetCatchMailer(s.catchMailer)

	log.Info("mail configuration reloaded",
		"mode", cfg.Mode,
		"from", cfg.From,
	)
	return nil
}

func (s *Server) setupRoutes() {
	// CORS middleware for browser-based apps
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Content-Range", "Range", "X-Total-Count"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	s.router.Use(log.RequestLogger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/health", s.handleHealth)

	// Auth routes
	s.router.Route("/auth/v1", func(r chi.Router) {
		r.Post("/signup", s.handleSignup)
		r.Post("/token", s.handleToken)
		r.Post("/recover", s.handleRecover)
		r.Post("/verify", s.handleVerify)
		r.Get("/verify", s.handleVerify)
		r.Get("/settings", s.handleSettings)
		r.Post("/magiclink", s.handleMagicLink)
		r.Post("/otp", s.handleOTP) // Supabase signInWithOtp endpoint
		r.Post("/resend", s.handleResend)

		// OAuth routes
		r.Get("/authorize", s.handleAuthorize)
		r.Get("/callback", s.handleCallback)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/user", s.handleGetUser)
			r.Put("/user", s.handleUpdateUser)
			r.Post("/logout", s.handleLogout)
			r.Post("/invite", s.handleInvite)
			r.Get("/user/identities", s.handleGetIdentities)
			r.Delete("/user/identities/{provider}", s.handleUnlinkIdentity)
		})
	})

	// REST routes (with API key validation and optional auth for RLS)
	s.router.Route("/rest/v1", func(r chi.Router) {
		r.Use(s.apiKeyMiddleware)       // Validates apikey header, extracts role
		r.Use(s.optionalAuthMiddleware) // Extracts user JWT if present
		// OpenAPI schema endpoint (must be before /{table} to avoid conflict)
		r.Get("/", s.handleOpenAPI)
		r.Get("/{table}", s.restHandler.HandleSelect)
		r.Head("/{table}", s.restHandler.HandleSelect) // HEAD for count-only queries
		r.Post("/{table}", s.restHandler.HandleInsert)
		r.Patch("/{table}", s.restHandler.HandleUpdate)
		r.Delete("/{table}", s.restHandler.HandleDelete)

		// RPC endpoint
		r.Post("/rpc/{name}", s.rpcHandler.HandleRPC)
	})

	// Admin API routes
	s.router.Route("/admin/v1", func(r chi.Router) {
		r.Post("/tables", s.adminHandler.CreateTable)
		r.Get("/tables", s.adminHandler.ListTables)
		r.Get("/tables/{name}", s.adminHandler.GetTable)
		r.Delete("/tables/{name}", s.adminHandler.DeleteTable)

		// FTS index management
		r.Post("/tables/{name}/fts", s.adminHandler.CreateFTSIndex)
		r.Get("/tables/{name}/fts", s.adminHandler.ListFTSIndexes)
		r.Get("/tables/{name}/fts/{index}", s.adminHandler.GetFTSIndex)
		r.Delete("/tables/{name}/fts/{index}", s.adminHandler.DeleteFTSIndex)
		r.Post("/tables/{name}/fts/{index}/rebuild", s.adminHandler.RebuildFTSIndex)
	})

	// Dashboard routes
	s.router.Route("/_", func(r chi.Router) {
		s.dashboardHandler.RegisterRoutes(r)
	})

	// Storage routes
	if s.storageHandler != nil {
		s.router.Route("/storage/v1", func(r chi.Router) {
			// Public routes - no API key required
			r.Get("/object/public/{bucketName}/*", s.storageHandler.GetPublicObject)
			r.Get("/object/sign/{bucketName}/*", s.storageHandler.GetSignedObject)           // Download via signed URL
			r.Put("/object/upload/sign/{bucketName}/*", s.storageHandler.UploadToSignedURL) // Upload via signed URL

			// All other routes require API key
			r.Group(func(r chi.Router) {
				r.Use(s.apiKeyMiddleware)       // Validates apikey header
				r.Use(s.optionalAuthMiddleware) // Extracts user JWT if present
				s.storageHandler.RegisterRoutes(r)
			})
		})
	}
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Server) ListenAndServe(addr string) error {
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server(s).
func (s *Server) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown HTTPS server if running
	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTPS server: %w", err))
		}
	}

	// Shutdown HTTP redirect server if running
	if s.httpRedirect != nil {
		if err := s.httpRedirect.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP redirect server: %w", err))
		}
	}

	// Shutdown main HTTP server if running (non-TLS mode)
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("HTTP server: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// handleOpenAPI generates and returns the OpenAPI 3.0 specification for the REST API.
func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec, err := rest.GenerateOpenAPISpec(s.db.DB)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "openapi_error",
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

// EmailService returns the email service for use by auth handlers.
func (s *Server) EmailService() *mail.EmailService {
	return s.emailService
}

// configureOAuthProvider creates and registers an OAuth provider.
// This is primarily used for testing.
func (s *Server) configureOAuthProvider(name, clientID, clientSecret string, enabled bool) {
	cfg := oauth.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Enabled:      enabled,
	}

	var provider oauth.Provider
	switch name {
	case "google":
		provider = oauth.NewGoogleProvider(cfg)
	case "github":
		provider = oauth.NewGitHubProvider(cfg)
	default:
		return
	}

	s.oauthRegistry.RegisterWithConfig(provider, enabled)
}

// setAllowedRedirectURLs sets the allowed redirect URLs for OAuth flows.
func (s *Server) setAllowedRedirectURLs(urls []string) {
	s.allowedRedirectURLs = urls
}

// SetBaseURL sets the base URL for constructing callback URLs.
func (s *Server) SetBaseURL(url string) {
	s.baseURL = url
}

// EnableFunctions enables edge functions support.
func (s *Server) EnableFunctions(cfg *functions.Config) error {
	if s.functionsEnabled {
		return nil
	}

	// Create functions service
	svc, err := functions.NewService(s.db.DB, cfg)
	if err != nil {
		return err
	}

	s.functionsService = svc
	s.functionsHandler = functions.NewHandler(svc, s.jwtSecret)
	s.functionsEnabled = true

	// Set functions service on dashboard handler for management UI
	s.dashboardHandler.SetFunctionsService(svc)

	// Register routes
	// Note: We need to clear the Content-Type header set by the global middleware,
	// because the edge function response will have its own Content-Type and we
	// don't want duplicates (which breaks JSON parsing in clients).
	s.router.Route("/functions/v1", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Delete the Content-Type header set by global middleware
				// The edge function's response will provide its own
				w.Header().Del("Content-Type")
				next.ServeHTTP(w, r)
			})
		})
		s.functionsHandler.RegisterRoutes(r)
	})

	log.Info("edge functions enabled", "functions_dir", cfg.FunctionsDir)
	return nil
}

// StartFunctions starts the edge runtime.
func (s *Server) StartFunctions(ctx context.Context) error {
	if !s.functionsEnabled || s.functionsService == nil {
		return nil
	}
	return s.functionsService.Start(ctx)
}

// StopFunctions stops the edge runtime.
func (s *Server) StopFunctions() error {
	if !s.functionsEnabled || s.functionsService == nil {
		return nil
	}
	return s.functionsService.Stop()
}

// FunctionsService returns the functions service if enabled.
func (s *Server) FunctionsService() *functions.Service {
	return s.functionsService
}

// FunctionsEnabled returns true if functions are enabled.
func (s *Server) FunctionsEnabled() bool {
	return s.functionsEnabled
}

// GetRPCInterceptor returns the RPC interceptor for SQL interception.
func (s *Server) GetRPCInterceptor() *rpc.Interceptor {
	return s.rpcInterceptor
}

// SetDashboardStore sets the dashboard store for auth settings.
func (s *Server) SetDashboardStore(store *dashboard.Store) {
	s.dashboardStore = store
}

// isAnonymousSigninEnabled checks if anonymous sign-in is enabled.
func (s *Server) isAnonymousSigninEnabled() bool {
	if s.dashboardStore == nil {
		return true // Default enabled if no store
	}
	val, err := s.dashboardStore.Get("auth_allow_anonymous")
	if err != nil {
		return true // Default enabled on error
	}
	return val != "false"
}

// EnableRealtime enables realtime WebSocket support
func (s *Server) EnableRealtime() {
	if s.realtimeService != nil {
		return
	}

	// Get API keys from dashboard store
	var anonKey, serviceKey string
	if s.dashboardStore != nil {
		anonKey, _ = s.dashboardStore.Get("anon_key")
		serviceKey, _ = s.dashboardStore.Get("service_role_key")
	}

	cfg := realtime.Config{
		JWTSecret:  s.jwtSecret,
		AnonKey:    anonKey,
		ServiceKey: serviceKey,
	}

	s.realtimeService = realtime.NewService(s.db.DB, s.rlsService, cfg)

	// Set notifier on REST handler
	if s.restHandler != nil {
		s.restHandler.SetRealtimeNotifier(s.realtimeService)
		log.Debug("realtime: notifier set on REST handler")
	} else {
		log.Warn("realtime: REST handler is nil, cannot set notifier")
	}

	// Set realtime service on dashboard handler for stats API
	if s.dashboardHandler != nil {
		s.dashboardHandler.SetRealtimeService(s.realtimeService)
	}

	// Register WebSocket route
	s.router.Get("/realtime/v1/websocket", s.realtimeService.HandleWebSocket)

	log.Info("realtime enabled")
}

// RealtimeService returns the realtime service (may be nil)
func (s *Server) RealtimeService() *realtime.Service {
	return s.realtimeService
}

// StartTUSCleanup starts the TUS expired upload cleanup routine.
func (s *Server) StartTUSCleanup(ctx context.Context) {
	if s.storageService == nil {
		return
	}
	tusService := s.storageService.TUSService("")
	if tusService != nil {
		tusService.StartCleanupRoutine(ctx, 1*time.Hour)
		log.Info("TUS upload cleanup routine started")
	}
}
