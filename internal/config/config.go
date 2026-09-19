package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken  string
	TelegramChatID int64
	Categories     []string
	PollInterval   time.Duration
	DatabaseURL    string

	AdminPassword string
	JWTSecret     string
	HTTPPort      string
}

func Load() (*Config, error) {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is not set")
	}

	telegramChatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid TELEGRAM_CHAT_ID: %w", err)
	}

	categoriesStr := os.Getenv("CATEGORIES")
	if categoriesStr == "" {
		categoriesStr = "2,13,14,15,16,17,18"
	}
	categories := strings.Split(categoriesStr, ",")

	pollIntervalMinutes := 5
	if intervalStr := os.Getenv("POLL_INTERVAL_MINUTES"); intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil {
			pollIntervalMinutes = interval
		}
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/prolotbot?sslmode=disable"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin123" // дефолт только для локальной разработки
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = ":8080"
	}

	return &Config{
		TelegramToken:  telegramToken,
		TelegramChatID: telegramChatID,
		Categories:     categories,
		PollInterval:   time.Duration(pollIntervalMinutes) * time.Minute,
		DatabaseURL:    databaseURL,
		AdminPassword:  adminPassword,
		JWTSecret:      jwtSecret,
		HTTPPort:       httpPort,
	}, nil
}
