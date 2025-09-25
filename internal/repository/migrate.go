package repository

import (
	"database/sql"
)

// Migrate выполняет миграцию схемы БД для напоминаний и пользователей.
func Migrate(db *sql.DB) error {
	// Create reminders table
	_, err := db.Exec(`
    CREATE TABLE IF NOT EXISTS reminders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        chat_id INTEGER NOT NULL,
        text TEXT NOT NULL,
        next_time DATETIME NOT NULL,
        repeat INTEGER NOT NULL,
        repeat_days TEXT,
        repeat_every INTEGER,
        paused BOOLEAN NOT NULL DEFAULT 0,
        created_at DATETIME NOT NULL,
        updated_at DATETIME NOT NULL
    );
    `)
	if err != nil {
		return err
	}

	// Create chats table (unified model)
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS chats (
        chat_id INTEGER PRIMARY KEY,
        type TEXT NOT NULL,
        name TEXT,
        username TEXT,
        timezone TEXT,
        created_at DATETIME NOT NULL,
        updated_at DATETIME NOT NULL
    );
    `)
	if err != nil {
		return err
	}

	// Drop legacy users table if exists
	if _, err := db.Exec(`DROP TABLE IF EXISTS users;`); err != nil {
		return err
	}

	return nil
}
