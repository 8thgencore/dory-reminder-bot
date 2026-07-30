package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestChatRepository_MigratePreservesAndFreezesData(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_loc=UTC")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, Migrate(db))

	const (
		oldChatID = int64(-5342885594)
		newChatID = int64(-1004314310091)
		userID    = int64(412414551)
	)
	ctx := context.Background()
	now := time.Now().UTC()

	chatRepo := NewChatRepository(db)
	memberRepo := NewMemberRepository(db)
	reminderRepo := NewReminderRepository(db)

	require.NoError(t, chatRepo.Upsert(ctx, &domain.Chat{
		ID:        oldChatID,
		Type:      "group",
		Name:      "Старая группа",
		Timezone:  "Europe/Moscow",
		Available: true,
		CreatedAt: now.Add(-time.Hour),
	}))
	require.NoError(t, chatRepo.Upsert(ctx, &domain.Chat{
		ID:        newChatID,
		Type:      "supergroup",
		Name:      "Новая супергруппа",
		Available: true,
		CreatedAt: now,
	}))

	reminder := &domain.Reminder{
		ChatID:   oldChatID,
		Text:     "не потерять",
		NextTime: now.Add(-time.Minute),
		Repeat:   domain.RepeatEveryDay,
	}
	require.NoError(t, reminderRepo.Create(ctx, reminder))
	require.NoError(t, memberRepo.Upsert(ctx, oldChatID, userID))
	require.NoError(t, memberRepo.Upsert(ctx, newChatID, userID))
	require.NoError(t, memberRepo.RememberWebAppLaunch(ctx, oldChatID, userID))

	require.NoError(t, chatRepo.Migrate(ctx, oldChatID, newChatID))
	// Повторное событие Telegram не должно портить уже перенесённые данные.
	require.NoError(t, chatRepo.Migrate(ctx, oldChatID, newChatID))

	_, err = chatRepo.GetByID(ctx, oldChatID)
	assert.ErrorIs(t, err, ErrChatNotFound)

	chat, err := chatRepo.GetByID(ctx, newChatID)
	require.NoError(t, err)
	assert.Equal(t, "Новая супергруппа", chat.Name)
	assert.Equal(t, "Europe/Moscow", chat.Timezone, "empty target timezone must inherit the old value")
	assert.True(t, chat.Available)

	resolvedID, err := chatRepo.ResolveID(ctx, oldChatID)
	require.NoError(t, err)
	assert.Equal(t, newChatID, resolvedID)

	moved, err := reminderRepo.ListByChat(ctx, newChatID)
	require.NoError(t, err)
	require.Len(t, moved, 1)
	assert.Equal(t, reminder.ID, moved[0].ID)

	chats, err := memberRepo.ListChatsByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, chats, 1, "duplicate memberships must be merged")
	assert.Equal(t, newChatID, chats[0].ID)

	launch, err := memberRepo.RecentWebAppLaunch(ctx, userID, now.Add(-time.Minute))
	require.NoError(t, err)
	assert.Equal(t, newChatID, launch.ID)

	require.NoError(t, chatRepo.SetAvailable(ctx, newChatID, false))
	chats, err = memberRepo.ListChatsByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, chats)

	_, err = memberRepo.RecentWebAppLaunch(ctx, userID, now.Add(-time.Minute))
	assert.True(t, errors.Is(err, ErrChatNotFound))

	due, err := reminderRepo.ListDue(ctx, now)
	require.NoError(t, err)
	assert.Empty(t, due, "unavailable chats must be frozen")

	stored, err := reminderRepo.ListByChat(ctx, newChatID)
	require.NoError(t, err)
	assert.Len(t, stored, 1, "freezing must not delete reminders")

	require.NoError(t, chatRepo.SetAvailable(ctx, newChatID, true))
	due, err = reminderRepo.ListDue(ctx, now)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, reminder.ID, due[0].ID)
}
