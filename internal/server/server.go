// internal/server/server.go
package server

import (
	"context"
	"encoding/json"
	"net/http"

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
	"github.com/markb/sblite/internal/mail/viewer"
	"github.com/markb/sblite/internal/oauth"
	"github.com/markb/sblite/internal/rest"
	"github.com/markb/sblite/internal/rls"
	"github.com/markb/sblite/internal/schema"
	"github.com/markb/sblite/internal/storage"
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

	// HTTP server for graceful shutdown
	httpServer *http.Server

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

	// Initialize mail services
	s.initMail()

	// Initialize dashboard handler
	s.dashboardHandler = dashboard.NewHandler(database.DB, cfg.MigrationsDir)
	s.dashboardHandler.SetJWTSecret(cfg.JWTSecret)
	s.dashboardStore = s.dashboardHandler.GetStore()

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
		// Set storage service on dashboard handler for management UI
		s.dashboardHandler.SetStorageService(storageService)
	} else {
		log.Warn("failed to initialize storage service", "error", err.Error())
	}

	s.setupRoutes()
	return s
}

// SetDashboardConfig sets the dashboard server configuration for display in settings.
func (s *Server) SetDashboardConfig(cfg *dashboard.ServerConfig) {
	s.dashboardHandler.SetServerConfig(cfg)
}

// initMail initializes the mail services based on configuration.
func (s *Server) initMail() {
	switch s.mailConfig.Mode {
	case mail.ModeCatch:
		s.catchMailer = mail.NewCatchMailer(s.db)
		s.mailer = s.catchMailer
	case mail.ModeSMTP:
		smtpConfig := mail.SMTPConfig{
			Host: s.mailConfig.SMTPHost,
			Port: s.mailConfig.SMTPPort,
			User: s.mailConfig.SMTPUser,
			Pass: s.mailConfig.SMTPPass,
		}
		s.mailer = mail.NewSMTPMailer(smtpConfig)
	default:
		// Default to log mode
		s.mailer = mail.NewLogMailer(nil)
	}

	// Create template and email services
	templates := mail.NewTemplateService(s.db)
	s.emailService = mail.NewEmailService(s.mailer, templates, s.mailConfig)
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

	// Mail viewer routes (only in catch mode)
	if s.mailConfig.Mode == mail.ModeCatch && s.catchMailer != nil {
		s.router.Route("/mail", func(r chi.Router) {
			viewerHandler := viewer.NewHandler(s.catchMailer)
			viewerHandler.RegisterRoutes(r)
		})
	}

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

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
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
	s.router.Route("/functions/v1", func(r chi.Router) {
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
