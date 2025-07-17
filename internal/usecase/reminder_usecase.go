package usecase

import (
	"context"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
)

// ReminderUsecase определяет бизнес-логику для работы с напоминаниями.
type ReminderUsecase interface {
	AddReminder(ctx context.Context, r *domain.Reminder) error
	EditReminder(ctx context.Context, r *domain.Reminder) error
	DeleteReminder(ctx context.Context, id int64) error
	PauseReminder(ctx context.Context, id int64) error
	ResumeReminder(ctx context.Context, id int64) error
	ListReminders(ctx context.Context, chatID int64) ([]*domain.Reminder, error)
	ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error)
}

type reminderUsecase struct {
	repo repository.ReminderRepository
}

// NewReminderUsecase создает новый ReminderUsecase.
func NewReminderUsecase(repo repository.ReminderRepository) ReminderUsecase {
	return &reminderUsecase{repo: repo}
}

// TODO: Реализация методов интерфейса

func (u *reminderUsecase) AddReminder(ctx context.Context, r *domain.Reminder) error {
	return u.repo.Create(ctx, r)
}

func (u *reminderUsecase) EditReminder(ctx context.Context, r *domain.Reminder) error {
	return u.repo.Update(ctx, r)
}

func (u *reminderUsecase) DeleteReminder(ctx context.Context, id int64) error {
	return u.repo.Delete(ctx, id)
}

func (u *reminderUsecase) PauseReminder(ctx context.Context, id int64) error {
	r, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	r.Paused = true

	return u.repo.Update(ctx, r)
}

func (u *reminderUsecase) ResumeReminder(ctx context.Context, id int64) error {
	r, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	r.Paused = false

	return u.repo.Update(ctx, r)
}

func (u *reminderUsecase) ListReminders(ctx context.Context, chatID int64) ([]*domain.Reminder, error) {
	return u.repo.ListByChat(ctx, chatID)
}

func (u *reminderUsecase) ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	return u.repo.ListDue(ctx, now)
}
