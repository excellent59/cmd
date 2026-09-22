package telegram

import (
	"fmt"
	"pro-lot-bot/internal/domain"
	"strings"
)

func FormatLotMessage(lot domain.Lot) string {
	msg := ""

	// 1. Описание (Марка + VIN)
	desc := lot.Title
	if desc == "" {
		desc = "Описание отсутствует"
	}

	msg += fmt.Sprintf("📝 Описание\n%s\n", desc)
	// 2. Год выпуска - ОТДЕЛЬНОЙ СТРОКОЙ
	if lot.Year > 0 {
		msg += fmt.Sprintf("📅 Год выпуска: %d\n", lot.Year)
	}
	// 2. Местонахождение
	loc := lot.Location
	if loc != "" {
		if strings.Contains(loc, "🌍 Местонахождение:") {
			msg += fmt.Sprintf("%s\n", loc)
		} else {
			msg += fmt.Sprintf("🌍 Местонахождение: %s\n", loc)
		}
	} else {
		msg += "🌍 Местонахождение: не указано\n"
	}

	// 3. Цена
	price := lot.Price
	if lot.PreviousPrice != "" {
		// Цена изменилась — показываем «было/стало» двумя строками.
		newPrice := price
		if newPrice == "" {
			newPrice = "не указана"
		}
		msg += fmt.Sprintf("<s>💰 Старая цена - %s</s>\n", lot.PreviousPrice)
		msg += fmt.Sprintf("<b>💰 Новая цена - %s</b>\n", newPrice)
	} else if price != "" {
		msg += fmt.Sprintf("\n💰 Цена - %s\n", price)
	} else {
		msg += "\n💰 Цена - не указана\n"
	}

	// 4. Умное предупреждение о цене
	msg += "\n"
	switch lot.PriceTrend {
	case "up":
		msg += "🔴 Цена может стать выше 🔴\n"
	case "down":
		msg += "🟢 Цена понижается, предложите свою цену 🟢\n"
	default:
		msg += "⚠️ Следите за изменением цены\n"
	}

	msg += "🔔 Подача заявки 2 т.р.+комиссия"

	// Финальная проверка длины
	if len(msg) > 1020 {
		msg = msg[:1017] + "..."
	}

	return msg
}
