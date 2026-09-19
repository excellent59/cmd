package domain

import (
	"time"
)

type Lot struct {
	ID    string
	Title string
	Price string
	// PreviousPrice заполняется только когда цена изменилась и лот пересылается
	// с уведомлением об изменении. Для новых лотов пустая строка.
	PreviousPrice string
	Location      string
	Year          int
	Images        []string
	URL           string
	CategoryID    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Views         int
	PostedAt      string
	PriceTrend    string
	IsSent        bool
}

type LotFilter struct {
	CategoryIDs []string
	Limit       int
	MaxPages    int
}
