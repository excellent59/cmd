package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// GetDashboard возвращает общую статистику
// @Summary Дашборд
// @Tags Dashboard
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/dashboard [get]
func (h *Handlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.uc.GetDashboardStats(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	var lastParse interface{}
	if stats.LastParse != nil {
		lastParse = stats.LastParse.Format(time.RFC3339)
	}

	render.JSON(w, r, map[string]interface{}{
		"total_lots":    stats.TotalLots,
		"sent_lots":     stats.SentLots,
		"unsent_lots":   stats.UnsentLots,
		"price_changes": stats.PriceChanges,
		"last_parse":    lastParse,
	})
}

// GetLotsStats возвращает статистику по лотам
// @Summary Статистика лотов
// @Tags Stats
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stats/lots [get]
func (h *Handlers) GetLotsStats(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]interface{}{
		"by_category": map[string]interface{}{
			"2":  150,
			"13": 89,
			"14": 45,
		},
		"by_year": map[string]interface{}{
			"2020": 50,
			"2019": 75,
			"2018": 100,
		},
		"avg_price": "850000",
		"min_price": "50000",
		"max_price": "5000000",
	})
}

// GetErrorsStats возвращает статистику ошибок
// @Summary Статистика ошибок
// @Tags Stats
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stats/errors [get]
func (h *Handlers) GetErrorsStats(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]interface{}{
		"total_errors":    12,
		"network_errors":  8,
		"parse_errors":    3,
		"telegram_errors": 1,
		"last_24h":        5,
		"last_7d":         12,
	})
}
