package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/logger"
	"pro-lot-bot/internal/usecase"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotHandler struct {
	bot        *tgbotapi.BotAPI
	usecase    *usecase.LotUsecase
	chatID     int64
	httpClient *http.Client
}

func NewBotHandler(
	bot *tgbotapi.BotAPI,
	usecase *usecase.LotUsecase,
	chatID int64,
) *BotHandler {
	return &BotHandler{
		bot:     bot,
		usecase: usecase,
		chatID:  chatID,
		httpClient: &http.Client{
			// Таймаут по умолчанию для клиента
			Timeout: 30 * time.Second,
		},
	}
}

// ProcessAndSendLots - главный метод, который вызывает планировщик.
// Реализует надежную доставку: синхронизация -> отправка всех неотправленных -> фиксация успеха.
func (h *BotHandler) ProcessAndSendLots(ctx context.Context) error {
	log := logger.Get()

	// 1. Синхронизируемся с сайтом (парсим и сохраняем новые/измененные лоты в БД со статусом is_sent=false)
	// Это первый этап флоу: данные из внешнего мира (сайт) попадают в нашу инфраструктуру (БД).
	log.Info("🔄 Начинаю синхронизацию с сайтом...")
	removedRefs, err := h.usecase.SyncWithWebsite(ctx)
	if err != nil {
		log.Errorw("Ошибка синхронизации с сайтом (пробуем отправить накопленные лоты)", "error", err)
		// Мы НЕ делаем return здесь! Мы все равно хотим попытаться отправить лоты,
		// которые не ушли в прошлые разы из-за ошибок сети.
	}

	// Удаляем в Telegram сообщения лотов, которые пропали с сайта (best-effort).
	h.deleteRemovedMessages(removedRefs)

	// Если во время синхронизации пришёл сигнал остановки — рассылку НЕ начинаем.
	// Иначе отправка пойдёт с уже отменённым ctx: фото не скачаются и уйдёт голый текст.
	if ctx.Err() != nil {
		log.Warn("⏹ Запрошена остановка — рассылка отменена")
		return ctx.Err()
	}

	// 2. Получаем ВСЕ лоты, которые еще не были успешно отправлены
	unsentLots, err := h.usecase.GetUnsentLots(ctx)
	if err != nil {
		log.Errorw("Критическая ошибка получения неотправленных лотов", "error", err)
		return err
	}

	if len(unsentLots) == 0 {
		log.Info("✅ Нет лотов для отправки. Все актуально.")
		return nil
	}

	log.Infow("Найдены лоты для отправки в Telegram", "count", len(unsentLots))

	// 3. Отправляем каждый лот и СРАЗУ фиксируем факт отправки в БД.
	// Так при сбое/аварийной остановке уже отправленный лот не уйдёт повторно:
	// максимум можно потерять фиксацию одного лота — того, что отправлялся в момент сбоя.
	sentCount := 0

	for _, lot := range unsentLots {
		// Реагируем на остановку ДО отправки следующего лота (graceful shutdown)
		if ctx.Err() != nil {
			log.Warnw("⏹ Запрошена остановка — прерываю рассылку", "sent", sentCount)
			break
		}

		msgIDs, err := h.sendLot(ctx, lot)
		if err != nil {
			log.Errorw("❌ Не удалось отправить лот (будет повторено в следующем цикле)",
				"lot_id", lot.ID, "error", err)
			continue // Не фиксируем — попробуем отправить в следующем цикле
		}

		// Фиксируем отправку сразу же, атомарно (is_sent + message_id в одной транзакции).
		// Контекст от context.Background(): даже при остановке важно записать, что лот
		// уже физически отправлен, иначе он уйдёт повторно при следующем запуске.
		markCtx, markCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := h.usecase.MarkLotSent(markCtx, lot.ID, h.chatID, msgIDs); err != nil {
			log.Errorw("⚠️ Лот отправлен, но не удалось зафиксировать статус (возможен повтор)",
				"lot_id", lot.ID, "error", err)
		} else {
			log.Infow("✅ Лот отправлен и зафиксирован", "lot_id", lot.ID)
			sentCount++
		}
		markCancel()

		// Пауза, чтобы не получить бан от Telegram, но прерываемая по остановке
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}

	if sentCount > 0 {
		log.Infow("Рассылка завершена", "sent", sentCount)
	}

	return nil
}

