package telegram

import (
	"context"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// StartScheduler запускает планировщик напоминаний
func StartScheduler(bot *tele.Bot, uc usecase.ReminderUsecase, chatUc usecase.ChatUsecase) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			deliverDueReminders(bot, uc, chatUc)
		}
	}()
}

func deliverDueReminders(bot *tele.Bot, uc usecase.ReminderUsecase, chatUc usecase.ChatUsecase) {
	now := time.Now()
	reminders, err := uc.ListDue(context.Background(), now)
	if err != nil {
		slog.Error("Failed to list due reminders", "error", err)
		return
	}
	if len(reminders) > 0 {
		slog.Info("Processing due reminders", "count", len(reminders))
	}
	for _, r := range reminders {
		if r.Paused {
			continue
		}
		chat := &tele.Chat{ID: r.ChatID}
		_, err := bot.Send(chat, "⏰ Напоминание: "+r.Text)
		if err != nil {
			slog.Error("Failed to send reminder", "chat_id", r.ChatID, "reminder_id", r.ID, "error", err)
			continue
		}
		slog.Info("Reminder sent", "chat_id", r.ChatID, "reminder_id", r.ID, "text", r.Text)
		// Обработка повторов и обновление next_time
		if r.Repeat == domain.RepeatNone {
			if err := uc.DeleteReminder(context.Background(), r.ID); err != nil {
				slog.Error("Failed to delete reminder", "reminder_id", r.ID, "error", err)
			} else {
				slog.Info("One-time reminder deleted", "reminder_id", r.ID)
			}
		} else {
			// Получаем часовой пояс чата
			loc := time.UTC
			if ch, err := chatUc.Get(context.Background(), r.ChatID); err == nil && ch != nil && ch.Timezone != "" {
				if l, err := time.LoadLocation(ch.Timezone); err == nil {
					loc = l
				}
			}

			next := calcNextTimeForward(r, now, loc)
			r.NextTime = next
			r.UpdatedAt = now
			if err := uc.EditReminder(context.Background(), r); err != nil {
				slog.Error("Failed to edit reminder", "reminder_id", r.ID, "error", err)
			} else {
				slog.Info("Repeating reminder updated", "reminder_id", r.ID, "next_time", next)
			}
		}
	}
}

// calcNextTimeForward: пересчитывает next_time для повторяющихся напоминаний так,
// чтобы оно всегда было в будущем относительно now (даже если бот был выключен долго)
func calcNextTimeForward(r *domain.Reminder, now time.Time, loc *time.Location) time.Time {
	// Конвертируем время в часовой пояс пользователя для правильного расчета
	nextInUserTZ := r.NextTime.In(loc)
	nowInUserTZ := now.In(loc)

	next := nextInUserTZ
	for !next.After(nowInUserTZ) {
		switch r.Repeat {
		case domain.RepeatEveryDay:
			next = next.Add(24 * time.Hour)
		case domain.RepeatEveryWeek:
			next = next.Add(7 * 24 * time.Hour)
		case domain.RepeatEveryMonth:
			next = next.AddDate(0, 1, 0)
		case domain.RepeatEveryNDays:
			next = next.AddDate(0, 0, r.RepeatEvery)
		case domain.RepeatEveryYear:
			next = next.AddDate(1, 0, 0)
		default:
			return now
		}
	}

	// Возвращаем время в UTC для сохранения в БД
	return next.UTC()
}
