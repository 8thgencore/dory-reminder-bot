package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

const (
	upsertMemberQuery = `INSERT INTO chat_members (chat_id, user_id, last_seen)
        VALUES (?, ?, ?)
        ON CONFLICT(chat_id, user_id) DO UPDATE SET last_seen = excluded.last_seen`

	// Чаты пользователя вместе с данными самого чата: личный чат Mini App подставляет сам,
	// поэтому здесь интересны прежде всего группы.
	listChatsByUserQuery = `SELECT c.chat_id, c.type, c.name, c.username, c.timezone, c.created_at, c.updated_at
        FROM chat_members m
        JOIN chats c ON c.chat_id = m.chat_id
        WHERE m.user_id = ?
        ORDER BY c.name, c.chat_id`
)

// MemberRepository определяет репозиторий связей "пользователь — чат".
type MemberRepository interface {
	Upsert(ctx context.Context, chatID, userID int64) error
	ListChatsByUser(ctx context.Context, userID int64) ([]*domain.Chat, error)
}

type memberRepository struct {
	db DBExecutor
}

// NewMemberRepository создает новый MemberRepository.
func NewMemberRepository(db *sql.DB) MemberRepository {
	if db == nil {
		panic("database connection cannot be nil")
	}

	return &memberRepository{db: db}
}

func (r *memberRepository) Upsert(ctx context.Context, chatID, userID int64) error {
	if chatID == 0 || userID == 0 {
		return fmt.Errorf("%w: chat ID and user ID must be non-zero", ErrInvalidReminder)
	}

	if _, err := r.db.ExecContext(ctx, upsertMemberQuery, chatID, userID, time.Now().UTC()); err != nil {
		return fmt.Errorf("%w: failed to upsert chat member: %v", ErrDatabaseError, err)
	}

	return nil
}

func (r *memberRepository) ListChatsByUser(ctx context.Context, userID int64) ([]*domain.Chat, error) {
	if userID == 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidReminder)
	}

	rows, err := r.db.QueryContext(ctx, listChatsByUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query chats by user: %v", ErrDatabaseError, err)
	}
	defer closeRows(rows)

	var chats []*domain.Chat
	for rows.Next() {
		var ch domain.Chat
		if err := rows.Scan(
			&ch.ID, &ch.Type, &ch.Name, &ch.Username, &ch.Timezone, &ch.CreatedAt, &ch.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%w: failed to scan chat: %v", ErrDatabaseError, err)
		}
		chats = append(chats, &ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: failed to iterate chats: %v", ErrDatabaseError, err)
	}

	return chats, nil
}
