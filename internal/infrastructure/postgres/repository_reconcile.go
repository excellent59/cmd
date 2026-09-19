package postgres

import (
	"context"
	"fmt"

	"pro-lot-bot/internal/domain"
)

// MarkLotSent атомарно фиксирует успешную отправку лота: сохраняет ID отправленных
// сообщений и помечает лот как отправленный (сбрасывая previous_price). Всё в одной
// транзакции — чтобы не осталось «наполовину отправленного» состояния при сбое.
func (r *PostgresRepository) MarkLotSent(ctx context.Context, lotID string, chatID int64, messageIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, mid := range messageIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sent_messages (lot_id, chat_id, message_id) VALUES ($1, $2, $3)`,
			lotID, chatID, mid,
		); err != nil {
			return fmt.Errorf("ошибка сохранения message_id: %w", err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE lots SET is_sent = true, previous_price = NULL WHERE id = $1`, lotID,
	); err != nil {
		return fmt.Errorf("ошибка отметки лота отправленным: %w", err)
	}

	return tx.Commit(ctx)
}

// DeleteRemovedLots удаляет из БД лоты, которых нет в seenIDs (пропали с сайта),
// и возвращает ссылки на их сообщения в Telegram (чтобы удалить и там), а также
// список удалённых ID (для логов). Всё в одной транзакции.
//
// ВАЖНО: при пустом seenIDs метод НИЧЕГО не удаляет — иначе снесли бы всю базу.
// Вызывать его нужно только после ДОСТОВЕРНО полного парсинга (ScrapeResult.Complete).
func (r *PostgresRepository) DeleteRemovedLots(ctx context.Context, seenIDs []string) ([]domain.SentMessageRef, []string, error) {
	if len(seenIDs) == 0 {
		return nil, nil, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Сообщения удаляемых лотов — читаем ДО удаления (каскад их снесёт).
	msgRows, err := tx.Query(ctx,
		`SELECT chat_id, message_id FROM sent_messages WHERE lot_id <> ALL($1)`, seenIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка выборки сообщений: %w", err)
	}
	var refs []domain.SentMessageRef
	for msgRows.Next() {
		var ref domain.SentMessageRef
		if err := msgRows.Scan(&ref.ChatID, &ref.MessageID); err != nil {
			msgRows.Close()
			return nil, nil, err
		}
		refs = append(refs, ref)
	}
	msgRows.Close()
	if err := msgRows.Err(); err != nil {
		return nil, nil, err
	}

	// 2. ID удаляемых лотов (для логов).
	idRows, err := tx.Query(ctx, `SELECT id FROM lots WHERE id <> ALL($1)`, seenIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка выборки удаляемых лотов: %w", err)
	}
	var removedIDs []string
	for idRows.Next() {
		var id string
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			return nil, nil, err
		}
		removedIDs = append(removedIDs, id)
	}
	idRows.Close()
	if err := idRows.Err(); err != nil {
		return nil, nil, err
	}

	// 3. Удаляем лоты (каскадом уйдут sent_messages и price_histories).
	if _, err := tx.Exec(ctx, `DELETE FROM lots WHERE id <> ALL($1)`, seenIDs); err != nil {
		return nil, nil, fmt.Errorf("ошибка удаления лотов: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return refs, removedIDs, nil
}
