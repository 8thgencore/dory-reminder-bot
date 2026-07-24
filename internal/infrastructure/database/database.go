package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"

	// SQLite driver registration
	_ "github.com/mattn/go-sqlite3"
)

// dsnParams — прагмы соединения SQLite.
//
// _journal_mode=WAL позволяет читать во время записи: HTTP-слой Mini App работает с той же
// базой параллельно с планировщиком. _busy_timeout заставляет драйвер подождать вместо
// немедленного "database is locked".
//
// _loc=UTC критичен: go-sqlite3 сериализует time.Time в строку со смещением, а сравнение
// next_time <= ? лексикографическое. Без единой зоны due-напоминания выпадают из выборки.
const dsnParams = "_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_loc=UTC&_synchronous=NORMAL"

// InitDatabase инициализирует базу данных и выполняет миграции
func InitDatabase(cfg config.DatabaseConfig, log *slog.Logger) (*sql.DB, error) {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Error("Failed to create data directory", "path", dir, "error", err)
		return nil, fmt.Errorf("create data directory %s: %w", dir, err)
	}
	log.Info("Data directory created/verified", "path", dir)

	db, err := sql.Open("sqlite3", cfg.Path+"?"+dsnParams)
	if err != nil {
		log.Error("Failed to open database", "path", cfg.Path, "error", err)
		return nil, fmt.Errorf("open database %s: %w", cfg.Path, err)
	}

	// go-sqlite3 не сериализует запись: единственное соединение убирает "database is locked"
	// между планировщиком и HTTP-хендлерами ценой сериализации запросов, что для этой
	// нагрузки несущественно.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		log.Error("Failed to connect to database", "path", cfg.Path, "error", err)

		return nil, fmt.Errorf("ping database: %w", err)
	}
	log.Info("Database connection opened", "path", cfg.Path)

	if err := repository.Migrate(db); err != nil {
		log.Error("Failed to migrate database", "error", err)
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	log.Info("Database migration completed")

	return db, nil
}

// CloseDatabase закрывает соединение с базой данных
func CloseDatabase(db *sql.DB, log *slog.Logger) {
	if err := db.Close(); err != nil {
		log.Error("Failed to close database", "error", err)

		return
	}
	log.Info("Database connection closed")
}
