package handlers

import (
	"net/http"
	"pro-lot-bot/internal/config"
	"pro-lot-bot/internal/usecase"
	"time"

	"github.com/go-chi/render"
)

type Handlers struct {
	uc  *usecase.LotUsecase
	cfg *config.Config
}

func NewHandlers(uc *usecase.LotUsecase, cfg *config.Config) *Handlers {
	return &Handlers{
		uc:  uc,
		cfg: cfg,
	}
}

// HealthCheck проверяет здоровье сервиса
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// TriggerParsing запускает парсинг вручную
func (h *Handlers) TriggerParsing(w http.ResponseWriter, r *http.Request) {
	// TODO: Вызвать uc.SyncWithWebsite(r.Context())
	render.JSON(w, r, map[string]interface{}{
		"status": "triggered",
	})
}

// StopParsing останавливает парсинг
func (h *Handlers) StopParsing(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]interface{}{
		"status": "stopped",
	})
}

// GetLogs возвращает логи
func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]interface{}{
		"logs": []string{},
	})
}
