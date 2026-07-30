package database

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestInitDatabase_AppliesPragmasAndMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "reminders.db")

	db, err := InitDatabase(config.DatabaseConfig{Path: path}, quietLogger())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	// WAL нужен, чтобы HTTP-слой Mini App читал базу, пока планировщик в неё пишет.
	var journalMode string
	require.NoError(t, db.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	assert.Equal(t, "wal", journalMode)

	var busyTimeout int
	require.NoError(t, db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout))
	assert.Equal(t, 5000, busyTimeout)

	for _, table := range []string{
		"reminders",
		"chats",
		"chat_members",
		"webapp_launch_contexts",
		"schema_migrations",
	} {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		assert.NoError(t, err, "table %q must exist", table)
	}

	var indexCount int
	require.NoError(t, db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN
         ('idx_reminders_due', 'idx_reminders_chat', 'idx_chat_members_user',
          'idx_webapp_launch_contexts_time')`,
	).Scan(&indexCount))
	assert.Equal(t, 4, indexCount, "scheduler and list queries rely on these indexes")
}

// Повторный запуск не должен ни падать, ни применять миграции заново.
func TestInitDatabase_IsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminders.db")

	first, err := InitDatabase(config.DatabaseConfig{Path: path}, quietLogger())
	require.NoError(t, err)

	var versionsAfterFirst int
	require.NoError(t, first.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versionsAfterFirst))
	require.NoError(t, first.Close())

	second, err := InitDatabase(config.DatabaseConfig{Path: path}, quietLogger())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })

	var versionsAfterSecond int
	require.NoError(t, second.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versionsAfterSecond))
	assert.Equal(t, versionsAfterFirst, versionsAfterSecond, "migrations must not re-apply")
}

// Существующая база, созданная до появления schema_migrations, обязана мигрировать
// без потери данных.
func TestInitDatabase_UpgradesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reminders.db")

	legacy, err := InitDatabase(config.DatabaseConfig{Path: path}, quietLogger())
	require.NoError(t, err)

	// Имитируем старое состояние: таблицы есть, журнала миграций нет, а в repeat_every
	// лежит день недели, который туда писал мастер добавления.
	_, err = legacy.Exec(`INSERT INTO reminders
        (chat_id, text, next_time, repeat, repeat_days, repeat_every, paused, created_at, updated_at)
        VALUES (1, 'еженедельное', '2025-06-10 09:00:00+00:00', 2, '3', 3, 0,
                '2025-06-01 00:00:00+00:00', '2025-06-01 00:00:00+00:00')`)
	require.NoError(t, err)
	_, err = legacy.Exec(`DROP TABLE schema_migrations`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	upgraded, err := InitDatabase(config.DatabaseConfig{Path: path}, quietLogger())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, upgraded.Close()) })

	var text string
	var repeatDays string
	var repeatEvery int
	require.NoError(t, upgraded.QueryRow(
		`SELECT text, repeat_days, repeat_every FROM reminders WHERE chat_id = 1`,
	).Scan(&text, &repeatDays, &repeatEvery))

	assert.Equal(t, "еженедельное", text, "existing data must survive the migration")
	assert.Equal(t, "3", repeatDays, "the real weekday lives in repeat_days")
	assert.Zero(t, repeatEvery, "polluted repeat_every must be cleared")
}
