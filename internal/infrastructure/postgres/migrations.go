package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations применяет миграции базы данных
// dbURL: строка подключения к БД (например, postgres://user:pass@host:port/dbname?sslmode=disable)
// migrationsPath: путь к директории с миграциями (например, "file://migrations")
func RunMigrations(dbURL string, migrationsPath string) error {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать golang-migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		// Если миграции уже применены, это не ошибка
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("не удалось применить миграции (Up): %w", err)
	}

	return nil
}
