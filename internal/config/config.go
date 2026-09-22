package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	// required означает, что без этой переменной бот не запустится
	TelegramToken  string `env:"TELEGRAM_TOKEN,required"`
	TelegramChatID int64  `env:"TELEGRAM_CHAT_ID,required"`
	JWTSecret      string `env:"JWT_SECRET,required"`

	// envDefault подставляет значение, если переменной нет в .env
	Categories    []string      `env:"CATEGORIES" envDefault:"2,13,14,15,16,17,18" envSeparator:","`
	PollInterval  time.Duration `env:"POLL_INTERVAL_MINUTES" envDefault:"15"`
	DatabaseURL   string        `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/prolotbot?sslmode=disable"`
	AdminPassword string        `env:"ADMIN_PASSWORD" envDefault:"admin123"`
	HTTPPort      string        `env:"HTTP_PORT" envDefault:":8080"`
}

// Небольшая коррекция: caarlos0/env парсит time.Duration из строк вида "15m", "1h".
// Если у тебя в .env написано просто число "15", лучше явно указать это в коде после парсинга:
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Хак: если PollInterval пришел как 15 (наносекунд), значит в .env было просто число.
	// Умножаем на минуту.
	if cfg.PollInterval < time.Minute {
		cfg.PollInterval = cfg.PollInterval * time.Minute
	}

	return cfg, nil
}
