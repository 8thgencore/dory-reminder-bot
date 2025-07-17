package repository

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// ReminderRepository определяет интерфейс репозитория напоминаний.
type ReminderRepository interface {
	Create(ctx context.Context, r *domain.Reminder) error
	Update(ctx context.Context, r *domain.Reminder) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*domain.Reminder, error)
	ListByChat(ctx context.Context, chatID int64) ([]*domain.Reminder, error)
	ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error)
}

type reminderRepository struct {
	db *sql.DB
}

// NewReminderRepository создает новый ReminderRepository.
func NewReminderRepository(db *sql.DB) ReminderRepository {
	return &reminderRepository{db: db}
}

// TODO: Реализация методов интерфейса

func (r *reminderRepository) Create(ctx context.Context, rem *domain.Reminder) error {
	q := `INSERT INTO reminders (chat_id, user_id, text, next_time, repeat, repeat_days, repeat_every, paused, created_at, updated_at, timezone)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	days := ""
	if len(rem.RepeatDays) > 0 {
		for i, d := range rem.RepeatDays {
			if i > 0 {
				days += ","
			}
			days += strconv.Itoa(d)
		}
	}
	_, err := r.db.ExecContext(ctx, q,
		rem.ChatID,
		rem.UserID,
		rem.Text,
		rem.NextTime,
		rem.Repeat,
		days,
		rem.RepeatEvery,
		rem.Paused,
		rem.CreatedAt,
		rem.UpdatedAt,
		rem.Timezone,
	)
	return err
}

func (r *reminderRepository) Update(ctx context.Context, rem *domain.Reminder) error {
	q := `UPDATE reminders SET chat_id=?, user_id=?, text=?, next_time=?, repeat=?, repeat_days=?, repeat_every=?, paused=?, created_at=?, updated_at=?, timezone=? WHERE id=?`
	days := ""
	if len(rem.RepeatDays) > 0 {
		for i, d := range rem.RepeatDays {
			if i > 0 {
				days += ","
			}
			days += strconv.Itoa(d)
		}
	}
	_, err := r.db.ExecContext(ctx, q,
		rem.ChatID,
		rem.UserID,
		rem.Text,
		rem.NextTime,
		rem.Repeat,
		days,
		rem.RepeatEvery,
		rem.Paused,
		rem.CreatedAt,
		rem.UpdatedAt,
		rem.Timezone,
		rem.ID,
	)
	return err
}

func (r *reminderRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM reminders WHERE id = ?", id)
	return err
}

func (r *reminderRepository) GetByID(ctx context.Context, id int64) (*domain.Reminder, error) {
	q := `SELECT id, chat_id, user_id, text, next_time, repeat, repeat_days, repeat_every, paused, created_at, updated_at, timezone
		FROM reminders WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	var rem domain.Reminder
	var days string
	if err := row.Scan(
		&rem.ID, &rem.ChatID, &rem.UserID, &rem.Text, &rem.NextTime, &rem.Repeat, &days, &rem.RepeatEvery, &rem.Paused, &rem.CreatedAt, &rem.UpdatedAt, &rem.Timezone,
	); err != nil {
		return nil, err
	}
	if days != "" {
		rem.RepeatDays = append(rem.RepeatDays, splitCommaInts(days)...)
	}

	return &rem, nil
}

func (r *reminderRepository) ListByChat(ctx context.Context, chatID int64) ([]*domain.Reminder, error) {
	q := `SELECT id, chat_id, user_id, text, next_time, repeat, repeat_days, repeat_every, paused, created_at, updated_at, timezone
		FROM reminders WHERE chat_id = ?`
	rows, err := r.db.QueryContext(ctx, q, chatID)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	var res []*domain.Reminder
	for rows.Next() {
		var rem domain.Reminder
		var days string
		if err := rows.Scan(
			&rem.ID, &rem.ChatID, &rem.UserID, &rem.Text, &rem.NextTime, &rem.Repeat, &days, &rem.RepeatEvery, &rem.Paused, &rem.CreatedAt, &rem.UpdatedAt, &rem.Timezone,
		); err != nil {
			return nil, err
		}
		if days != "" {
			rem.RepeatDays = append(rem.RepeatDays, splitCommaInts(days)...)
		}
		res = append(res, &rem)
	}

	return res, nil
}

func (r *reminderRepository) ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	q := `SELECT id, chat_id, user_id, text, next_time, repeat, repeat_days, repeat_every, paused, created_at, updated_at, timezone
		FROM reminders WHERE next_time <= ? AND paused = 0`
	rows, err := r.db.QueryContext(ctx, q, now)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	var res []*domain.Reminder
	for rows.Next() {
		var rem domain.Reminder
		var days string
		if err := rows.Scan(
			&rem.ID, &rem.ChatID, &rem.UserID, &rem.Text, &rem.NextTime, &rem.Repeat, &days, &rem.RepeatEvery, &rem.Paused, &rem.CreatedAt, &rem.UpdatedAt, &rem.Timezone,
		); err != nil {
			return nil, err
		}
		if days != "" {
			rem.RepeatDays = append(rem.RepeatDays, splitCommaInts(days)...)
		}
		res = append(res, &rem)
	}

	return res, nil
}

func splitCommaInts(s string) []int {
	var res []int
	for _, part := range splitAndTrim(s, ",") {
		if n, err := strconv.Atoi(part); err == nil {
			res = append(res, n)
		}
	}
	return res
}

func splitAndTrim(s, sep string) []string {
	var out []string
	out = append(out, splitNoEmpty(s, sep)...)
	return out
}

func splitNoEmpty(s, sep string) []string {
	var out []string
	out = append(out, splitRaw(s, sep)...)
	return out
}

func splitRaw(s, sep string) []string {
	var res []string
	start := 0
	for i := 0; i+len(sep) <= len(s); {
		if s[i:i+len(sep)] == sep {
			res = append(res, s[start:i])
			start = i + len(sep)
			i = start
		} else {
			i++
		}
	}
	res = append(res, s[start:])
	return res
}
