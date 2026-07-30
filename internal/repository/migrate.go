package repository

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// migration — один шаг схемы. Version должен строго возрастать, а Statements —
// быть идемпотентными настолько, насколько это позволяет SQLite.
type migration struct {
	Version int
	Name    string
	Stmts   []string
}

// migrations — упорядоченный список миграций схемы.
//
// Никогда не меняйте и не удаляйте уже выпущенную миграцию: на существующих базах она
// не выполнится повторно. Вместо этого добавляйте новую в конец.
var migrations = []migration{
	{
		Version: 1,
		Name:    "initial schema",
		Stmts: []string{
			`CREATE TABLE IF NOT EXISTS reminders (
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
            )`,
			`CREATE TABLE IF NOT EXISTS chats (
                chat_id INTEGER PRIMARY KEY,
                type TEXT NOT NULL,
                name TEXT,
                username TEXT,
                timezone TEXT,
                created_at DATETIME NOT NULL,
                updated_at DATETIME NOT NULL
            )`,
			// Пользователи объединены с чатами: в личном чате chat_id совпадает с user_id.
			`DROP TABLE IF EXISTS users`,
		},
	},
	{
		Version: 2,
		Name:    "reminder indexes",
		Stmts: []string{
			// Планировщик опрашивает due-напоминания каждые 30 секунд — без индекса это full scan.
			`CREATE INDEX IF NOT EXISTS idx_reminders_due ON reminders(next_time) WHERE paused = 0`,
			// Список напоминаний чата всегда сортируется по времени срабатывания.
			`CREATE INDEX IF NOT EXISTS idx_reminders_chat ON reminders(chat_id, next_time)`,
		},
	},
	{
		Version: 3,
		Name:    "clear polluted repeat_every",
		Stmts: []string{
			// Мастер добавления писал в repeat_every день недели (week) и число месяца (month).
			// Настоящее значение уже лежит в repeat_days, поэтому мусор просто обнуляем.
			`UPDATE reminders SET repeat_every = 0 WHERE repeat IN (2, 3)`,
		},
	},
	{
		Version: 4,
		Name:    "chat members",
		Stmts: []string{
			// Нужна, чтобы Mini App показал пользователю список его чатов. Права на каждый
			// чат всё равно перепроверяются живым вызовом getChatMember.
			`CREATE TABLE IF NOT EXISTS chat_members (
                chat_id INTEGER NOT NULL,
                user_id INTEGER NOT NULL,
                last_seen DATETIME NOT NULL,
                PRIMARY KEY (chat_id, user_id)
            )`,
			`CREATE INDEX IF NOT EXISTS idx_chat_members_user ON chat_members(user_id)`,
		},
	},
	{
		Version: 5,
		Name:    "webapp launch context",
		Stmts: []string{
			`CREATE TABLE IF NOT EXISTS webapp_launch_contexts (
                user_id INTEGER PRIMARY KEY,
                chat_id INTEGER NOT NULL,
                launched_at DATETIME NOT NULL
            )`,
			`CREATE INDEX IF NOT EXISTS idx_webapp_launch_contexts_time
                ON webapp_launch_contexts(launched_at)`,
		},
	},
}

// Migrate приводит схему БД к последней версии, применяя недостающие миграции по порядку.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	current, err := currentVersion(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
		slog.Info("Applied migration", "version", m.Version, "name", m.Name)
	}

	return nil
}

func currentVersion(db *sql.DB) (int, error) {
	var version sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}

	return int(version.Int64), nil
}

// applyMigration выполняет все шаги миграции и её регистрацию в одной транзакции,
// чтобы прерванный запуск не оставил схему в полуприменённом состоянии.
func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.Version, err)
	}
	defer func() {
		// После успешного Commit откат — no-op, поэтому ошибку здесь игнорируем осознанно.
		_ = tx.Rollback()
	}()

	for _, stmt := range m.Stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name); err != nil {
		return fmt.Errorf("record migration %d: %w", m.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.Version, err)
	}

	return nil
}
