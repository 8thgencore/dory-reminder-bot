package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// UserRepository определяет интерфейс репозитория пользователей.
type UserRepository interface {
	GetByChatAndUser(ctx context.Context, chatID, userID int64) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	UpdateTimezone(ctx context.Context, chatID, userID int64, timezone string) error
}

type userRepository struct {
	db *sql.DB
}

// NewUserRepository создает новый UserRepository.
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByChatAndUser(ctx context.Context, chatID, userID int64) (*domain.User, error) {
	slog.Info("[GetByChatAndUser] called", "chatID", chatID, "userID", userID)

	q := `SELECT chat_id, user_id, username, first_name, last_name, timezone, created_at, updated_at
		FROM users WHERE chat_id = ? AND user_id = ?`
	row := r.db.QueryRowContext(ctx, q, chatID, userID)
	var user domain.User
	if err := row.Scan(
		&user.ChatID, &user.ID, &user.Username, &user.FirstName, &user.LastName, &user.Timezone,
		&user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			slog.Info("[GetByChatAndUser] user not found", "chatID", chatID, "userID", userID)
			return nil, nil
		}
		slog.Error("[GetByChatAndUser] scan error", "chatID", chatID, "userID", userID, "error", err)

		return nil, err
	}

	slog.Info("[GetByChatAndUser] user found", "user", user)

	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	slog.Info("[Create] called", "user", user)

	q := `INSERT INTO users (chat_id, user_id, username, first_name, last_name, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		user.ChatID, user.ID, user.Username, user.FirstName, user.LastName, user.Timezone, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		slog.Error("[Create] failed", "user", user, "error", err)
		return err
	}

	slog.Info("[Create] user created successfully", "user", user)

	return nil
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	slog.Info("[Update] called", "user", user)

	q := `UPDATE users SET username=?, first_name=?, last_name=?, timezone=?, updated_at=?
		WHERE chat_id=? AND user_id=?`
	_, err := r.db.ExecContext(ctx, q,
		user.Username, user.FirstName, user.LastName, user.Timezone, user.UpdatedAt,
		user.ChatID, user.ID,
	)
	if err != nil {
		slog.Error("[Update] failed", "user", user, "error", err)
		return err
	}

	slog.Info("[Update] user updated successfully", "user", user)

	return nil
}

func (r *userRepository) UpdateTimezone(ctx context.Context, chatID, userID int64, timezone string) error {
	slog.Info("[UpdateTimezone] called", "chatID", chatID, "userID", userID, "timezone", timezone)

	q := `UPDATE users SET timezone=?, updated_at=? WHERE chat_id=? AND user_id=?`
	_, err := r.db.ExecContext(ctx, q, timezone, time.Now(), chatID, userID)
	if err != nil {
		slog.Error("[UpdateTimezone] failed", "chatID", chatID, "userID", userID, "timezone", timezone, "error", err)
		return err
	}

	slog.Info("[UpdateTimezone] timezone updated successfully", "chatID", chatID, "userID", userID, "timezone", timezone)

	return nil
}
