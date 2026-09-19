package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"pro-lot-bot/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// lotToMap приводит доменный лот к JSON-представлению для админки.
func lotToMap(lot domain.Lot) map[string]interface{} {
	return map[string]interface{}{
		"id":          lot.ID,
		"title":       lot.Title,
		"price":       lot.Price,
		"year":        lot.Year,
		"location":    lot.Location,
		"price_trend": lot.PriceTrend,
		"images":      lot.Images,
		"url":         lot.URL,
		"is_sent":     lot.IsSent,
		"created_at":  lot.CreatedAt.Format(time.RFC3339),
		"updated_at":  lot.UpdatedAt.Format(time.RFC3339),
	}
}

// GetLots возвращает список лотов с пагинацией и фильтром по статусу отправки
// @Summary Получить список лотов
// @Tags Lots
// @Accept json
// @Produce json
// @Param page query int false "Номер страницы" default(1)
// @Param limit query int false "Количество на странице" default(20)
// @Param sent query bool false "Статус отправки (true/false)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/lots [get]
func (h *Handlers) GetLots(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var isSent *bool
	switch r.URL.Query().Get("sent") {
	case "true":
		v := true
		isSent = &v
	case "false":
		v := false
		isSent = &v
	}

	pageData, err := h.uc.GetLotsPage(r.Context(), domain.LotQuery{Page: page, Limit: limit, IsSent: isSent})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	items := make([]map[string]interface{}, 0, len(pageData.Lots))
	for _, lot := range pageData.Lots {
		items = append(items, lotToMap(lot))
	}

	pages := 0
	if pageData.Limit > 0 {
		pages = (pageData.Total + pageData.Limit - 1) / pageData.Limit
	}

	render.JSON(w, r, map[string]interface{}{
		"data": items,
		"pagination": map[string]interface{}{
			"page":  pageData.Page,
			"limit": pageData.Limit,
			"total": pageData.Total,
			"pages": pages,
		},
	})
}

// GetLotByID возвращает детали лота
// @Summary Получить лот по ID
// @Tags Lots
// @Accept json
// @Produce json
// @Param id path string true "ID лота"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/lots/{id} [get]
func (h *Handlers) GetLotByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	lot, err := h.uc.GetLotByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrLotNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{"error": "lot not found"})
			return
		}
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	render.JSON(w, r, lotToMap(lot))
}

// GetLotPriceHistory возвращает историю цен лота
// @Summary История цен лота
// @Tags Lots
// @Accept json
// @Produce json
// @Param id path string true "ID лота"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/lots/{id}/history [get]
func (h *Handlers) GetLotPriceHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	history, err := h.uc.GetPriceHistory(r.Context(), id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	items := make([]map[string]interface{}, 0, len(history))
	for _, hst := range history {
		items = append(items, map[string]interface{}{
			"old_price":  hst.OldPrice,
			"new_price":  hst.NewPrice,
			"changed_at": hst.ChangedAt.Format(time.RFC3339),
		})
	}

	render.JSON(w, r, map[string]interface{}{
		"lot_id":  id,
		"history": items,
	})
}

// SendLotManually — ручная отправка лота (пока не реализовано).
// @Summary Отправить лот вручную
// @Tags Lots
// @Router /api/v1/lots/{id}/send [post]
func (h *Handlers) SendLotManually(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusNotImplemented)
	render.JSON(w, r, map[string]interface{}{"error": "not implemented yet"})
}

// DeleteLot удаляет лот
// @Summary Удалить лот
// @Tags Lots
// @Accept json
// @Produce json
// @Param id path string true "ID лота"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/lots/{id} [delete]
func (h *Handlers) DeleteLot(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.uc.DeleteLot(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrLotNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{"error": "lot not found"})
			return
		}
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{"error": err.Error()})
		return
	}

	render.JSON(w, r, map[string]interface{}{"status": "deleted", "lot_id": id})
}
