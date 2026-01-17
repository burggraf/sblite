// internal/server/server.go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite/internal/admin"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/mail/viewer"
	"github.com/markb/sblite/internal/rest"
	"github.com/markb/sblite/internal/rls"
	"github.com/markb/sblite/internal/schema"
)

type Server struct {
	db           *db.DB
	router       *chi.Mux
	authService  *auth.Service
	rlsService   *rls.Service
	rlsEnforcer  *rls.Enforcer
	restHandler  *rest.Handler
	mailConfig   *mail.Config
	mailer       mail.Mailer
	catchMailer  *mail.CatchMailer
	emailService *mail.EmailService
	adminHandler *admin.Handler
	schema       *schema.Schema
}

func New(database *db.DB, jwtSecret string, mailConfig *mail.Config) *Server {
	rlsService := rls.NewService(database)
	rlsEnforcer := rls.NewEnforcer(rlsService)

	// Use default config if nil
	if mailConfig == nil {
		mailConfig = mail.DefaultConfig()
	}

	// Initialize schema first (needed by REST handler)
	schemaInstance := schema.New(database.DB)

	s := &Server{
		db:          database,
		router:      chi.NewRouter(),
		authService: auth.NewService(database, jwtSecret),
		rlsService:  rlsService,
		rlsEnforcer: rlsEnforcer,
		restHandler: rest.NewHandler(database, rlsEnforcer, schemaInstance),
		mailConfig:  mailConfig,
		schema:      schemaInstance,
	}

	// Initialize admin handler (uses schema)
	s.adminHandler = admin.NewHandler(s.db, s.schema)

	// Initialize mail services
	s.initMail()

	s.setupRoutes()
	return s
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
		r.Post("/resend", s.handleResend)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/user", s.handleGetUser)
			r.Put("/user", s.handleUpdateUser)
			r.Post("/logout", s.handleLogout)
			r.Post("/invite", s.handleInvite)
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
	})

	// Mail viewer routes (only in catch mode)
	if s.mailConfig.Mode == mail.ModeCatch && s.catchMailer != nil {
		s.router.Route("/mail", func(r chi.Router) {
			viewerHandler := viewer.NewHandler(s.catchMailer)
			viewerHandler.RegisterRoutes(r)
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
	return http.ListenAndServe(addr, s.router)
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
