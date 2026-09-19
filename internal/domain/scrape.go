package domain

// ScrapeResult — результат обхода сайта скрапером.
type ScrapeResult struct {
	// Lots — полностью распарсенные лоты (для сохранения и отправки).
	Lots []Lot
	// SeenIDs — ID ВСЕХ лотов, встреченных в листинге (даже если детали не распарсились).
	// Используется для сверки: чего нет в этом списке — то пропало с сайта.
	SeenIDs []string
	// Complete = true только если парсинг дошёл до конца листинга без ошибок страниц.
	// Если false — доверять SeenIDs для удаления НЕЛЬЗЯ (парсинг неполный).
	Complete bool
}

// SentMessageRef — ссылка на отправленное в Telegram сообщение (для удаления).
type SentMessageRef struct {
	ChatID    int64
	MessageID int
}
