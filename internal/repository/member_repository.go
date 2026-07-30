package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

const (
	upsertMemberQuery = `INSERT INTO chat_members (chat_id, user_id, last_seen)
        VALUES (?, ?, ?)
        ON CONFLICT(chat_id, user_id) DO UPDATE SET last_seen = excluded.last_seen`

	rememberWebAppLaunchQuery = `INSERT INTO webapp_launch_contexts
        (user_id, chat_id, launched_at)
        VALUES (?, ?, ?)
        ON CONFLICT(user_id) DO UPDATE SET
            chat_id = excluded.chat_id,
            launched_at = excluded.launched_at`

	// Чаты пользователя вместе с данными самого чата: личный чат Mini App подставляет сам,
	// поэтому здесь интересны прежде всего группы.
	listChatsByUserQuery = `SELECT c.chat_id, c.type, c.name, c.username, c.timezone,
            c.available, c.created_at, c.updated_at
        FROM chat_members m
        JOIN chats c ON c.chat_id = m.chat_id
        WHERE m.user_id = ? AND c.available = 1
        ORDER BY c.name, c.chat_id`

	recentWebAppLaunchQuery = `SELECT c.chat_id, c.type, c.name, c.username, c.timezone,
            c.available, c.created_at, c.updated_at
        FROM webapp_launch_contexts l
        JOIN chats c ON c.chat_id = l.chat_id
        WHERE l.user_id = ? AND l.launched_at >= ? AND c.available = 1
        LIMIT 1`
)

// MemberRepository определяет репозиторий связей "пользователь — чат".
type MemberRepository interface {
	Upsert(ctx context.Context, chatID, userID int64) error
	RememberWebAppLaunch(ctx context.Context, chatID, userID int64) error
	ListChatsByUser(ctx context.Context, userID int64) ([]*domain.Chat, error)
	RecentWebAppLaunch(ctx context.Context, userID int64, since time.Time) (*domain.Chat, error)
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

func (r *memberRepository) RememberWebAppLaunch(ctx context.Context, chatID, userID int64) error {
	if chatID == 0 || userID == 0 {
		return fmt.Errorf("%w: chat ID and user ID must be non-zero", ErrInvalidReminder)
	}

	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, rememberWebAppLaunchQuery, userID, chatID, now); err != nil {
		return fmt.Errorf("%w: failed to remember webapp launch: %v", ErrDatabaseError, err)
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
			&ch.ID,
			&ch.Type,
			&ch.Name,
			&ch.Username,
			&ch.Timezone,
			&ch.Available,
			&ch.CreatedAt,
			&ch.UpdatedAt,
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

func (r *memberRepository) RecentWebAppLaunch(
	ctx context.Context,
	userID int64,
	since time.Time,
) (*domain.Chat, error) {
	if userID == 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidReminder)
	}

	var ch domain.Chat
	err := r.db.QueryRowContext(ctx, recentWebAppLaunchQuery, userID, since.UTC()).Scan(
		&ch.ID,
		&ch.Type,
		&ch.Name,
		&ch.Username,
		&ch.Timezone,
		&ch.Available,
		&ch.CreatedAt,
		&ch.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChatNotFound
		}

		return nil, fmt.Errorf("%w: failed to query recent webapp launch: %v", ErrDatabaseError, err)
	}

	return &ch, nil
}
