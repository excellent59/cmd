package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"pro-lot-bot/internal/config"
	httpDelivery "pro-lot-bot/internal/delivery/http"
	"pro-lot-bot/internal/delivery/scheduler"
	"pro-lot-bot/internal/delivery/scraper"
	"pro-lot-bot/internal/delivery/telegram"
	"pro-lot-bot/internal/domain"
	"pro-lot-bot/internal/infrastructure/postgres"
	"pro-lot-bot/internal/logger"
	"pro-lot-bot/internal/usecase"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Инициализация логгера
	logger.Init("development")
	defer logger.Sync()
	log := logger.Get()

	// 2. Загрузка .env
	if err := godotenv.Load(); err != nil {
		log.Warnw("Файл .env не найден, используются системные переменные", "error", err)
	}

	// 3. Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		log.Fatalw("Ошибка загрузки конфигурации", "error", err)
	}

	log.Infow("Конфигурация успешно загружена",
		"categories_count", len(cfg.Categories),
		"poll_interval", cfg.PollInterval,
	)

	// 4. Инициализация базы данных (Infrastructure)
	ctx := context.Background()
	log.Infow("Подключение к базе данных PostgreSQL...", "url", cfg.DatabaseURL)

	// Запуск миграций перед инициализацией репозитория
	if err := postgres.RunMigrations(cfg.DatabaseURL, "file://migrations"); err != nil {
		log.Fatalw("Ошибка выполнения миграций", "error", err)
	}
	log.Info("✅ Миграции базы данных успешно применены")

	repo, err := postgres.NewPostgresRepository(ctx, cfg.DatabaseURL, cfg.Categories)
	if err != nil {
		log.Fatalw("Критическая ошибка подключения к базе данных", "error", err)
	}
	defer repo.Close()

	log.Info("✅ Подключение к базе данных успешно установлено")

	// 5. Инициализация Telegram бота (Delivery)
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalw("Ошибка создания клиента Telegram", "error", err)
	}
	log.Infow("Бот успешно авторизован", "username", bot.Self.UserName)

	// 6. Инициализация скрапера (Delivery)
	webScraper := scraper.NewWebScraper(cfg.Categories)

	// 7. Инициализация бизнес-логики (UseCase)
	// UseCase не знает ни о postgres, ни о scraper, он знает только интерфейсы!
	uc := usecase.NewLotUsecase(webScraper, repo)

	// Инициализируем настройки в БД из .env, если их ещё нет
	// (дальше ими управляет админка, а планировщик читает их на лету).
	if _, err := uc.GetSettings(ctx); err != nil {
		if err := uc.SaveSettings(ctx, domain.Settings{
			PollInterval: cfg.PollInterval,
			Categories:   cfg.Categories,
		}); err != nil {
			log.Warnw("Не удалось инициализировать настройки в БД", "error", err)
		}
	}

	// Актуальный интервал берём из БД (мог быть изменён через админку).
	pollInterval := cfg.PollInterval
	if st, err := uc.GetSettings(ctx); err == nil && st.PollInterval > 0 {
		pollInterval = st.PollInterval
	}

	// 8. Инициализация обработчика и планировщика (Delivery)
	handler := telegram.NewBotHandler(bot, uc, cfg.TelegramChatID)
	sched := scheduler.NewScheduler(handler, uc, pollInterval)

	log.Info("🚀 Запуск бота pro-lot...")

	httpRouter := httpDelivery.NewRouter(uc, cfg)

	httpServer := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      httpRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Infow("🌐 HTTP API запущен", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("Ошибка HTTP сервера", "error", err)
		}
	}()

	// 9. Graceful Shutdown
	sigCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Warn("🛑 Получен сигнал завершения. Начинаем безопасную остановку...")
		cancel()

		// Останавливаем HTTP сервер
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	// 10. Запуск
	sched.Start(sigCtx)

}
