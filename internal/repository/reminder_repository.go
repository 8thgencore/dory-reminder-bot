package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// SQL запросы вынесены в константы для лучшей читаемости и переиспользования
const (
	createReminderQuery = `INSERT INTO reminders (chat_id, text, next_time, repeat, repeat_days, 
        repeat_every, paused, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	updateReminderQuery = `UPDATE reminders SET chat_id=?, text=?, next_time=?, repeat=?, repeat_days=?, 
        repeat_every=?, paused=?, created_at=?, updated_at=? WHERE id=?`

	deleteReminderQuery = `DELETE FROM reminders WHERE id = ?`

	getReminderByIDQuery = `SELECT id, chat_id, text, next_time, repeat, repeat_days, repeat_every, paused, 
        created_at, updated_at
        FROM reminders WHERE id = ?`

	listRemindersByChatQuery = `SELECT id, chat_id, text, next_time, repeat, repeat_days, repeat_every, paused, 
        created_at, updated_at
        FROM reminders WHERE chat_id = ?`

	listDueRemindersQuery = `SELECT id, chat_id, text, next_time, repeat, repeat_days, repeat_every, paused, 
        created_at, updated_at
        FROM reminders WHERE next_time <= ? AND paused = 0`
)

// Ошибки репозитория
var (
	ErrReminderNotFound = errors.New("reminder not found")
	ErrInvalidReminder  = errors.New("invalid reminder data")
	ErrDatabaseError    = errors.New("database error")
)

// DBExecutor определяет интерфейс для работы с базой данных
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

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
	db DBExecutor
}

// NewReminderRepository создает новый ReminderRepository.
func NewReminderRepository(db *sql.DB) ReminderRepository {
	if db == nil {
		panic("database connection cannot be nil")
	}
	return &reminderRepository{db: db}
}

// validateReminder проверяет корректность данных напоминания
func validateReminder(rem *domain.Reminder) error {
	if rem == nil {
		return fmt.Errorf("%w: reminder is nil", ErrInvalidReminder)
	}
	if rem.Text == "" {
		return fmt.Errorf("%w: reminder text cannot be empty", ErrInvalidReminder)
	}
	if rem.ChatID == 0 {
		return fmt.Errorf("%w: invalid chat ID", ErrInvalidReminder)
	}
	// user scope removed; we only validate chat

	return nil
}

func (r *reminderRepository) Create(ctx context.Context, rem *domain.Reminder) error {
	slog.Info("[Create] called", "reminder", rem)

	if err := validateReminder(rem); err != nil {
		slog.Error("[Create] validation failed", "reminder", rem, "error", err)
		return err
	}

	// Устанавливаем временные метки если они не установлены
	if rem.CreatedAt.IsZero() {
		rem.CreatedAt = time.Now()
	}
	if rem.UpdatedAt.IsZero() {
		rem.UpdatedAt = time.Now()
	}

	days := serializeRepeatDays(rem.RepeatDays)
	slog.Info("[Create] prepared data",
		"chatID", rem.ChatID, "text", rem.Text,
		"nextTime", rem.NextTime, "repeat", rem.Repeat, "days", days)

	result, err := r.db.ExecContext(ctx, createReminderQuery,
		rem.ChatID,
		rem.Text,
		rem.NextTime,
		rem.Repeat,
		days,
		rem.RepeatEvery,
		rem.Paused,
		rem.CreatedAt,
		rem.UpdatedAt,
	)
	if err != nil {
		slog.Error("[Create] exec failed", "reminder", rem, "error", err)
		return fmt.Errorf("%w: failed to create reminder: %v", ErrDatabaseError, err)
	}

	// Получаем ID созданного напоминания
	id, err := result.LastInsertId()
	if err != nil {
		slog.Error("[Create] failed to get last insert ID", "reminder", rem, "error", err)
		return fmt.Errorf("%w: failed to get last insert ID: %v", ErrDatabaseError, err)
	}

	rem.ID = id
	slog.Info("[Create] reminder created successfully", "reminderID", id, "reminder", rem)

	return nil
}

func (r *reminderRepository) Update(ctx context.Context, rem *domain.Reminder) error {
	if err := validateReminder(rem); err != nil {
		return err
	}

	if rem.ID <= 0 {
		return fmt.Errorf("%w: invalid reminder ID", ErrInvalidReminder)
	}

	rem.UpdatedAt = time.Now()
	days := serializeRepeatDays(rem.RepeatDays)

	result, err := r.db.ExecContext(ctx, updateReminderQuery,
		rem.ChatID,
		rem.Text,
		rem.NextTime,
		rem.Repeat,
		days,
		rem.RepeatEvery,
		rem.Paused,
		rem.CreatedAt,
		rem.UpdatedAt,
		rem.ID,
	)
	if err != nil {
		return fmt.Errorf("%w: failed to update reminder: %v", ErrDatabaseError, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: failed to get rows affected: %v", ErrDatabaseError, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: reminder with ID %d not found", ErrReminderNotFound, rem.ID)
	}

	return nil
}

func (r *reminderRepository) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: invalid reminder ID", ErrInvalidReminder)
	}

	result, err := r.db.ExecContext(ctx, deleteReminderQuery, id)
	if err != nil {
		return fmt.Errorf("%w: failed to delete reminder: %v", ErrDatabaseError, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: failed to get rows affected: %v", ErrDatabaseError, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: reminder with ID %d not found", ErrReminderNotFound, id)
	}

	return nil
}

func (r *reminderRepository) GetByID(ctx context.Context, id int64) (*domain.Reminder, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: invalid reminder ID", ErrInvalidReminder)
	}

	row := r.db.QueryRowContext(ctx, getReminderByIDQuery, id)

	rem, err := scanReminder(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: reminder with ID %d not found", ErrReminderNotFound, id)
		}
		return nil, fmt.Errorf("%w: failed to get reminder: %v", ErrDatabaseError, err)
	}

	return rem, nil
}

func (r *reminderRepository) ListByChat(ctx context.Context, chatID int64) ([]*domain.Reminder, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("%w: invalid chat ID", ErrInvalidReminder)
	}

	rows, err := r.db.QueryContext(ctx, listRemindersByChatQuery, chatID)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query reminders by chat: %v", ErrDatabaseError, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Error("failed to close rows", "error", closeErr)
		}
	}()

	return scanReminders(rows)
}

func (r *reminderRepository) ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	if now.IsZero() {
		now = time.Now()
	}

	rows, err := r.db.QueryContext(ctx, listDueRemindersQuery, now)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query due reminders: %v", ErrDatabaseError, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Error("failed to close rows", "error", closeErr)
		}
	}()

	return scanReminders(rows)
}

// serializeRepeatDays сериализует массив дней в строку для хранения в БД
func serializeRepeatDays(days []int) string {
	if len(days) == 0 {
		return ""
	}

	var parts []string
	for _, day := range days {
		parts = append(parts, strconv.Itoa(day))
	}

	return strings.Join(parts, ",")
}

// scanReminder сканирует одну строку результата в структуру Reminder
func scanReminder(row *sql.Row) (*domain.Reminder, error) {
	var rem domain.Reminder
	var days string

	err := row.Scan(
		&rem.ID, &rem.ChatID, &rem.Text, &rem.NextTime, &rem.Repeat, &days,
		&rem.RepeatEvery, &rem.Paused, &rem.CreatedAt, &rem.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rem.RepeatDays = deserializeRepeatDays(days)

	return &rem, nil
}

// scanReminders сканирует множество строк результата в слайс Reminder
func scanReminders(rows *sql.Rows) ([]*domain.Reminder, error) {
	var reminders []*domain.Reminder

	for rows.Next() {
		var rem domain.Reminder
		var days string

		err := rows.Scan(
			&rem.ID, &rem.ChatID, &rem.Text, &rem.NextTime, &rem.Repeat, &days,
			&rem.RepeatEvery, &rem.Paused, &rem.CreatedAt, &rem.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		rem.RepeatDays = deserializeRepeatDays(days)
		reminders = append(reminders, &rem)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reminders, nil
}

// deserializeRepeatDays десериализует строку дней обратно в массив
func deserializeRepeatDays(days string) []int {
	if days == "" {
		return nil
	}

	var result []int
	for _, part := range splitAndTrim(days, ",") {
		if n, err := strconv.Atoi(part); err == nil {
			result = append(result, n)
		}
	}

	return result
}

// splitAndTrim разбивает строку по разделителю и удаляет пробелы
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
