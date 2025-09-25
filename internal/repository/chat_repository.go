package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// ChatRepository определяет интерфейс репозитория чатов.
type ChatRepository interface {
	GetByID(ctx context.Context, chatID int64) (*domain.Chat, error)
	Upsert(ctx context.Context, chat *domain.Chat) error
	UpdateTimezone(ctx context.Context, chatID int64, timezone string) error
}

type chatRepository struct {
	db *sql.DB
}

// NewChatRepository создает новый ChatRepository.
func NewChatRepository(db *sql.DB) ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) GetByID(ctx context.Context, chatID int64) (*domain.Chat, error) {
	slog.Info("[Chat.GetByID] called", "chatID", chatID)

	q := `SELECT chat_id, type, name, username, timezone, created_at, updated_at FROM chats WHERE chat_id=?`
	row := r.db.QueryRowContext(ctx, q, chatID)
	var ch domain.Chat
	if err := row.Scan(&ch.ID, &ch.Type, &ch.Name, &ch.Username, &ch.Timezone, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			slog.Info("[Chat.GetByID] not found", "chatID", chatID)
			return nil, nil
		}
		slog.Error("[Chat.GetByID] scan error", "chatID", chatID, "error", err)

		return nil, err
	}

	return &ch, nil
}

func (r *chatRepository) Upsert(ctx context.Context, chat *domain.Chat) error {
	slog.Info("[Chat.Upsert] called", "chat", chat)

	now := time.Now()
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}
	chat.UpdatedAt = now

	// Try update first
	uq := `UPDATE chats SET type=?, name=?, username=?, timezone=?, updated_at=? WHERE chat_id=?`
	res, err := r.db.ExecContext(ctx, uq, chat.Type, chat.Name, chat.Username, chat.Timezone, chat.UpdatedAt, chat.ID)
	if err != nil {
		slog.Error("[Chat.Upsert] update failed", "chat", chat, "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows > 0 {
		return nil
	}

	// Insert if not updated
	iq := `INSERT INTO chats (chat_id, type, name, username, timezone, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(
		ctx,
		iq,
		chat.ID,
		chat.Type,
		chat.Name,
		chat.Username,
		chat.Timezone,
		chat.CreatedAt,
		chat.UpdatedAt,
	)
	if err != nil {
		slog.Error("[Chat.Upsert] insert failed", "chat", chat, "error", err)
		return err
	}

	return nil
}

func (r *chatRepository) UpdateTimezone(ctx context.Context, chatID int64, timezone string) error {
	slog.Info("[Chat.UpdateTimezone] called", "chatID", chatID, "timezone", timezone)

	q := `UPDATE chats SET timezone=?, updated_at=? WHERE chat_id=?`
	_, err := r.db.ExecContext(ctx, q, timezone, time.Now(), chatID)
	if err != nil {
		slog.Error("[Chat.UpdateTimezone] failed", "chatID", chatID, "timezone", timezone, "error", err)
		return err
	}

	return nil
}
