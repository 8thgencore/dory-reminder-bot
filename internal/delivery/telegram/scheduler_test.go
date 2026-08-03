package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// --- Заглушки -------------------------------------------------------------

type sentMessage struct {
	chatID int64
	text   string
}

type stubSender struct {
	mu   sync.Mutex
	sent []sentMessage
	err  error
}

func (s *stubSender) Send(to tele.Recipient, what any, _ ...any) (*tele.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return nil, s.err
	}

	chat, _ := to.(*tele.Chat)
	text, _ := what.(string)
	s.sent = append(s.sent, sentMessage{chatID: chat.ID, text: text})

	return &tele.Message{}, nil
}

func (s *stubSender) messages() []sentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]sentMessage(nil), s.sent...)
}

// stubReminderUC хранит напоминания в памяти и умеет ломать запись по требованию.
type stubReminderUC struct {
	mu        sync.Mutex
	reminders map[int64]*domain.Reminder
	editErr   error
	deleteErr error
	edits     int
	deletes   int
	pauses    int
}

func newStubReminderUC(reminders ...*domain.Reminder) *stubReminderUC {
	s := &stubReminderUC{reminders: make(map[int64]*domain.Reminder)}
	for _, r := range reminders {
		s.reminders[r.ID] = r
	}

	return s
}

func (s *stubReminderUC) ListDue(_ context.Context, now time.Time) ([]*domain.Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var due []*domain.Reminder
	for _, r := range s.reminders {
		if !r.Paused && !r.NextTime.After(now) {
			copied := *r
			due = append(due, &copied)
		}
	}

	return due, nil
}

func (s *stubReminderUC) EditReminder(_ context.Context, r *domain.Reminder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.edits++
	if s.editErr != nil {
		return s.editErr
	}
	copied := *r
	s.reminders[r.ID] = &copied

	return nil
}

func (s *stubReminderUC) DeleteReminder(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.reminders, id)

	return nil
}

func (s *stubReminderUC) PauseReminder(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pauses++
	if r, ok := s.reminders[id]; ok {
		r.Paused = true
	}

	return nil
}

func (s *stubReminderUC) get(id int64) *domain.Reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reminders[id]
}

func (s *stubReminderUC) counts() (edits, deletes, pauses int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.edits, s.deletes, s.pauses
}

type stubChatUC struct {
	loc             *time.Location
	availabilitySet bool
	availableChatID int64
	available       bool
}

func (s *stubChatUC) Location(context.Context, int64) *time.Location {
	if s.loc == nil {
		return time.UTC
	}

	return s.loc
}

func (s *stubChatUC) SetAvailable(_ context.Context, chatID int64, available bool) error {
	s.availabilitySet = true
	s.availableChatID = chatID
	s.available = available
	return nil
}

// --- Тесты ----------------------------------------------------------------

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	return loc
}

func TestDeliverDue_SendsAndReschedulesRepeating(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, loc)

	rem := &domain.Reminder{
		ID:       1,
		ChatID:   100,
		Text:     "выпить воды",
		NextTime: time.Date(2025, time.June, 10, 9, 0, 0, 0, loc).UTC(),
		Repeat:   domain.RepeatEveryDay,
	}

	uc := newStubReminderUC(rem)
	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{loc: loc})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	sent := bot.messages()
	require.Len(t, sent, 1)
	assert.Equal(t, int64(100), sent[0].chatID)
	assert.Contains(t, sent[0].text, "выпить воды")

	stored := uc.get(1)
	require.NotNil(t, stored)
	assert.True(t, stored.NextTime.After(now), "next time must move into the future")
	assert.Equal(t, 9, stored.NextTime.In(loc).Hour())
	assert.Equal(t, 11, stored.NextTime.In(loc).Day())
}

func TestDeliverDue_DeletesOneTimeReminder(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "разовое",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatNone,
	})
	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Len(t, bot.messages(), 1)
	assert.Nil(t, uc.get(1), "one-time reminder must be removed after firing")
}

// Ключевая проверка: если запись в базу упала, напоминание не должно уйти
// пользователю — иначе оно будет приходить каждые 30 секунд, пока база не оживёт.
func TestDeliverDue_DoesNotSendWhenRescheduleFails(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "ежедневное",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryDay,
	})
	uc.editErr = errors.New("database is locked")

	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Empty(t, bot.messages(), "reminder must not be sent if it could not be rescheduled")
}

func TestDeliverDue_DoesNotSendWhenDeleteFails(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "разовое",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatNone,
	})
	uc.deleteErr = errors.New("database is locked")

	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Empty(t, bot.messages())
}

// Битые данные не должны приводить к бесконечной рассылке: такое напоминание
// ставится на паузу.
func TestDeliverDue_PausesReminderWithBrokenRepeat(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "битое",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryNDays, RepeatEvery: 0,
	})

	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Empty(t, bot.messages())
	_, _, pauses := uc.counts()
	assert.Equal(t, 1, pauses, "broken reminder must be paused")
}

func TestDeliverDue_SkipsPausedReminders(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "на паузе",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryDay, Paused: true,
	})

	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Empty(t, bot.messages())
	edits, deletes, _ := uc.counts()
	assert.Zero(t, edits)
	assert.Zero(t, deletes)
}

// Недоступный чат не должен задерживать остальные: отправка идёт параллельно,
// а сдвиг времени уже сохранён.
func TestDeliverDue_SendFailureStillReschedules(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: 100, Text: "ежедневное",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryDay,
	})

	bot := &stubSender{err: errors.New("chat not found")}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	stored := uc.get(1)
	require.NotNil(t, stored)
	assert.True(t, stored.NextTime.After(now))
}

func TestDeliverDue_KickedBotFreezesChat(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)
	uc := newStubReminderUC(&domain.Reminder{
		ID: 1, ChatID: -1004314310091, Text: "ежедневное",
		NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryDay,
	})
	bot := &stubSender{err: tele.ErrKickedFromSuperGroup}
	chatUC := &stubChatUC{}
	s := NewScheduler(bot, uc, chatUC)
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.True(t, chatUC.availabilitySet)
	assert.Equal(t, int64(-1004314310091), chatUC.availableChatID)
	assert.False(t, chatUC.available)
}

func TestDeliverDue_HandlesBatch(t *testing.T) {
	now := time.Date(2025, time.June, 10, 9, 0, 30, 0, time.UTC)

	var reminders []*domain.Reminder
	for i := int64(1); i <= 20; i++ {
		reminders = append(reminders, &domain.Reminder{
			ID: i, ChatID: 100 + i, Text: "пачка",
			NextTime: now.Add(-time.Minute), Repeat: domain.RepeatEveryDay,
		})
	}

	uc := newStubReminderUC(reminders...)
	bot := &stubSender{}
	s := NewScheduler(bot, uc, &stubChatUC{})
	s.nowFunc = func() time.Time { return now }

	s.deliverDue(context.Background())

	assert.Len(t, bot.messages(), 20)
}

// Run обязан завершаться по отмене контекста, иначе процесс не остановится
// по SIGTERM.
func TestRun_StopsOnContextCancel(t *testing.T) {
	s := NewScheduler(&stubSender{}, newStubReminderUC(), &stubChatUC{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		s.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop on context cancellation")
	}
}
