package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// ErrChatNotFound возвращается, когда чата нет в базе.
var ErrChatNotFound = errors.New("chat not found")

// ChatRepository определяет интерфейс репозитория чатов.
type ChatRepository interface {
	// GetByID возвращает чат или ErrChatNotFound, если его нет.
	GetByID(ctx context.Context, chatID int64) (*domain.Chat, error)
	Upsert(ctx context.Context, chat *domain.Chat) error
	UpdateTimezone(ctx context.Context, chatID int64, timezone string) error
	// ResolveID заменяет устаревший ID группы на актуальный ID супергруппы.
	ResolveID(ctx context.Context, chatID int64) (int64, error)
	// Migrate атомарно переносит все данные группы на новый Telegram ID.
	Migrate(ctx context.Context, oldChatID, newChatID int64) error
	// SetAvailable включает или замораживает чат без удаления его данных.
	SetAvailable(ctx context.Context, chatID int64, available bool) error
}

type chatRepository struct {
	db *sql.DB
}

// NewChatRepository создает новый ChatRepository.
func NewChatRepository(db *sql.DB) ChatRepository {
	if db == nil {
		panic("database connection cannot be nil")
	}

	return &chatRepository{db: db}
}

func (r *chatRepository) GetByID(ctx context.Context, chatID int64) (*domain.Chat, error) {
	slog.Debug("[Chat.GetByID] called", "chatID", chatID)

	q := `SELECT chat_id, type, name, username, timezone, available, created_at, updated_at
        FROM chats WHERE chat_id=?`
	row := r.db.QueryRowContext(ctx, q, chatID)
	var ch domain.Chat
	if err := row.Scan(
		&ch.ID,
		&ch.Type,
		&ch.Name,
		&ch.Username,
		&ch.Timezone,
		&ch.Available,
		&ch.CreatedAt,
		&ch.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("[Chat.GetByID] not found", "chatID", chatID)
			return nil, ErrChatNotFound
		}
		slog.Error("[Chat.GetByID] scan error", "chatID", chatID, "error", err)

		return nil, err
	}

	return &ch, nil
}

