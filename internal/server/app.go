package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hyfleet/hyfleet/internal/config"
	"github.com/hyfleet/hyfleet/internal/cryptoutil"
	"github.com/hyfleet/hyfleet/internal/store"
	"github.com/hyfleet/hyfleet/internal/webui"
)

type App struct {
	config        config.Server
	store         *store.Store
	masterKey     []byte
	logger        *slog.Logger
	publicOrigin  string
	dummyHash     string
	loginLimiter  *rateLimiter
	enrollLimiter *rateLimiter
}

func New(cfg config.Server, database *store.Store, masterKey []byte, logger *slog.Logger) (*App, error) {
	publicURL, err := url.Parse(cfg.PublicURL)
	if err != nil {
		return nil, err
	}
	dummyHash, err := cryptoutil.HashPassword("not-a-real-password", cryptoutil.DefaultPasswordParams)
	if err != nil {
		return nil, err
	}
	return &App{
		config:        cfg,
		store:         database,
		masterKey:     masterKey,
		logger:        logger,
		publicOrigin:  strings.TrimSuffix(publicURL.Scheme+"://"+publicURL.Host, "/"),
		dummyHash:     dummyHash,
		loginLimiter:  newRateLimiter(8, 5*time.Minute),
		enrollLimiter: newRateLimiter(20, 5*time.Minute),
	}, nil
}

func (a *App) Handler() (http.Handler, error) {
	frontend, err := webui.Handler()
	if err != nil {
		return nil, err
	}
	router := chi.NewRouter()
	router.Use(a.requestIDMiddleware)
	router.Use(a.recoveryMiddleware)
	router.Use(a.loggingMiddleware)
	router.Use(a.securityHeadersMiddleware)

	router.Get("/healthz", a.handleHealth)
	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/setup/status", a.handleSetupStatus)
		api.Post("/setup/bootstrap", a.handleBootstrap)
		api.Post("/auth/login", a.handleLogin)
		api.Group(func(authenticated chi.Router) {
			authenticated.Use(a.sessionMiddleware)
			authenticated.Use(a.csrfMiddleware)
			authenticated.Get("/auth/session", a.handleSession)
			authenticated.Post("/auth/logout", a.handleLogout)
			authenticated.Get("/nodes", a.handleListNodes)
			authenticated.Post("/nodes", a.handleCreateNode)
			authenticated.Get("/nodes/{nodeID}", a.handleGetNode)
			authenticated.Put("/nodes/{nodeID}", a.handleUpdateNode)
			authenticated.Delete("/nodes/{nodeID}", a.handleArchiveNode)
			authenticated.Post("/nodes/{nodeID}/enrollment-token", a.handleEnrollmentToken)
		})
	})
	router.Route("/agent/v1", func(agent chi.Router) {
		agent.Post("/enroll", a.handleAgentEnroll)
		agent.Group(func(secured chi.Router) {
			secured.Use(a.agentMiddleware)
			secured.Post("/heartbeat", a.handleAgentHeartbeat)
			secured.Get("/desired", a.handleAgentDesired)
			secured.Post("/desired/{version}/ack", a.handleAgentDesiredAck)
		})
	})
	router.NotFound(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") || strings.HasPrefix(request.URL.Path, "/agent/") {
			a.writeError(response, request, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		frontend.ServeHTTP(response, request)
	})
	return router, nil
}
