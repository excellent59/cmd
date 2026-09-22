package scraper

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/logger"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gocolly/colly/v2"
)

type WebScraper struct {
	client     *http.Client
	categories []string
}

func NewWebScraper(categories []string) *WebScraper {
	return &WebScraper{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		categories: categories,
	}
}

// extractBackgroundImage безопасно извлекает URL из style="background-image: url('...')"
func extractBackgroundImage(style string) string {
	if !strings.Contains(style, "url(") {
		return ""
	}
	start := strings.Index(style, "url('")
	quote := "'"
	if start == -1 {
		start = strings.Index(style, `url("`)
		quote = `"`
	}
	if start == -1 {
		return ""
	}
	contentStart := start + 5
	if contentStart >= len(style) {
		return ""
	}
	end := strings.Index(style[contentStart:], quote)
	if end == -1 {
		return ""
	}
	url := strings.TrimSpace(style[contentStart : contentStart+end])
	if url == "" {
		return ""
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://pro-lot.ru" + url
	}
	return url
}

func cleanText(s string) string {
	// УДАЛЯЕМ невалидные UTF-8 символы (это решает проблему с PostgreSQL)
	s = strings.ToValidUTF8(s, "")

	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func extractPrice(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)

	// Удаляем "Текущая цена" и подобные слова
	s = strings.ReplaceAll(s, "Текущая цена", "")
	s = strings.TrimSpace(s)

	// Ищем все цифры, пробелы, запятые, точки и символы валют
	var priceChars []rune
	for _, r := range s {
		if unicode.IsDigit(r) || r == ' ' || r == '₽' || r == 'р' || r == 'Р' || r == '.' || r == ',' {
			priceChars = append(priceChars, r)
		} else if len(priceChars) > 0 {
			// Если встретили другой символ после начала цены, останавливаемся
			break
		}
	}

	cleaned := strings.TrimSpace(string(priceChars))

	// Убираем лишние пробелы
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}

	return cleaned
}

// extractStructuredData извлекает Марку/VIN (или номер кузова) и Местонахождение
func extractStructuredData(fullPageText string) (string, string) {
	lines := strings.Split(fullPageText, "\n")

	markInfo := ""
	location := ""
	foundDescHeader := false

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 1. Ищем Местонахождение
		if strings.Contains(line, "Местонахождение:") {
			parts := strings.SplitN(line, "Местонахождение:", 2)
			if len(parts) > 1 {
				location = strings.TrimSpace(parts[1])
			}
		}

		// 2. Ищем Описание и VIN / номер кузова
		if strings.ToLower(line) == "описание" {
			foundDescHeader = true
			continue
		}

		if foundDescHeader && markInfo == "" && len(line) > 10 {
			vinIndex := strings.Index(line, "VIN")
			if vinIndex != -1 {
				prefix := line[:vinIndex]
				afterVin := line[vinIndex:]

				vinCodeStart := 0
				for j, char := range afterVin {
					if j > 3 {
						if unicode.IsUpper(char) || unicode.IsDigit(char) {
							vinCodeStart = j
							break
						}
					}
				}

				vinCode := afterVin[vinCodeStart:]
				if len(vinCode) >= 18 {
					vinCode = vinCode[:18]
				}

				markInfo = cleanText(prefix) + " VIN " + vinCode
			} else {
				bodyIndex := strings.Index(strings.ToLower(line), "номер кузова")
				if bodyIndex != -1 {
					prefix := cleanText(line[:bodyIndex])

					if i+1 < len(lines) {
						nextLine := strings.TrimSpace(lines[i+1])
						if nextLine != "" &&
							!strings.Contains(strings.ToLower(nextLine), "обременение") &&
							!strings.Contains(strings.ToLower(nextLine), "сведения") &&
							!strings.Contains(strings.ToLower(nextLine), "банк") {
							markInfo = prefix + " номер кузова " + nextLine
						} else {
							markInfo = prefix
						}
					} else {
						markInfo = prefix
					}
				} else {
					markInfo = line
				}
			}
			foundDescHeader = false
		}
	}

	// Fallback: если не нашли по заголовку
	if markInfo == "" {
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 20 && !strings.Contains(line, "Местонахождение") {
				vinIndex := strings.Index(line, "VIN")
				if vinIndex != -1 {
					prefix := line[:vinIndex]
					vinPart := line[vinIndex:]
					targetLen := 4 + 18
					if len(vinPart) >= targetLen {
						markInfo = cleanText(prefix) + " " + vinPart[:targetLen]
					} else {
						markInfo = line
					}
				} else {
					bodyIndex := strings.Index(strings.ToLower(line), "номер кузова")
					if bodyIndex != -1 {
						prefix := cleanText(line[:bodyIndex])
						if i+1 < len(lines) {
							nextLine := strings.TrimSpace(lines[i+1])
							if nextLine != "" && !strings.Contains(strings.ToLower(nextLine), "обременение") {
								markInfo = prefix + " номер кузова " + nextLine
							} else {
								markInfo = prefix
							}
						} else {
							markInfo = prefix
						}
					} else {
						markInfo = line
					}
				}
				break
			}
		}
	}

	return cleanText(markInfo), cleanText(location)
}

