package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool       *pgxpool.Pool
	categories []string
}

func NewPostgresRepository(ctx context.Context, databaseURL string, categories []string) (*PostgresRepository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга URL базы данных: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания пула соединений: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	repo := &PostgresRepository{
		pool:       pool,
		categories: categories,
	}

	// Миграции теперь должны запускаться извне (например, при старте приложения).
	// Мы убираем createTables, так как за структуру БД теперь отвечает golang-migrate.
	// if err := repo.createTables(ctx); err != nil {
	// 	return nil, fmt.Errorf("ошибка создания таблиц: %w", err)
	// }

	return repo, nil
}

func (r *PostgresRepository) GetCategories() []string {
	return r.categories
}

func (r *PostgresRepository) HasLot(ctx context.Context, lotID string) bool {
	log := logger.Get()
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM lots WHERE id = $1)"
	err := r.pool.QueryRow(ctx, query, lotID).Scan(&exists)
	if err != nil {
		log.Errorw("Ошибка проверки существования лота", "lot_id", lotID, "error", err)
		return false
	}
	return exists
}

func (r *PostgresRepository) GetLotPrice(ctx context.Context, lotID string) (string, bool) {
	var price string
	query := "SELECT current_price FROM lots WHERE id = $1"
	err := r.pool.QueryRow(ctx, query, lotID).Scan(&price)
	if err != nil {
		return "", false
	}
	return price, true
}

func (r *PostgresRepository) AddLot(ctx context.Context, lot domain.Lot) error {
	log := logger.Get()

	// Очищаем и ОБРЕЗАЕМ текстовые поля
	lot.Title = truncateString(strings.ToValidUTF8(lot.Title, ""), 1000)
	lot.URL = truncateString(strings.ToValidUTF8(lot.URL, ""), 1000)
	lot.Price = strings.ToValidUTF8(lot.Price, "")
	lot.Location = strings.ToValidUTF8(lot.Location, "")
	lot.PriceTrend = strings.ToValidUTF8(lot.PriceTrend, "")

	// Преобразуем массив изображений в формат JSON для базы данных
	imagesJSON := "[]"
	if len(lot.Images) > 0 {
		bytes, err := json.Marshal(lot.Images)
		if err == nil {
			imagesJSON = string(bytes)
		}
	}

	// ДОБАВЛЕНО поле images ($8)
	query := `
		INSERT INTO lots (id, url, title, year, current_price, location, price_trend, images, is_sent, last_checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING`

	_, err := r.pool.Exec(ctx, query,
		lot.ID,         // $1
		lot.URL,        // $2
		lot.Title,      // $3
		lot.Year,       // $4
		lot.Price,      // $5
		lot.Location,   // $6
		lot.PriceTrend, // $7
		imagesJSON,     // $8 <-- КАРТИНКИ
		false,          // $9 (is_sent)
		lot.CreatedAt,  // $10
	)

	if err != nil {
		log.Errorw("Ошибка добавления лота в базу данных", "lot_id", lot.ID, "error", err)
		return fmt.Errorf("ошибка добавления лота: %w", err)
	}

	log.Infow("Лот успешно добавлен в базу данных",
		"lot_id", lot.ID,
		"title", lot.Title,
		"year", lot.Year,
		"price", lot.Price,
		"images_count", len(lot.Images), // <-- Добавили для проверки
	)
	return nil
}
func (r *PostgresRepository) UpdateLotPrice(ctx context.Context, lotID, price string) error {
	log := logger.Get()

	price = strings.ToValidUTF8(price, "")

	oldPrice, exists := r.GetLotPrice(ctx, lotID)
	if !exists {
		log.Errorw("Не удалось обновить цену: лот не найден", "lot_id", lotID)
		return fmt.Errorf("лот %s не найден", lotID)
	}

	// is_sent = false, чтобы бот переслал лот с уведомлением об изменении цены.
	// previous_price сохраняем, чтобы показать в сообщении «старая/новая цена».
	query := `UPDATE lots SET current_price = $1, previous_price = $2, is_sent = false, updated_at = NOW() WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, price, oldPrice, lotID)
	if err != nil {
		log.Errorw("Ошибка обновления цены лота", "lot_id", lotID, "error", err)
		return fmt.Errorf("ошибка обновления цены: %w", err)
	}

	historyQuery := `
		INSERT INTO price_histories (lot_id, old_price, new_price)
		VALUES ($1, $2, $3)`

	_, err = r.pool.Exec(ctx, historyQuery, lotID, oldPrice, price)
	if err != nil {
		log.Errorw("Ошибка добавления записи в историю цен", "lot_id", lotID, "error", err)
	}

	log.Infow("Цена лота обновлена",
		"lot_id", lotID,
		"old_price", oldPrice,
		"new_price", price,
	)
	return nil
}

func (r *PostgresRepository) Close() {
	logger.Get().Info("Подключение к базе данных закрыто")
	r.pool.Close()
}

// truncateString безопасно обрезает строку до maxLen символов, не разрезая UTF-8
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// GetUnsentLots возвращает все лоты, которые еще не были успешно отправлены
func (r *PostgresRepository) GetUnsentLots(ctx context.Context) ([]domain.Lot, error) {
	log := logger.Get()

	// ДОБАВЛЕНО поле images в выборку
	query := `
		SELECT id, url, title, year, current_price, COALESCE(previous_price, ''), location, price_trend, images, last_checked_at, created_at
		FROM lots 
		WHERE is_sent = false 
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		log.Errorw("Ошибка получения неотправленных лотов", "error", err)
		return nil, err
	}
	defer rows.Close()

	var lots []domain.Lot
	for rows.Next() {
		var lot domain.Lot
		var imagesJSON string       // Временная переменная для чтения JSON из БД
		var lastCheckedAt time.Time // last_checked_at читаем отдельно (в domain.Lot поля нет)

		// ДОБАВЛЕНО чтение imagesJSON
		err := rows.Scan(
			&lot.ID, &lot.URL, &lot.Title, &lot.Year, &lot.Price, &lot.PreviousPrice,
			&lot.Location, &lot.PriceTrend, &imagesJSON, &lastCheckedAt, &lot.CreatedAt,
		)
		if err != nil {
			log.Errorw("Ошибка сканирования лота", "error", err)
			continue
		}

		// Преобразуем JSON обратно в массив строк
		if imagesJSON != "" && imagesJSON != "null" {
			json.Unmarshal([]byte(imagesJSON), &lot.Images)
		}

		lot.IsSent = false
		lots = append(lots, lot)
	}

	return lots, rows.Err()
}
