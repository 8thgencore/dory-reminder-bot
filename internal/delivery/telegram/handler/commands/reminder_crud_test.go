package commands

import (
	"context"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

func TestNextTimeAtClockUsesChatTimezoneAndCalendarDays(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		base     time.Time
		value    string
		want     time.Time
	}{
		{
			name:     "Moscow time rolls to the next local day",
			timezone: "Europe/Moscow",
			base:     time.Date(2026, time.July, 31, 7, 0, 0, 0, time.UTC),
			value:    "09:00",
			want:     time.Date(2026, time.August, 1, 6, 0, 0, 0, time.UTC),
		},
		{
			name:     "Berlin DST transition keeps wall clock",
			timezone: "Europe/Berlin",
			base:     time.Date(2025, time.March, 29, 9, 0, 0, 0, time.UTC),
			value:    "09:00",
			want:     time.Date(2025, time.March, 30, 7, 0, 0, 0, time.UTC),
		},
		{
			name:     "later UTC time stays on the same day",
			timezone: "UTC",
			base:     time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC),
			value:    "11:30",
			want:     time.Date(2026, time.July, 31, 11, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tt.timezone)
			require.NoError(t, err)

			got, err := nextTimeAtClock(tt.value, tt.base, loc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

type reminderCommandsStub struct {
	reminders []*domain.Reminder
	edited    *domain.Reminder
}

func (s *reminderCommandsStub) ListReminders(context.Context, int64) ([]*domain.Reminder, error) {
	return s.reminders, nil
}

func (s *reminderCommandsStub) EditReminder(_ context.Context, reminder *domain.Reminder) error {
	s.edited = reminder
	return nil
}

func (s *reminderCommandsStub) DeleteReminder(context.Context, int64) error { return nil }
func (s *reminderCommandsStub) PauseReminder(context.Context, int64) error  { return nil }
func (s *reminderCommandsStub) ResumeReminder(context.Context, int64) error { return nil }

type reminderChatsStub struct {
	loc *time.Location
}

func (s *reminderChatsStub) Get(context.Context, int64) (*domain.Chat, error) {
	return &domain.Chat{}, nil
}

func (s *reminderChatsStub) HasTimezone(context.Context, int64) (bool, error) {
	return true, nil
}

func (s *reminderChatsStub) Location(context.Context, int64) *time.Location {
	return s.loc
}

type reminderCommandContext struct {
	tele.Context
	chat    *tele.Chat
	message *tele.Message
	sent    []string
}

func (c *reminderCommandContext) Chat() *tele.Chat       { return c.chat }
func (c *reminderCommandContext) Message() *tele.Message { return c.message }
func (c *reminderCommandContext) Send(message any, _ ...any) error {
	c.sent = append(c.sent, message.(string))
	return nil
}

func TestOnEditInterpretsTimeInChatTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	service := &reminderCommandsStub{reminders: []*domain.Reminder{{
		ID:       1,
		ChatID:   42,
		Text:     "старый текст",
		NextTime: time.Date(2026, time.July, 31, 7, 0, 0, 0, time.UTC),
	}}}
	handler := NewReminderCRUD(service, &reminderChatsStub{loc: loc})
	ctx := &reminderCommandContext{
		chat:    &tele.Chat{ID: 42},
		message: &tele.Message{Payload: "1 09:00 новый текст"},
	}

	require.NoError(t, handler.OnEdit(ctx))
	require.NotNil(t, service.edited)
	assert.Equal(t, "новый текст", service.edited.Text)
	assert.Equal(t, time.Date(2026, time.August, 1, 6, 0, 0, 0, time.UTC), service.edited.NextTime)
}
