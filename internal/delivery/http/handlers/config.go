package handlers

import (
	"net/http"
	"time"

	"pro-lot-bot/internal/domain"

	"github.com/go-chi/render"
)

// GetConfig возвращает текущую конфигурацию бота (из БД, с откатом на .env)
// @Summary Получить конфигурацию
// @Tags Config
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config [get]
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	st, err := h.uc.GetSettings(r.Context())
	if err != nil {
		// Настроек в БД ещё нет — отдаём значения из .env
		render.JSON(w, r, map[string]interface{}{
			"poll_interval_minutes": int(h.cfg.PollInterval.Minutes()),
			"categories":            h.cfg.Categories,
			"telegram_chat_id":      h.cfg.TelegramChatID,
		})
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"poll_interval_minutes": int(st.PollInterval.Minutes()),
		"categories":            st.Categories,
		"telegram_chat_id":      h.cfg.TelegramChatID,
		"updated_at":            st.UpdatedAt.Format(time.RFC3339),
	})
}

type updateConfigRequest struct {
	PollIntervalMinutes *int      `json:"poll_interval_minutes"`
	Categories          *[]string `json:"categories"`
}

// UpdateConfig сохраняет настройки в БД. Планировщик подхватит новый интервал на следующем цикле.
// @Summary Обновить конфигурацию
// @Tags Config
// @Accept json
// @Produce json
// @Param config body updateConfigRequest true "Новая конфигурация"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config [put]
func (h *Handlers) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateConfigRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]interface{}{"error": "invalid request body"})
		return
	}

	// За основу берём текущие настройки (или дефолты из .env, если их ещё нет).
	st, err := h.uc.GetSettings(r.Context())
	if err != nil {
		st = domain.Settings{PollInterval: h.cfg.PollInterval, Categories: h.cfg.Categories}
	}

	if req.PollIntervalMinutes != nil {
		if *req.PollIntervalMinutes < 1 {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]interface{}{"error": "poll_interval_minutes must be >= 1"})
			return
		}
		st.PollInterval = time.Duration(*req.PollIntervalMinutes) * time.Minute
	}
	if req.Categories != nil {
		st.Categories = *req.Categories
	}

	if err := h.uc.SaveSettings(r.Context(), st); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"status":                "updated",
		"poll_interval_minutes": int(st.PollInterval.Minutes()),
		"categories":            st.Categories,
	})
}
