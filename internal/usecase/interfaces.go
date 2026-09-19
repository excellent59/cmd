package usecase

import (
	"context"
	"pro-lot-bot/internal/domain"
)

// LotScraper - интерфейс скрапинг
type LotScraper interface {
	GetLots(ctx context.Context, filter domain.LotFilter) (domain.ScrapeResult, error)
}

// LotRepository - интерфейс база данных
type LotRepository interface {
	GetCategories() []string
	HasLot(ctx context.Context, lotID string) bool
	GetLotPrice(ctx context.Context, lotID string) (string, bool)
	AddLot(ctx context.Context, lot domain.Lot) error
	UpdateLotPrice(ctx context.Context, lotID, price string) error
	Close()
	GetUnsentLots(ctx context.Context) ([]domain.Lot, error)

	// Фиксация отправки и удаление снятых с сайта лотов
	MarkLotSent(ctx context.Context, lotID string, chatID int64, messageIDs []int) error
	DeleteRemovedLots(ctx context.Context, seenIDs []string) ([]domain.SentMessageRef, []string, error)

	// Методы для админ-панели
	GetDashboardStats(ctx context.Context) (domain.DashboardStats, error)
	GetLotsPage(ctx context.Context, q domain.LotQuery) (domain.LotsPage, error)
	GetLotByID(ctx context.Context, id string) (domain.Lot, error)
	DeleteLot(ctx context.Context, id string) error
	GetPriceHistory(ctx context.Context, id string) ([]domain.PriceHistory, error)
	GetSettings(ctx context.Context) (domain.Settings, error)
	SaveSettings(ctx context.Context, s domain.Settings) error
}
