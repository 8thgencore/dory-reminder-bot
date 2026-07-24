package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRepository = errors.New("repository failure")

type reminderRepositoryStub struct {
	reminder  *domain.Reminder
	reminders []*domain.Reminder
	err       error
	created   *domain.Reminder
	updated   *domain.Reminder
	deletedID int64
	listCalls int
}

func (s *reminderRepositoryStub) Create(_ context.Context, reminder *domain.Reminder) error {
	s.created = reminder

	return s.err
}

func (s *reminderRepositoryStub) Update(_ context.Context, reminder *domain.Reminder) error {
	s.updated = reminder

	return s.err
}

func (s *reminderRepositoryStub) Delete(_ context.Context, id int64) error {
	s.deletedID = id

	return s.err
}

func (s *reminderRepositoryStub) GetByID(_ context.Context, _ int64) (*domain.Reminder, error) {
	return s.reminder, s.err
}

func (s *reminderRepositoryStub) ListByChat(_ context.Context, _ int64) ([]*domain.Reminder, error) {
	s.listCalls++

	return s.reminders, s.err
}

func (s *reminderRepositoryStub) ListDue(_ context.Context, _ time.Time) ([]*domain.Reminder, error) {
	return s.reminders, s.err
}

func validReminder() *domain.Reminder {
	return &domain.Reminder{
		ID:       7,
		ChatID:   42,
		Text:     " reminder\x00 ",
		NextTime: time.Date(2026, time.July, 24, 15, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60)),
		Repeat:   domain.RepeatEveryDay,
	}
}

func TestReminderUsecaseAddReminder(t *testing.T) {
	t.Run("normalizes and creates a valid reminder", func(t *testing.T) {
		repo := &reminderRepositoryStub{}
		reminder := validReminder()
		usecase := NewReminderUsecase(repo)

		err := usecase.AddReminder(t.Context(), reminder)

		require.NoError(t, err)
		assert.Same(t, reminder, repo.created)
		assert.Equal(t, "reminder", reminder.Text)
		assert.Equal(t, time.UTC, reminder.NextTime.Location())
		assert.Equal(t, 1, repo.listCalls)
	})

	t.Run("rejects invalid data before querying the repository", func(t *testing.T) {
		repo := &reminderRepositoryStub{}
		reminder := validReminder()
		reminder.Text = " \x00 "

		err := NewReminderUsecase(repo).AddReminder(t.Context(), reminder)

		require.ErrorIs(t, err, domain.ErrEmptyText)
		assert.Zero(t, repo.listCalls)
		assert.Nil(t, repo.created)
	})

	t.Run("propagates list failure", func(t *testing.T) {
		repo := &reminderRepositoryStub{err: errRepository}

		err := NewReminderUsecase(repo).AddReminder(t.Context(), validReminder())

		require.ErrorIs(t, err, errRepository)
		assert.Nil(t, repo.created)
	})

	t.Run("enforces the per-chat limit", func(t *testing.T) {
		repo := &reminderRepositoryStub{
			reminders: make([]*domain.Reminder, domain.MaxRemindersPerChat),
		}

		err := NewReminderUsecase(repo).AddReminder(t.Context(), validReminder())

		require.ErrorIs(t, err, domain.ErrTooManyReminders)
		assert.Nil(t, repo.created)
	})
}

func TestReminderUsecaseOwnedOperations(t *testing.T) {
	createdAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)

	t.Run("hides a reminder owned by another chat", func(t *testing.T) {
		repo := &reminderRepositoryStub{reminder: &domain.Reminder{ID: 7, ChatID: 99}}

		reminder, err := NewReminderUsecase(repo).GetOwned(t.Context(), 7, 42)

		require.ErrorIs(t, err, repository.ErrReminderNotFound)
		assert.Nil(t, reminder)
	})

	t.Run("preserves server-owned fields when updating", func(t *testing.T) {
		repo := &reminderRepositoryStub{
			reminder: &domain.Reminder{ID: 7, ChatID: 42, CreatedAt: createdAt},
		}
		replacement := validReminder()
		replacement.ChatID = 100
		replacement.CreatedAt = time.Time{}

		err := NewReminderUsecase(repo).UpdateOwned(t.Context(), replacement, 42)

		require.NoError(t, err)
		require.Same(t, replacement, repo.updated)
		assert.Equal(t, int64(42), repo.updated.ChatID)
		assert.Equal(t, createdAt, repo.updated.CreatedAt)
		assert.Equal(t, "reminder", repo.updated.Text)
	})

	t.Run("does not delete a reminder owned by another chat", func(t *testing.T) {
		repo := &reminderRepositoryStub{reminder: &domain.Reminder{ID: 7, ChatID: 99}}

		err := NewReminderUsecase(repo).DeleteOwned(t.Context(), 7, 42)

		require.ErrorIs(t, err, repository.ErrReminderNotFound)
		assert.Zero(t, repo.deletedID)
	})

	t.Run("changes paused state for an owned reminder", func(t *testing.T) {
		reminder := &domain.Reminder{ID: 7, ChatID: 42}
		repo := &reminderRepositoryStub{reminder: reminder}

		err := NewReminderUsecase(repo).SetPausedOwned(t.Context(), 7, 42, true)

		require.NoError(t, err)
		assert.True(t, reminder.Paused)
		assert.Same(t, reminder, repo.updated)
	})
}
