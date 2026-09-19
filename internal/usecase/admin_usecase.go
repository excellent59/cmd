package usecase

import (
	"context"

	"pro-lot-bot/internal/domain"
)

// Методы для админ-панели — тонкие делегаты к репозиторию.
// Бизнес-логики тут нет: usecase лишь предоставляет доступ через свои интерфейсы.

func (u *LotUsecase) GetDashboardStats(ctx context.Context) (domain.DashboardStats, error) {
	return u.repository.GetDashboardStats(ctx)
}

func (u *LotUsecase) GetLotsPage(ctx context.Context, q domain.LotQuery) (domain.LotsPage, error) {
	return u.repository.GetLotsPage(ctx, q)
}

func (u *LotUsecase) GetLotByID(ctx context.Context, id string) (domain.Lot, error) {
	return u.repository.GetLotByID(ctx, id)
}

func (u *LotUsecase) DeleteLot(ctx context.Context, id string) error {
	return u.repository.DeleteLot(ctx, id)
}

func (u *LotUsecase) GetPriceHistory(ctx context.Context, id string) ([]domain.PriceHistory, error) {
	return u.repository.GetPriceHistory(ctx, id)
}

func (u *LotUsecase) GetSettings(ctx context.Context) (domain.Settings, error) {
	return u.repository.GetSettings(ctx)
}

func (u *LotUsecase) SaveSettings(ctx context.Context, s domain.Settings) error {
	return u.repository.SaveSettings(ctx, s)
}
