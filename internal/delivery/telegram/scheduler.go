package telegram

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/infrastructure/telegramapi"
	"github.com/8thgencore/dory-reminder-bot/internal/scheduling"
	tele "gopkg.in/telebot.v4"
)

const (
	// tickInterval — как часто проверяются наступившие напоминания.
	tickInterval = 30 * time.Second
	// sendConcurrency ограничивает число одновременных отправок: один недоступный чат
	// не должен задерживать всю пачку на таймаут HTTP-клиента.
	sendConcurrency = 8
	// batchTimeout ограничивает обработку одной пачки, чтобы тики не наслаивались.
	batchTimeout = 25 * time.Second
)

// sender — часть API бота, нужная планировщику. Интерфейс позволяет тестировать доставку
// без реального клиента Telegram.
type sender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
}

type reminderScheduler interface {
	ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error)
	EditReminder(ctx context.Context, reminder *domain.Reminder) error
	DeleteReminder(ctx context.Context, id int64) error
	PauseReminder(ctx context.Context, id int64) error
}

type schedulerChats interface {
	Location(ctx context.Context, chatID int64) *time.Location
	SetAvailable(ctx context.Context, chatID int64, available bool) error
}

// Scheduler рассылает наступившие напоминания и переносит их на следующий раз.
type Scheduler struct {
	bot     sender
	uc      reminderScheduler
	chatUc  schedulerChats
	nowFunc func() time.Time
}

// NewScheduler создает планировщик напоминаний.
func NewScheduler(bot sender, uc reminderScheduler, chatUc schedulerChats) *Scheduler {
	return &Scheduler{bot: bot, uc: uc, chatUc: chatUc, nowFunc: time.Now}
}

// Run опрашивает базу до отмены контекста. Вызов блокирующий.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	slog.Info("Reminder scheduler started", "interval", tickInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Reminder scheduler stopped")
			return
		case <-ticker.C:
			s.deliverDue(ctx)
		}
	}
}

func (s *Scheduler) deliverDue(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, batchTimeout)
	defer cancel()

	now := s.nowFunc()

	reminders, err := s.uc.ListDue(ctx, now)
	if err != nil {
		slog.Error("Failed to list due reminders", "error", err)
		return
	}
	if len(reminders) == 0 {
		return
	}
	slog.Info("Processing due reminders", "count", len(reminders))

	var wg sync.WaitGroup
	slots := make(chan struct{}, sendConcurrency)

	for _, r := range reminders {
		wg.Add(1)
		go func(r *domain.Reminder) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			s.deliverOne(ctx, r, now)
		}(r)
	}

	wg.Wait()
}

// deliverOne переносит напоминание и только потом отправляет его.
//
// Порядок принципиален: при обратном порядке падение записи в базу оставляло бы next_time
// в прошлом, и пользователь получал бы одно и то же напоминание каждые 30 секунд.
// Ценой этого выбора становится пропуск напоминания, если отправка не удалась, — что
// заметно лучше бесконечного потока дублей.
func (s *Scheduler) deliverOne(ctx context.Context, r *domain.Reminder, now time.Time) {
	if r.Paused {
		return
	}

	text := r.Text

	if r.Repeat == domain.RepeatNone {
		if err := s.uc.DeleteReminder(ctx, r.ID); err != nil {
			slog.Error("Failed to delete one-time reminder", "reminder_id", r.ID, "error", err)
			return
		}
	} else {
		loc := s.chatUc.Location(ctx, r.ChatID)

		next, err := scheduling.Advance(r, now, loc)
		if err != nil {
			slog.Error("Failed to compute next time, pausing reminder",
				"reminder_id", r.ID, "repeat", r.Repeat, "error", err)
			// Пересчитать время не удалось — ставим на паузу, иначе напоминание
			// останется навсегда просроченным и будет отправляться каждый тик.
			if err := s.uc.PauseReminder(ctx, r.ID); err != nil {
				slog.Error("Failed to pause broken reminder", "reminder_id", r.ID, "error", err)
			}

			return
		}

		r.NextTime = next
		r.UpdatedAt = now
		if err := s.uc.EditReminder(ctx, r); err != nil {
			slog.Error("Failed to reschedule reminder", "reminder_id", r.ID, "error", err)
			return
		}
		slog.Info("Reminder rescheduled", "reminder_id", r.ID, "next_time", next)
	}

	if _, err := s.bot.Send(&tele.Chat{ID: r.ChatID}, texts.ReminderPrefix+text); err != nil {
		if telegramapi.IsBotUnavailable(err) {
			if stateErr := s.chatUc.SetAvailable(ctx, r.ChatID, false); stateErr != nil {
				slog.Error(
					"Failed to freeze unavailable chat",
					"chat_id", r.ChatID,
					"error", stateErr,
				)
			} else {
				slog.Warn("Telegram bot is unavailable in chat", "chat_id", r.ChatID, "error", err)
			}
			return
		}
		slog.Error("Failed to send reminder", "chat_id", r.ChatID, "reminder_id", r.ID, "error", err)
		return
	}
	slog.Info("Reminder sent", "chat_id", r.ChatID, "reminder_id", r.ID)
}