// deleteRemovedMessages удаляет в Telegram сообщения по лотам, снятым с сайта (best-effort).
// Telegram может отказать (например, сообщение старше 48 часов) — тогда просто логируем.
func (h *BotHandler) deleteRemovedMessages(refs []domain.SentMessageRef) {
	if len(refs) == 0 {
		return
	}
	log := logger.Get()
	deleted := 0
	for _, ref := range refs {
		if _, err := h.bot.Request(tgbotapi.NewDeleteMessage(ref.ChatID, ref.MessageID)); err != nil {
			log.Warnw("Не удалось удалить сообщение в Telegram (возможно, старше 48 часов)",
				"chat_id", ref.ChatID, "message_id", ref.MessageID, "error", err)
			continue
		}
		deleted++
	}
	if deleted > 0 {
		log.Infow("🗑 Удалены сообщения в Telegram по снятым с сайта лотам", "count", deleted)
	}
}

// sendLot отправляет лот и возвращает ID отправленных сообщений (для последующего удаления).
func (h *BotHandler) sendLot(ctx context.Context, lot domain.Lot) ([]int, error) {
	if len(lot.Images) > 0 {
		return h.sendLotWithImages(ctx, lot)
	}

	msg := tgbotapi.NewMessage(h.chatID, FormatLotMessage(lot))
	msg.ParseMode = "HTML"
	sent, err := h.bot.Send(msg)
	if err != nil {
		return nil, err
	}
	return []int{sent.MessageID}, nil
}

func (h *BotHandler) sendLotWithImages(ctx context.Context, lot domain.Lot) ([]int, error) {
	log := logger.Get()

	if len(lot.Images) == 0 {
		return nil, fmt.Errorf("no images found")
	}

	// Скачиваем все изображения параллельно
	imagesData := h.downloadImagesParallel(ctx, lot.Images)

	// Фильтруем успешные загрузки и логируем неудачные
	var validImages []ImageData
	for _, img := range imagesData {
		if img.data != nil && img.err == nil {
			validImages = append(validImages, img)
		} else if img.err != nil {
			log.Warnw("Не удалось скачать изображение для лота",
				"lot_id", lot.ID,
				"url", img.url,
				"error", img.err,
			)
		}
	}

	// Если ни одно фото не загрузилось, отправляем хотя бы текст
	if len(validImages) == 0 {
		log.Warnw("Ни одно изображение не загрузилось, отправляем только текст", "lot_id", lot.ID)
		msg := tgbotapi.NewMessage(h.chatID, FormatLotMessage(lot))
		msg.ParseMode = "HTML"
		sent, err := h.bot.Send(msg)
		if err != nil {
			return nil, err
		}
		return []int{sent.MessageID}, nil
	}

	// Создаем массив медиа для альбома
	var mediaItems []interface{}

	// Первое фото с подписью
	photo1 := tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
		Name:  "lot_main.jpg",
		Bytes: validImages[0].data,
	})
	photo1.Caption = FormatLotMessage(lot)
	photo1.ParseMode = "HTML"
	mediaItems = append(mediaItems, photo1)

	// Остальные фото (максимум 9 дополнительных, итого 10 в альбоме Telegram)
	for i, img := range validImages[1:] {
		if i >= 9 {
			break
		}

		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileBytes{
			Name:  fmt.Sprintf("lot_%d.jpg", i+1),
			Bytes: img.data,
		})
		mediaItems = append(mediaItems, photo)
	}

	// Создаем и отправляем медиа-группу
	mediaGroup := tgbotapi.NewMediaGroup(h.chatID, mediaItems)
	sentMsgs, err := h.bot.SendMediaGroup(mediaGroup)
	if err != nil {
		log.Errorw("Ошибка отправки медиа-группы в Telegram", "lot_id", lot.ID, "error", err)
		return nil, err
	}

	msgIDs := make([]int, 0, len(sentMsgs))
	for _, m := range sentMsgs {
		msgIDs = append(msgIDs, m.MessageID)
	}

	log.Infow("Лот с фото успешно отправлен",
		"lot_id", lot.ID,
		"images_sent", len(validImages),
		"messages", len(msgIDs),
	)

	return msgIDs, nil
}

type ImageData struct {
	url  string
	data []byte
	err  error
}

// downloadImagesParallel — скачивает все фото ПАРАЛЛЕЛЬНО
func (h *BotHandler) downloadImagesParallel(ctx context.Context, urls []string) []ImageData {
	results := make([]ImageData, len(urls))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Ограничиваем до 5 одновременных загрузок

	var mu sync.Mutex

	for i, url := range urls {
		wg.Add(1)
		go func(index int, imgURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			data, err := h.downloadImage(ctx, imgURL)

			// Безопасная запись с мьютексом
			mu.Lock()
			results[index] = ImageData{
				url:  imgURL,
				data: data,
				err:  err,
			}
			mu.Unlock()
		}(i, url)
	}

	wg.Wait()
	return results
}

func (h *BotHandler) downloadImage(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://pro-lot.ru/")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
