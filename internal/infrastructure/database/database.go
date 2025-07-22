package database

import (
	"database/sql"
	"log/slog"
	"os"

	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

// InitDatabase инициализирует базу данных и выполняет миграции
func InitDatabase(log *slog.Logger) (*sql.DB, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0o750); err != nil {
		log.Error("Failed to create data directory", "error", err)
		return nil, err
	}
	log.Info("Data directory created/verified", "path", "data")

	// Open database connection
	db, err := sql.Open("sqlite3", "data/reminders.db")
	if err != nil {
		log.Error("Failed to open database", "error", err)
		return nil, err
	}

	log.Info("Database connection opened")

	// Migrate schema
	if err := repository.Migrate(db); err != nil {
		log.Error("Failed to migrate database", "error", err)
		return nil, err
	}
	log.Info("Database migration completed")

	return db, nil
}

// CloseDatabase закрывает соединение с базой данных
func CloseDatabase(db *sql.DB, log *slog.Logger) {
	if err := db.Close(); err != nil {
		log.Error("Failed to close database", "error", err)
	}
}
