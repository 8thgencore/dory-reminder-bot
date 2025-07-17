package repository

import (
	"context"
	"database/sql"
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
	q := `SELECT id, chat_id, username, first_name, last_name, timezone, created_at, updated_at
		FROM users WHERE id = ? AND chat_id = ?`
	row := r.db.QueryRowContext(ctx, q, userID, chatID)
	var user domain.User
	if err := row.Scan(
		&user.ID, &user.ChatID, &user.Username, &user.FirstName, &user.LastName, &user.Timezone, &user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	q := `INSERT INTO users (id, chat_id, username, first_name, last_name, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		user.ID, user.ChatID, user.Username, user.FirstName, user.LastName, user.Timezone, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	q := `UPDATE users SET username=?, first_name=?, last_name=?, timezone=?, updated_at=? WHERE id=? AND chat_id=?`
	_, err := r.db.ExecContext(ctx, q,
		user.Username, user.FirstName, user.LastName, user.Timezone, user.UpdatedAt, user.ID, user.ChatID,
	)
	return err
}

func (r *userRepository) UpdateTimezone(ctx context.Context, chatID, userID int64, timezone string) error {
	q := `UPDATE users SET timezone=?, updated_at=? WHERE id=? AND chat_id=?`
	_, err := r.db.ExecContext(ctx, q, timezone, time.Now(), userID, chatID)
	return err
}
