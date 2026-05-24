package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/t0mer/dnsmon/internal/api/docs"
	v1 "github.com/t0mer/dnsmon/internal/api/v1"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/monitor"
	"github.com/t0mer/dnsmon/internal/notify"
	"github.com/t0mer/dnsmon/internal/resolvers"
	"github.com/t0mer/dnsmon/internal/storage"
	"github.com/t0mer/dnsmon/web"
)

// Server holds the HTTP router and its dependencies.
type Server struct {
	router   chi.Router
	checker  *checker.Checker
	registry *resolvers.Registry
	storage  storage.Storage
	cfg      *config.Config
	log      *slog.Logger
}

// NewServer constructs a Server and registers all routes.
func NewServer(
	chkr *checker.Checker,
	registry *resolvers.Registry,
	store storage.Storage,
	cfg *config.Config,
	log *slog.Logger,
) *Server {
	s := &Server{
		router:   chi.NewRouter(),
		checker:  chkr,
		registry: registry,
		storage:  store,
		cfg:      cfg,
		log:      log,
	}
	s.routes()
	return s
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) routes() {
	r := s.router

	r.Use(RequestID)
	r.Use(Logger(s.log))
	r.Use(Recovery(s.log))
	r.Use(middleware.StripSlashes)

	// Static web assets — catch-all serves /css/*, /js/*, /vendor/*, and root
	fileServer := web.Handler()
	r.Get("/", fileServer.ServeHTTP)
	r.Get("/css/*", fileServer.ServeHTTP)
	r.Get("/js/*", fileServer.ServeHTTP)
	r.Get("/static/*", fileServer.ServeHTTP)
	r.Get("/vendor/*", fileServer.ServeHTTP)

	// SPA routes — serve index.html for client-side routing
	r.Get("/check/{id}", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
	r.Get("/lookup", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/lookup.html"
		fileServer.ServeHTTP(w, r)
	})
	r.Get("/reverse", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/reverse.html"
		fileServer.ServeHTTP(w, r)
	})
	r.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/about.html"
		fileServer.ServeHTTP(w, r)
	})
	r.With(RequireAuth(s.storage, false)).Get("/settings", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/settings.html"
		fileServer.ServeHTTP(w, r)
	})
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/login.html"
		fileServer.ServeHTTP(w, r)
	})
	r.Get("/api-docs", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/api-docs.html"
		fileServer.ServeHTTP(w, r)
	})

	// API docs
	r.Mount("/api/docs", docs.Handler())

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(CORS())

		r.Get("/health", v1.Health)
		r.Get("/readyz", v1.Readyz(s.storage))
		r.Get("/version", v1.Version)

		r.Get("/resolvers", v1.ListResolvers(s.registry))
		r.Get("/resolvers/{id}", v1.GetResolver(s.registry))

		r.Get("/record-types", v1.RecordTypes())

		r.Post("/check", v1.PostCheck(s.checker))
		r.Get("/check/stream", v1.StreamCheck(s.checker))
		r.Get("/check/{id}", v1.GetCheck(s.storage))
		r.Get("/check/{id}/export", v1.ExportCheck(s.storage))

		r.Post("/lookup", v1.PostLookup(s.checker))
		r.Post("/reverse", v1.PostReverse(s.checker))

		r.Get("/history", v1.ListHistory(s.storage))
		r.Delete("/history/{id}", v1.DeleteHistory(s.storage))

		// Authentication (public so the login form can reach it)
		r.Post("/auth/login", v1.Login(s.storage))
		r.Post("/auth/logout", v1.Logout())
		r.Get("/auth/session", v1.Session(s.storage))

		// Settings — gated by UI auth when enabled.
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(s.storage, true))

			r.Get("/settings", v1.GetSettings(s.storage))
			r.Put("/settings", v1.UpdateSettings(s.storage))
			r.Post("/settings/notifications/test", v1.TestNotification(notify.New()))
			r.Get("/settings/tokens", v1.ListTokens(s.storage))
			r.Post("/settings/tokens", v1.CreateToken(s.storage))
			r.Delete("/settings/tokens/{id}", v1.DeleteToken(s.storage))
			r.Get("/settings/schedules", v1.ListSchedules(s.storage))
			r.Post("/settings/schedules", v1.CreateSchedule(s.storage))
			r.Put("/settings/schedules/{id}", v1.UpdateSchedule(s.storage))
			r.Delete("/settings/schedules/{id}", v1.DeleteSchedule(s.storage))

			r.Get("/settings/monitors", v1.ListMonitors(s.storage))
			r.Post("/settings/monitors", v1.CreateMonitor(s.storage))
			r.Put("/settings/monitors/{id}", v1.UpdateMonitor(s.storage))
			r.Delete("/settings/monitors/{id}", v1.DeleteMonitor(s.storage))
			r.Get("/settings/monitors/{id}/history", v1.MonitorHistory(s.storage))
			r.Post("/settings/monitors/{id}/run", v1.RunMonitor(monitor.New(s.checker, s.storage, notify.New())))
		})
	})

	// Prometheus metrics
	if s.cfg.Metrics.Enabled {
		r.Handle(s.cfg.Metrics.Path, promhttp.Handler())
	}
}
