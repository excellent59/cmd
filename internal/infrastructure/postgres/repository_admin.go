package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"pro-lot-bot/internal/domain"

	"github.com/jackc/pgx/v5"
)

// rowScanner объединяет pgx.Row и pgx.Rows — обе умеют Scan.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanLotRow читает один лот из строки с фиксированным набором колонок.
func scanLotRow(row rowScanner) (domain.Lot, error) {
	var lot domain.Lot
	var imagesJSON string
	if err := row.Scan(
		&lot.ID, &lot.URL, &lot.Title, &lot.Year, &lot.Price,
		&lot.Location, &lot.PriceTrend, &imagesJSON, &lot.IsSent, &lot.CreatedAt, &lot.UpdatedAt,
	); err != nil {
		return domain.Lot{}, err
	}
	if imagesJSON != "" && imagesJSON != "null" {
		_ = json.Unmarshal([]byte(imagesJSON), &lot.Images)
	}
	return lot, nil
}

// GetDashboardStats возвращает агрегаты для дашборда админки.
func (r *PostgresRepository) GetDashboardStats(ctx context.Context) (domain.DashboardStats, error) {
	var s domain.DashboardStats
	var lastParse *time.Time

	query := `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE is_sent),
			COUNT(*) FILTER (WHERE NOT is_sent),
			MAX(created_at)
		FROM lots`
	if err := r.pool.QueryRow(ctx, query).Scan(&s.TotalLots, &s.SentLots, &s.UnsentLots, &lastParse); err != nil {
		return domain.DashboardStats{}, fmt.Errorf("ошибка получения статистики: %w", err)
	}
	s.LastParse = lastParse

	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM price_histories`).Scan(&s.PriceChanges); err != nil {
		return domain.DashboardStats{}, fmt.Errorf("ошибка подсчёта изменений цен: %w", err)
	}
	return s, nil
}

// GetLotsPage возвращает страницу лотов с фильтром по статусу отправки.
func (r *PostgresRepository) GetLotsPage(ctx context.Context, q domain.LotQuery) (domain.LotsPage, error) {
	where := ""
	args := []any{}
	if q.IsSent != nil {
		where = "WHERE is_sent = $1"
		args = append(args, *q.IsSent)
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM lots "+where, args...).Scan(&total); err != nil {
		return domain.LotsPage{}, fmt.Errorf("ошибка подсчёта лотов: %w", err)
	}

	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	listArgs := append([]any{}, args...)
	limitPos := len(listArgs) + 1
	offsetPos := len(listArgs) + 2
	listArgs = append(listArgs, limit, offset)

	listQuery := fmt.Sprintf(`
		SELECT id, url, title, year, current_price, location, price_trend, images, is_sent, created_at, updated_at
		FROM lots %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return domain.LotsPage{}, fmt.Errorf("ошибка выборки лотов: %w", err)
	}
	defer rows.Close()

	lots := make([]domain.Lot, 0, limit)
	for rows.Next() {
		lot, err := scanLotRow(rows)
		if err != nil {
			return domain.LotsPage{}, err
		}
		lots = append(lots, lot)
	}
	if err := rows.Err(); err != nil {
		return domain.LotsPage{}, err
	}

	return domain.LotsPage{Lots: lots, Total: total, Page: page, Limit: limit}, nil
}

// GetLotByID возвращает один лот или domain.ErrLotNotFound.
func (r *PostgresRepository) GetLotByID(ctx context.Context, id string) (domain.Lot, error) {
	query := `
		SELECT id, url, title, year, current_price, location, price_trend, images, is_sent, created_at, updated_at
		FROM lots WHERE id = $1`
	lot, err := scanLotRow(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lot{}, domain.ErrLotNotFound
		}
		return domain.Lot{}, fmt.Errorf("ошибка получения лота: %w", err)
	}
	return lot, nil
}

// DeleteLot удаляет лот (история цен уйдёт каскадом).
func (r *PostgresRepository) DeleteLot(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM lots WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("ошибка удаления лота: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLotNotFound
	}
	return nil
}

// GetPriceHistory возвращает историю изменения цены лота (свежие сверху).
func (r *PostgresRepository) GetPriceHistory(ctx context.Context, id string) ([]domain.PriceHistory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT old_price, new_price, changed_at
		FROM price_histories
		WHERE lot_id = $1
		ORDER BY changed_at DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения истории цен: %w", err)
	}
	defer rows.Close()

	history := make([]domain.PriceHistory, 0)
	for rows.Next() {
		var h domain.PriceHistory
		if err := rows.Scan(&h.OldPrice, &h.NewPrice, &h.ChangedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

// GetSettings читает настройки бота из БД (единственная строка id=1).
func (r *PostgresRepository) GetSettings(ctx context.Context) (domain.Settings, error) {
	var (
		s          domain.Settings
		seconds    int
		categories string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT poll_interval_seconds, categories, updated_at FROM settings WHERE id = 1`).
		Scan(&seconds, &categories, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Settings{}, domain.ErrNoSettings
		}
		return domain.Settings{}, fmt.Errorf("ошибка чтения настроек: %w", err)
	}
	s.PollInterval = time.Duration(seconds) * time.Second
	if categories != "" {
		s.Categories = strings.Split(categories, ",")
	}
	return s, nil
}

// SaveSettings сохраняет настройки (upsert единственной строки id=1).
func (r *PostgresRepository) SaveSettings(ctx context.Context, s domain.Settings) error {
	seconds := int(s.PollInterval / time.Second)
	categories := strings.Join(s.Categories, ",")
	_, err := r.pool.Exec(ctx, `
		INSERT INTO settings (id, poll_interval_seconds, categories, updated_at)
		VALUES (1, $1, $2, NOW())
		ON CONFLICT (id) DO UPDATE SET
			poll_interval_seconds = EXCLUDED.poll_interval_seconds,
			categories = EXCLUDED.categories,
			updated_at = NOW()`, seconds, categories)
	if err != nil {
		return fmt.Errorf("ошибка сохранения настроек: %w", err)
	}
	return nil
}
