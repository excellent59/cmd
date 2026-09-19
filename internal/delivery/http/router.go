package http

import (
	"net/http"
	"pro-lot-bot/internal/config"
	"pro-lot-bot/internal/delivery/http/handlers"
	"pro-lot-bot/internal/delivery/http/middleware"
	"pro-lot-bot/internal/usecase"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chi      *chi.Mux
	uc       *usecase.LotUsecase
	cfg      *config.Config
	handlers *handlers.Handlers
}

func NewRouter(uc *usecase.LotUsecase, cfg *config.Config) *Router {
	h := handlers.NewHandlers(uc, cfg)

	r := chi.NewRouter()

	// Глобальные middleware.
	// CORS-заголовки не выставляем: админка отдаётся тем же сервером (same-origin),
	// а Content-Type для JSON проставляет render.JSON.
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)

	// HTML админ-панели и health check (без auth)
	r.Get("/", h.AdminPage)
	r.Get("/health", h.HealthCheck)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Публичные эндпоинты
		r.Post("/auth/login", h.Login)

		// Защищенные эндпоинты
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))

			// Dashboard & Stats
			r.Get("/dashboard", h.GetDashboard)
			r.Get("/stats/lots", h.GetLotsStats)
			r.Get("/stats/errors", h.GetErrorsStats)

			// Lots
			r.Get("/lots", h.GetLots)
			r.Get("/lots/{id}", h.GetLotByID)
			r.Get("/lots/{id}/history", h.GetLotPriceHistory)
			r.Post("/lots/{id}/send", h.SendLotManually)
			r.Delete("/lots/{id}", h.DeleteLot)

			// Control
			r.Post("/parse/trigger", h.TriggerParsing)
			r.Post("/parse/stop", h.StopParsing)

			// Config
			r.Get("/config", h.GetConfig)
			r.Put("/config", h.UpdateConfig)

			// Logs
			r.Get("/logs", h.GetLogs)
		})
	})

	return &Router{
		chi:      r,
		uc:       uc,
		cfg:      cfg,
		handlers: h,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.chi.ServeHTTP(w, req)
}
