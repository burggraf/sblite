// internal/server/server.go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/rest"
	"github.com/markb/sblite/internal/rls"
)

type Server struct {
	db          *db.DB
	router      *chi.Mux
	authService *auth.Service
	rlsService  *rls.Service
	rlsEnforcer *rls.Enforcer
	restHandler *rest.Handler
}

func New(database *db.DB, jwtSecret string) *Server {
	rlsService := rls.NewService(database)
	rlsEnforcer := rls.NewEnforcer(rlsService)

	s := &Server{
		db:          database,
		router:      chi.NewRouter(),
		authService: auth.NewService(database, jwtSecret),
		rlsService:  rlsService,
		rlsEnforcer: rlsEnforcer,
		restHandler: rest.NewHandler(database, rlsEnforcer),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))

	s.router.Get("/health", s.handleHealth)

	// Auth routes
	s.router.Route("/auth/v1", func(r chi.Router) {
		r.Post("/signup", s.handleSignup)
		r.Post("/token", s.handleToken)
		r.Post("/verify", s.handleVerify)
		r.Get("/verify", s.handleVerify)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Get("/user", s.handleGetUser)
			r.Put("/user", s.handleUpdateUser)
			r.Post("/logout", s.handleLogout)
		})
	})

	// REST routes (with optional auth for RLS)
	s.router.Route("/rest/v1", func(r chi.Router) {
		r.Use(s.optionalAuthMiddleware)
		r.Get("/{table}", s.restHandler.HandleSelect)
		r.Post("/{table}", s.restHandler.HandleInsert)
		r.Patch("/{table}", s.restHandler.HandleUpdate)
		r.Delete("/{table}", s.restHandler.HandleDelete)
	})
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
