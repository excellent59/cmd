package domain

import (
	"errors"
	"time"
)

// Сентинел-ошибки доменного уровня.
var (
	ErrLotNotFound = errors.New("lot not found")
	ErrNoSettings  = errors.New("settings not found")
)

// PriceHistory — запись из истории изменения цены лота.
type PriceHistory struct {
	OldPrice  string
	NewPrice  string
	ChangedAt time.Time
}

// DashboardStats — агрегаты для дашборда админки.
type DashboardStats struct {
	TotalLots    int
	SentLots     int
	UnsentLots   int
	PriceChanges int
	LastParse    *time.Time // nil, если лотов ещё нет
}

// LotQuery — фильтры и пагинация для списка лотов в админке.
type LotQuery struct {
	Page     int
	Limit    int
	Category string
	IsSent   *bool // nil = любой статус
}

// LotsPage — страница лотов с общим количеством для пагинации.
type LotsPage struct {
	Lots  []Lot
	Total int
	Page  int
	Limit int
}

// Settings — параметры бота, которые редактируются через админку и хранятся в БД.
type Settings struct {
	PollInterval time.Duration
	Categories   []string
	UpdatedAt    time.Time
}