func (r *chatRepository) Upsert(ctx context.Context, chat *domain.Chat) error {
	slog.Debug("[Chat.Upsert] called", "chatID", chat.ID, "type", chat.Type)

	now := time.Now()
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = now
	}
	chat.UpdatedAt = now

	// Try update first
	uq := `UPDATE chats SET type=?, name=?, username=?, timezone=?, available=?, updated_at=? WHERE chat_id=?`
	res, err := r.db.ExecContext(
		ctx,
		uq,
		chat.Type,
		chat.Name,
		chat.Username,
		chat.Timezone,
		chat.Available,
		chat.UpdatedAt,
		chat.ID,
	)
	if err != nil {
		slog.Error("[Chat.Upsert] update failed", "chat", chat, "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows > 0 {
		return nil
	}

	// Insert if not updated
	iq := `INSERT INTO chats
        (chat_id, type, name, username, timezone, available, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(
		ctx,
		iq,
		chat.ID,
		chat.Type,
		chat.Name,
		chat.Username,
		chat.Timezone,
		chat.Available,
		chat.CreatedAt,
		chat.UpdatedAt,
	)
	if err != nil {
		slog.Error("[Chat.Upsert] insert failed", "chat", chat, "error", err)
		return err
	}

	return nil
}

func (r *chatRepository) ResolveID(ctx context.Context, chatID int64) (int64, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("%w: invalid chat ID", ErrDatabaseError)
	}

	const maxAliasDepth = 8
	current := chatID
	seen := map[int64]struct{}{current: {}}

	for range maxAliasDepth {
		var next int64
		err := r.db.QueryRowContext(
			ctx,
			`SELECT new_chat_id FROM chat_id_aliases WHERE old_chat_id=?`,
			current,
		).Scan(&next)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return current, nil
		case err != nil:
			return 0, fmt.Errorf("%w: resolve chat ID: %v", ErrDatabaseError, err)
		}

		if next == 0 {
			return 0, fmt.Errorf("%w: invalid alias for chat %d", ErrDatabaseError, current)
		}
		if _, exists := seen[next]; exists {
			return 0, fmt.Errorf("%w: cyclic chat ID alias", ErrDatabaseError)
		}

		seen[next] = struct{}{}
		current = next
	}

	return 0, fmt.Errorf("%w: chat ID alias chain is too deep", ErrDatabaseError)
}

func (r *chatRepository) Migrate(ctx context.Context, oldChatID, newChatID int64) error {
	if oldChatID == 0 || newChatID == 0 {
		return fmt.Errorf("%w: chat IDs must be non-zero", ErrDatabaseError)
	}
	if oldChatID == newChatID {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: begin chat migration: %v", ErrDatabaseError, err)
	}
	defer func() { _ = tx.Rollback() }()

	oldChat, err := getChatTx(ctx, tx, oldChatID)
	if err != nil && !errors.Is(err, ErrChatNotFound) {
		return err
	}
	newChat, err := getChatTx(ctx, tx, newChatID)
	if err != nil && !errors.Is(err, ErrChatNotFound) {
		return err
	}

	merged := mergeMigratedChat(oldChat, newChat, newChatID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO chats
        (chat_id, type, name, username, timezone, available, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(chat_id) DO UPDATE SET
            type=excluded.type,
            name=excluded.name,
            username=excluded.username,
            timezone=excluded.timezone,
            available=excluded.available,
            created_at=excluded.created_at,
            updated_at=excluded.updated_at`,
		merged.ID,
		merged.Type,
		merged.Name,
		merged.Username,
		merged.Timezone,
		merged.Available,
		merged.CreatedAt,
		merged.UpdatedAt,
	); err != nil {
		return fmt.Errorf("%w: upsert migrated chat: %v", ErrDatabaseError, err)
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_id_aliases
        (old_chat_id, new_chat_id, migrated_at) VALUES (?, ?, ?)
        ON CONFLICT(old_chat_id) DO UPDATE SET
            new_chat_id=excluded.new_chat_id,
            migrated_at=excluded.migrated_at`,
		oldChatID, newChatID, now,
	); err != nil {
		return fmt.Errorf("%w: store chat ID alias: %v", ErrDatabaseError, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE reminders SET chat_id=?, updated_at=? WHERE chat_id=?`,
		newChatID,
		now,
		oldChatID,
	); err != nil {
		return fmt.Errorf("%w: move reminders: %v", ErrDatabaseError, err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_members (chat_id, user_id, last_seen)
        SELECT ?, user_id, last_seen FROM chat_members WHERE chat_id=?
        ON CONFLICT(chat_id, user_id) DO UPDATE SET
            last_seen=CASE
                WHEN excluded.last_seen > chat_members.last_seen THEN excluded.last_seen
                ELSE chat_members.last_seen
            END`,
		newChatID, oldChatID,
	); err != nil {
		return fmt.Errorf("%w: merge chat members: %v", ErrDatabaseError, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chat_members WHERE chat_id=?`, oldChatID); err != nil {
		return fmt.Errorf("%w: delete old chat members: %v", ErrDatabaseError, err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE webapp_launch_contexts SET chat_id=? WHERE chat_id=?`,
		newChatID,
		oldChatID,
	); err != nil {
		return fmt.Errorf("%w: move webapp launch context: %v", ErrDatabaseError, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chats WHERE chat_id=?`, oldChatID); err != nil {
		return fmt.Errorf("%w: delete old chat: %v", ErrDatabaseError, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit chat migration: %v", ErrDatabaseError, err)
	}

	return nil
}

func (r *chatRepository) SetAvailable(ctx context.Context, chatID int64, available bool) error {
	if chatID == 0 {
		return fmt.Errorf("%w: invalid chat ID", ErrDatabaseError)
	}

	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO chats
        (chat_id, type, name, username, timezone, available, created_at, updated_at)
        VALUES (?, 'group', '', '', '', ?, ?, ?)
        ON CONFLICT(chat_id) DO UPDATE SET
            available=excluded.available,
            updated_at=excluded.updated_at`,
		chatID,
		available,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("%w: set chat availability: %v", ErrDatabaseError, err)
	}

	return nil
}

func getChatTx(ctx context.Context, tx *sql.Tx, chatID int64) (*domain.Chat, error) {
	var ch domain.Chat
	err := tx.QueryRowContext(ctx, `SELECT chat_id, type, name, username, timezone,
        available, created_at, updated_at FROM chats WHERE chat_id=?`, chatID).Scan(
		&ch.ID,
		&ch.Type,
		&ch.Name,
		&ch.Username,
		&ch.Timezone,
		&ch.Available,
		&ch.CreatedAt,
		&ch.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: get chat for migration: %v", ErrDatabaseError, err)
	}

	return &ch, nil
}

func mergeMigratedChat(oldChat, newChat *domain.Chat, newChatID int64) *domain.Chat {
	now := time.Now().UTC()
	merged := &domain.Chat{
		ID:        newChatID,
		Type:      "supergroup",
		Available: true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if oldChat != nil {
		merged.Name = oldChat.Name
		merged.Username = oldChat.Username
		merged.Timezone = oldChat.Timezone
		merged.Available = oldChat.Available
		merged.CreatedAt = oldChat.CreatedAt
	}
	if newChat != nil {
		if newChat.Type != "" {
			merged.Type = newChat.Type
		}
		if newChat.Name != "" {
			merged.Name = newChat.Name
		}
		if newChat.Username != "" {
			merged.Username = newChat.Username
		}
		if newChat.Timezone != "" {
			merged.Timezone = newChat.Timezone
		}
		// Уже зафиксированное состояние нового ID авторитетнее состояния старой группы.
		merged.Available = newChat.Available
		if !newChat.CreatedAt.IsZero() && (merged.CreatedAt.IsZero() || newChat.CreatedAt.Before(merged.CreatedAt)) {
			merged.CreatedAt = newChat.CreatedAt
		}
	}

	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = now
	}

	return merged
}

func (r *chatRepository) UpdateTimezone(ctx context.Context, chatID int64, timezone string) error {
	slog.Debug("[Chat.UpdateTimezone] called", "chatID", chatID, "timezone", timezone)

	q := `UPDATE chats SET timezone=?, updated_at=? WHERE chat_id=?`
	_, err := r.db.ExecContext(ctx, q, timezone, time.Now(), chatID)
	if err != nil {
		slog.Error("[Chat.UpdateTimezone] failed", "chatID", chatID, "timezone", timezone, "error", err)
		return err
	}

	return nil
}
