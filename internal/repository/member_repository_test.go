package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestMemberRepository_RemembersLatestWebAppLaunch(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_loc=UTC")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, Migrate(db))

	now := time.Now().UTC()
	for _, chatID := range []int64{-1001, -1002} {
		_, err = db.Exec(
			`INSERT INTO chats
             (chat_id, type, name, username, timezone, created_at, updated_at)
             VALUES (?, 'group', 'Test', '', '', ?, ?)`,
			chatID,
			now,
			now,
		)
		require.NoError(t, err)
	}

	repo := NewMemberRepository(db)
	require.NoError(t, repo.RememberWebAppLaunch(context.Background(), -1001, 42))
	require.NoError(t, repo.RememberWebAppLaunch(context.Background(), -1002, 42))

	chat, err := repo.RecentWebAppLaunch(context.Background(), 42, now.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(-1002), chat.ID)

	_, err = repo.RecentWebAppLaunch(context.Background(), 42, time.Now().Add(time.Minute))
	assert.True(t, errors.Is(err, ErrChatNotFound))
}