func extractYear(text string) int {
	// Ищем 4 цифры (от 1950 до 2029), за которыми может следовать "г.в.", "г.в", "год" или "года"
	// \b гарантирует, что мы ищем целое слово, а не часть VIN-номера
	re := regexp.MustCompile(`\b(19[5-9]\d|20[0-2]\d)\s*(?:г\.?\s*в\.?|года?)?\b`)

	// Ищем все совпадения в тексте
	matches := re.FindAllStringSubmatch(text, -1)

	// Берем первое найденное совпадение (обычно это и есть год выпуска)
	for _, match := range matches {
		if len(match) >= 2 {
			year, err := strconv.Atoi(match[1])
			if err == nil && year >= 1950 && year <= 2030 {
				return year
			}
		}
	}

	return 0
}

// extractLotID достаёт ID лота из ссылки вида .../lot?id=123&...
// Если id в ссылке нет — детерминированный хэш от ссылки (запасной вариант).
func extractLotID(link string) string {
	parts := strings.Split(link, "id=")
	if len(parts) > 1 {
		return strings.Split(parts[1], "&")[0]
	}
	hash := sha256.Sum256([]byte(link))
	return fmt.Sprintf("%x", hash)[:16]
}

func (w *WebScraper) GetLots(ctx context.Context, filter domain.LotFilter) (domain.ScrapeResult, error) {
	log := logger.Get()
	var lots []domain.Lot
	seen := make(map[string]bool) // защита от дублей ID между страницами
	var seenIDs []string          // ID всех лотов из листинга (для сверки/удаления)
	reachedEnd := false           // дошли до конца листинга
	anyPageError := false         // была ошибка загрузки хотя бы одной страницы
	catIDs := strings.Join(filter.CategoryIDs, ",")

	// Определяем, сколько страниц парсить
	maxPages := filter.MaxPages
	if maxPages == 0 {
		maxPages = 2 // По умолчанию 2 страницы
	}

	log.Infow("Начинаю парсинг страниц",
		"pages", maxPages,
		"categories", filter.CategoryIDs,
	)

	// Парсим каждую страницу
	for page := 1; page <= maxPages; page++ {
		var url string
		if page == 1 {
			url = fmt.Sprintf("https://pro-lot.ru/lots?cat_id=%s", catIDs)
		} else {
			url = fmt.Sprintf("https://pro-lot.ru/lots?page=%d&cat_id=%s", page, catIDs)
		}

		log.Infow("Парсинг страницы",
			"page", page,
			"total_pages", maxPages,
			"url", url,
		)

		c := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		)

		// 1. Собираем ссылки на лоты с текущей страницы
		var lotLinks []string
		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			href := e.Attr("href")
			if href != "" && strings.Contains(href, "lot?id=") {
				if !strings.HasPrefix(href, "http") {
					href = "https://pro-lot.ru" + href
				}
				// Проверяем, нет ли уже такой ссылки (защита от дублей)
				exists := false
				for _, link := range lotLinks {
					if link == href {
						exists = true
						break
					}
				}
				if !exists {
					lotLinks = append(lotLinks, href)
				}
			}
		})

		err := c.Visit(url)
		if err != nil {
			log.Errorw("Ошибка парсинга страницы",
				"page", page,
				"url", url,
				"error", err,
			)
			anyPageError = true // парсинг неполный — удалять лоты в этом цикле нельзя
			continue            // Продолжаем со следующей страницей
		}

		log.Infow("Страница распарсена",
			"page", page,
			"links_found", len(lotLinks),
		)

		// Считаем ID лотов на странице и решаем, не дошли ли до конца листинга.
		newOnPage := 0
		for _, link := range lotLinks {
			id := extractLotID(link)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			seenIDs = append(seenIDs, id)
			newOnPage++
		}

		// Пустая страница ИЛИ ни одного НОВОГО лота (сайт «зациклил» пагинацию на
		// последней странице) — это конец листинга.
		if len(lotLinks) == 0 || newOnPage == 0 {
			log.Infow("Достигнут конец листинга", "page", page)
			reachedEnd = true
			break
		}

		// Применяем лимит, если задан
		if filter.Limit > 0 && len(lotLinks) > filter.Limit {
			log.Warnw("Активирован лимит лотов",
				"limit", filter.Limit,
				"found", len(lotLinks),
			)
			lotLinks = lotLinks[:filter.Limit]
		}

		// 2. Парсим детали каждого лота
		for _, link := range lotLinks {
			tempLot := domain.Lot{URL: link}
			tempLot.ID = extractLotID(link)

			// Свежий коллектор на каждый лот: один обработчик = один лот (без накопления O(n^2))
			detailCollector := colly.NewCollector(
				colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
			)

			detailCollector.OnHTML("body", func(e *colly.HTMLElement) {
				// === ЦЕНА И ТРЕНД ===
				priceText := ""
				trend := ""

				e.ForEach("div.card-price__value", func(_ int, el *colly.HTMLElement) {
					priceText = el.Text

					el.ForEach("div[class*='arrow']", func(_ int, arrowDiv *colly.HTMLElement) {
						arrowClass := strings.ToLower(arrowDiv.Attr("class"))
						if strings.Contains(arrowClass, "auction") || strings.Contains(arrowClass, "up") {
							trend = "up"
						} else if strings.Contains(arrowClass, "public") || strings.Contains(arrowClass, "down") {
							trend = "down"
						}
					})
				})

				if priceText == "" {
					e.ForEach(".price, .lot_price", func(_ int, el *colly.HTMLElement) {
						if priceText == "" {
							priceText = el.Text
						}
					})
				}

				tempLot.Price = extractPrice(priceText)
				tempLot.PriceTrend = trend

				// === ОПИСАНИЕ И ЛОКАЦИЯ ===
				fullText := e.Text

				location := ""

				// Пробуем разные селекторы для поиска локации
				locationSelectors := []string{
					"div.region_lot",
					"[class*='region']",
					"[class*='city']",
					".lot_location",
					".location",
					"div[class*='location']",
				}

				for _, selector := range locationSelectors {
					if location == "" {
						e.ForEach(selector, func(_ int, el *colly.HTMLElement) {
							if location == "" {
								locText := strings.TrimSpace(el.Text)

								// Фильтруем мусор
								if locText != "" &&
									len(locText) < 100 &&
									!strings.Contains(locText, "Главная") &&
									!strings.Contains(locText, "Размещено") &&
									!strings.Contains(locText, "Просмотры") {
									location = locText

								}
							}
						})
					}
				}

				// Если не нашли в явных блоках, пробуем извлечь из описания
				if location == "" {
					_, locFromText := extractStructuredData(fullText)
					if locFromText != "" {
						location = locFromText

					} else {
						log.Warnw("Локация не найдена",
							"lot_id", tempLot.ID,
						)
					}
				}

				tempLot.Location = location

				markInfo, _ := extractStructuredData(fullText)
				tempLot.Title = markInfo

				tempLot.Year = extractYear(fullText)

				// === ФОТО ===
				galleryFound := false
				e.ForEach("div[class*='slider'], div[class*='gallery'], div.thumbnails, div.images", func(_ int, gallery *colly.HTMLElement) {
					imgCount := 0
					gallery.ForEach("img", func(_ int, img *colly.HTMLElement) {
						imgCount++
					})

					if imgCount > 1 {
						galleryFound = true
						gallery.ForEach("img", func(_ int, img *colly.HTMLElement) {
							src := img.Attr("src")
							if src != "" && !strings.Contains(src, "logo") && !strings.Contains(src, "icon") && !strings.Contains(src, "sprite") {
								if !strings.HasPrefix(src, "http") {
									src = "https://pro-lot.ru" + src
								}
								tempLot.Images = append(tempLot.Images, src)
							}
						})
					}
				})

				if !galleryFound {
					e.ForEach("img", func(_ int, img *colly.HTMLElement) {
						src := img.Attr("src")
						isLikelyMainPhoto := (src != "" && !strings.Contains(src, "logo") && !strings.Contains(src, "icon") && !strings.Contains(src, "sprite"))
						if isLikelyMainPhoto && len(tempLot.Images) == 0 {
							if !strings.HasPrefix(src, "http") {
								src = "https://pro-lot.ru" + src
							}
							tempLot.Images = append(tempLot.Images, src)
						}
					})
				}
			})

			// log.Debugw("Парсинг лота",
			// 	"lot_id", tempLot.ID,
			// 	"url", link,
			// )
			err := detailCollector.Visit(link)
			if err != nil {
				log.Errorw("Ошибка парсинга лота",
					"lot_id", tempLot.ID,
					"url", link,
					"error", err,
				)
				continue
			}

			if tempLot.Title != "" || tempLot.Price != "" {
				lots = append(lots, tempLot)
				// 	log.Infow("Лот успешно распарсен",
				// 		"lot_id", tempLot.ID,
				// 		"title", tempLot.Title,
				// 		"price", tempLot.Price,
				// 		"location", tempLot.Location,
				// 		"images_count", len(tempLot.Images),
				// 	)
			}

			select {
			case <-ctx.Done():
				return domain.ScrapeResult{Lots: lots, SeenIDs: seenIDs, Complete: false}, ctx.Err()
			case <-time.After(2 * time.Second): // Пауза между лотами
			}
		}

		select {
		case <-ctx.Done():
			return domain.ScrapeResult{Lots: lots, SeenIDs: seenIDs, Complete: false}, ctx.Err()
		case <-time.After(3 * time.Second): // Пауза между страницами
		}
	}

	complete := reachedEnd && !anyPageError
	log.Infow("Парсинг завершен",
		"total_lots", len(lots),
		"seen_ids", len(seenIDs),
		"reached_end", reachedEnd,
		"complete", complete,
	)
	return domain.ScrapeResult{Lots: lots, SeenIDs: seenIDs, Complete: complete}, nil
}
