package usecase

import (
	"context"

	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/logger"
)

type LotUsecase struct {
	scraper    LotScraper
	repository LotRepository
}

func NewLotUsecase(
	scraper LotScraper,
	repository LotRepository,
) *LotUsecase {
	return &LotUsecase{
		scraper:    scraper,
		repository: repository,
	}
}

func (u *LotUsecase) GetNewLots(ctx context.Context) ([]domain.Lot, error) {
	log := logger.Get()

	// 1. Скрапер идет на сайт и собирает все лоты.
	// Мы передаем фильтры: какие категории искать и сколько страниц парсить.
	result, err := u.scraper.GetLots(ctx, domain.LotFilter{
		CategoryIDs: u.repository.GetCategories(),
		Limit:       0,
		MaxPages:    25,
	})
	if err != nil {
		log.Errorw("Ошибка при получении лотов от скрапера", "error", err)
		return nil, err
	}
	allLots := result.Lots

	var lotsToSend []domain.Lot
	newCount := 0
	updatedCount := 0

	for _, lot := range allLots {
		if u.repository.HasLot(ctx, lot.ID) {
			oldPrice, exists := u.repository.GetLotPrice(ctx, lot.ID)
			if exists && oldPrice != lot.Price {
				log.Infow("Цена изменилась для лота",
					"lot_id", lot.ID,
					"old_price", oldPrice,
					"new_price", lot.Price,
				)

				err := u.repository.UpdateLotPrice(ctx, lot.ID, lot.Price)
				if err != nil {
					log.Errorw("Ошибка обновления цены лота", "lot_id", lot.ID, "error", err)
					continue
				}

				lotsToSend = append(lotsToSend, lot)
				updatedCount++
			}
		} else {
			log.Infow("Обнаружен новый лот", "lot_id", lot.ID)

			// ПЕРЕДАЕМ ЧИСТЫЙ domain.Lot!
			// Репозиторий сам разберется, как его сохранить.
			err := u.repository.AddLot(ctx, lot)
			if err != nil {
				log.Errorw("Ошибка добавления лота в БД", "lot_id", lot.ID, "error", err)
				continue
			}

			lotsToSend = append(lotsToSend, lot)
			newCount++
		}
	}

	log.Infow("Анализ лотов завершен",
		"total_to_send", len(lotsToSend),
		"new_lots", newCount,
		"updated_lots", updatedCount,
	)

	return lotsToSend, nil
}

// SyncWithWebsite парсит сайт, сохраняет новые/обновлённые лоты (is_sent = false) и
// удаляет лоты, пропавшие с сайта. Возвращает ссылки на сообщения в Telegram,
// которые нужно удалить (для снятых лотов) — их удаляет уже delivery-слой (бот).
func (u *LotUsecase) SyncWithWebsite(ctx context.Context) ([]domain.SentMessageRef, error) {
	log := logger.Get()

	// Категории берём из настроек (их можно менять через админку), иначе — из конфигурации.
	categories := u.repository.GetCategories()
	if st, err := u.repository.GetSettings(ctx); err == nil && len(st.Categories) > 0 {
		categories = st.Categories
	}

	result, err := u.scraper.GetLots(ctx, domain.LotFilter{
		CategoryIDs: categories,
		Limit:       0,
		MaxPages:    24,
	})
	if err != nil {
		log.Errorw("Ошибка при получении лотов от скрапера", "error", err)
		return nil, err
	}

	for _, lot := range result.Lots {
		if u.repository.HasLot(ctx, lot.ID) {
			oldPrice, exists := u.repository.GetLotPrice(ctx, lot.ID)
			if exists && oldPrice != lot.Price {
				log.Infow("Цена изменилась", "lot_id", lot.ID, "old_price", oldPrice, "new_price", lot.Price)
				// UpdateLotPrice сохранит старую цену и выставит is_sent = false,
				// чтобы лот переслался с уведомлением «старая/новая цена».
				u.repository.UpdateLotPrice(ctx, lot.ID, lot.Price)
			}
		} else {
			log.Infow("Обнаружен новый лот", "lot_id", lot.ID)
			if err := u.repository.AddLot(ctx, lot); err != nil { // Внутри AddLot is_sent = false
				log.Errorw("Ошибка добавления лота в БД", "lot_id", lot.ID, "error", err)
			}
		}
	}

	// Удаляем лоты, которых больше нет на сайте — ТОЛЬКО если парсингу можно доверять.
	if !result.Complete {
		log.Warn("Парсинг неполный (ошибка страницы или не дошли до конца) — удаление снятых лотов пропущено")
		return nil, nil
	}
	if len(result.SeenIDs) == 0 {
		log.Warn("Парсинг вернул 0 лотов — удаление пропущено (защита от очистки всей базы)")
		return nil, nil
	}

	refs, removedIDs, err := u.repository.DeleteRemovedLots(ctx, result.SeenIDs)
	if err != nil {
		log.Errorw("Ошибка удаления снятых с сайта лотов", "error", err)
		return nil, nil // не срываем цикл рассылки из-за ошибки удаления
	}
	if len(removedIDs) > 0 {
		log.Infow("Удалены лоты, которых больше нет на сайте", "count", len(removedIDs), "ids", removedIDs)
	}
	return refs, nil
}

// GetUnsentLots возвращает лоты, требующие отправки
func (u *LotUsecase) GetUnsentLots(ctx context.Context) ([]domain.Lot, error) {
	return u.repository.GetUnsentLots(ctx)
}

// MarkLotSent атомарно фиксирует отправку лота (статус is_sent + ID сообщений).
func (u *LotUsecase) MarkLotSent(ctx context.Context, lotID string, chatID int64, messageIDs []int) error {
	return u.repository.MarkLotSent(ctx, lotID, chatID, messageIDs)
}
