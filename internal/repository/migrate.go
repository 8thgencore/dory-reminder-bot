package repository

import (
	"database/sql"
)

func Migrate(db *sql.DB) error {
	// Create reminders table
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS reminders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		text TEXT NOT NULL,
		next_time DATETIME NOT NULL,
		repeat INTEGER NOT NULL,
		repeat_days TEXT,
		repeat_every INTEGER,
		paused BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		timezone TEXT
	);
	`)
	if err != nil {
		return err
	}

	// Create users table
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		chat_id INTEGER NOT NULL,
		username TEXT,
		first_name TEXT,
		last_name TEXT,
		timezone TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(id, chat_id)
	);
	`)
	return err
}
