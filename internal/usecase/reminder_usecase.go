package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
)

// ReminderUsecase определяет бизнес-логику для работы с напоминаниями.
//
// Методы с суффиксом Owned принимают chatID и обязаны использоваться всюду, где
// идентификатор напоминания приходит извне: без сверки владельца пользователь
// Mini App смог бы адресовать чужое напоминание перебором ID.
type ReminderUsecase interface {
	AddReminder(ctx context.Context, r *domain.Reminder) error
	EditReminder(ctx context.Context, r *domain.Reminder) error
	DeleteReminder(ctx context.Context, id int64) error
	PauseReminder(ctx context.Context, id int64) error
	ResumeReminder(ctx context.Context, id int64) error
	ListReminders(ctx context.Context, chatID int64) ([]*domain.Reminder, error)
	ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error)

	// GetReminder читает напоминание без проверки владельца. Вызывающий обязан
	// авторизовать доступ к ChatID полученной записи.
	GetReminder(ctx context.Context, id int64) (*domain.Reminder, error)
	GetOwned(ctx context.Context, id, chatID int64) (*domain.Reminder, error)
	UpdateOwned(ctx context.Context, r *domain.Reminder, chatID int64) error
	DeleteOwned(ctx context.Context, id, chatID int64) error
	SetPausedOwned(ctx context.Context, id, chatID int64, paused bool) error
}

type reminderUsecase struct {
	repo repository.ReminderRepository
}

// NewReminderUsecase создает новый ReminderUsecase.
func NewReminderUsecase(repo repository.ReminderRepository) ReminderUsecase {
	return &reminderUsecase{repo: repo}
}

func (u *reminderUsecase) AddReminder(ctx context.Context, r *domain.Reminder) error {
	r.Normalize()
	if err := r.Validate(); err != nil {
		return err
	}

	existing, err := u.repo.ListByChat(ctx, r.ChatID)
	if err != nil {
		return err
	}
	if len(existing) >= domain.MaxRemindersPerChat {
		return domain.ErrTooManyReminders
	}

	return u.repo.Create(ctx, r)
}

func (u *reminderUsecase) EditReminder(ctx context.Context, r *domain.Reminder) error {
	r.Normalize()
	if err := r.Validate(); err != nil {
		return err
	}

	return u.repo.Update(ctx, r)
}

func (u *reminderUsecase) DeleteReminder(ctx context.Context, id int64) error {
	return u.repo.Delete(ctx, id)
}

func (u *reminderUsecase) PauseReminder(ctx context.Context, id int64) error {
	return u.setPaused(ctx, id, true)
}

func (u *reminderUsecase) ResumeReminder(ctx context.Context, id int64) error {
	return u.setPaused(ctx, id, false)
}

func (u *reminderUsecase) setPaused(ctx context.Context, id int64, paused bool) error {
	r, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	r.Paused = paused

	return u.repo.Update(ctx, r)
}

func (u *reminderUsecase) ListReminders(ctx context.Context, chatID int64) ([]*domain.Reminder, error) {
	return u.repo.ListByChat(ctx, chatID)
}

func (u *reminderUsecase) ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	return u.repo.ListDue(ctx, now)
}

func (u *reminderUsecase) GetReminder(ctx context.Context, id int64) (*domain.Reminder, error) {
	return u.repo.GetByID(ctx, id)
}

// GetOwned возвращает напоминание, только если оно принадлежит указанному чату.
//
// Чужое напоминание неотличимо от несуществующего — возвращается та же ошибка,
// чтобы перебором ID нельзя было узнать, какие из них существуют.
func (u *reminderUsecase) GetOwned(ctx context.Context, id, chatID int64) (*domain.Reminder, error) {
	r, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.ChatID != chatID {
		return nil, fmt.Errorf("%w: reminder with ID %d not found", repository.ErrReminderNotFound, id)
	}

	return r, nil
}

func (u *reminderUsecase) UpdateOwned(ctx context.Context, r *domain.Reminder, chatID int64) error {
	existing, err := u.GetOwned(ctx, r.ID, chatID)
	if err != nil {
		return err
	}

	// Поля, которые клиент менять не может.
	r.ChatID = existing.ChatID
	r.CreatedAt = existing.CreatedAt

	return u.EditReminder(ctx, r)
}

func (u *reminderUsecase) DeleteOwned(ctx context.Context, id, chatID int64) error {
	if _, err := u.GetOwned(ctx, id, chatID); err != nil {
		return err
	}

	return u.repo.Delete(ctx, id)
}

func (u *reminderUsecase) SetPausedOwned(ctx context.Context, id, chatID int64, paused bool) error {
	r, err := u.GetOwned(ctx, id, chatID)
	if err != nil {
		return err
	}
	r.Paused = paused

	return u.repo.Update(ctx, r)
}
