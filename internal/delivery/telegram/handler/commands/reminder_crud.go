package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/ui"
	usecase_domain "github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/8thgencore/dory-reminder-bot/pkg/validator"
	tele "gopkg.in/telebot.v4"
)

// Пагинация: сколько напоминаний на страницу
const remindersPerPage = 10

// ReminderCRUD содержит обработчики CRUD операций с напоминаниями
type ReminderCRUD struct {
	Usecase     usecase.ReminderUsecase
	ChatUsecase usecase.ChatUsecase
}

// NewReminderCRUD создает новый экземпляр ReminderCRUD
func NewReminderCRUD(reminderUc usecase.ReminderUsecase, chatUc usecase.ChatUsecase) *ReminderCRUD {
	return &ReminderCRUD{
		Usecase:     reminderUc,
		ChatUsecase: chatUc,
	}
}

// checkTimezone проверяет, установлен ли таймзона у пользователя
func (rc *ReminderCRUD) checkTimezone(c tele.Context) (bool, error) {
	return rc.ChatUsecase.HasTimezone(context.Background(), c.Chat().ID)
}

// getReminders возвращает список напоминаний для чата
func (rc *ReminderCRUD) getReminders(chatID int64) ([]*usecase_domain.Reminder, error) {
	return rc.Usecase.ListReminders(context.Background(), chatID)
}

// getReminderNumber возвращает номер напоминания из строки аргумента
func getReminderNumber(arg string) (int, error) {
	num, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || num <= 0 {
		return 0, errors.New("некорректный номер")
	}
	return num, nil
}

// OnAdd обрабатывает команду /add
func (rc *ReminderCRUD) OnAdd(c tele.Context) error {
	hasTZ, err := rc.checkTimezone(c)
	if err != nil {
		return c.Send(texts.ErrCheckSettings)
	}
	if !hasTZ {
		return c.Send(texts.TimezoneRequired)
	}
	if c.Message().Payload != "" {
		return c.Send(texts.AddViaWizardOnly)
	}

	return c.Send(texts.HelpAdd, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, ui.GetAddMenu())
}

// OnList обрабатывает команду /list
func (rc *ReminderCRUD) OnList(c tele.Context) error {
	reminders, err := rc.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send(texts.ErrGetReminders)
	}
	if len(reminders) == 0 {
		return c.Send(texts.ErrNoReminders)
	}

	// Получаем часовой пояс пользователя
	loc := time.UTC
	if ch, err := rc.ChatUsecase.Get(context.Background(), c.Chat().ID); err == nil && ch != nil && ch.Timezone != "" {
		if l, err := time.LoadLocation(ch.Timezone); err == nil {
			loc = l
		}
	}

	page := 0
	if cb := c.Callback(); cb != nil {
		data := strings.TrimSpace(cb.Data)
		if after, ok := strings.CutPrefix(data, "rem_page_"); ok {
			if p, err := strconv.Atoi(after); err == nil && p >= 0 {
				page = p
			}
		}
	}

	start, end := page*remindersPerPage, (page+1)*remindersPerPage
	if end > len(reminders) {
		end = len(reminders)
	}

	// Сообщение уходит в режиме MarkdownV2, поэтому экранировать нужно всё
	// подставляемое — не только текст пользователя, но и даты с описанием повтора:
	// точки, скобки и дефисы в них тоже спецсимволы.
	var builder strings.Builder
	builder.WriteString(texts.RemindersHeader + "\n\n")

	if ch, err := rc.ChatUsecase.Get(context.Background(), c.Chat().ID); err == nil && ch != nil && ch.Timezone != "" {
		builder.WriteString(fmt.Sprintf("🕐 *Часовой пояс:* %s\n\n", ui.EscapeMarkdownV2(ch.Timezone)))
	}

	for i := start; i < end; i++ {
		r := reminders[i]

		status := ui.FormatStatus(r.Paused)
		timeStr := ui.EscapeMarkdownV2(ui.FormatTime(r.NextTime, loc))
		repeatStr := ui.EscapeMarkdownV2(ui.FormatRepeat(r))

		builder.WriteString(fmt.Sprintf("*%d\\.* %s\n", i+1, ui.EscapeMarkdownV2(r.Text)))

		// Отображаем статус только если напоминание приостановлено
		if status != "" {
			builder.WriteString(fmt.Sprintf("   %s \\| 📅 %s\n", ui.EscapeMarkdownV2(status), timeStr))
		} else {
			builder.WriteString(fmt.Sprintf("   📅 %s\n", timeStr))
		}

		builder.WriteString(fmt.Sprintf("   🔁 %s\n", repeatStr))
		builder.WriteString("\n")
	}

	msg := builder.String()

	var nav tele.ReplyMarkup
	rows := []tele.Row{}
	if start > 0 {
		rows = append(rows, nav.Row(nav.Data("⬅ Назад", "rem_page_"+strconv.Itoa(page-1))))
	}
	if end < len(reminders) {
		rows = append(rows, nav.Row(nav.Data("Далее ➡", "rem_page_"+strconv.Itoa(page+1))))
	}

	options := &tele.SendOptions{ParseMode: tele.ModeMarkdownV2}

	if len(rows) > 0 {
		nav.Inline(rows...)
		if c.Callback() != nil {
			return c.Edit(msg, options, &nav)
		}
		return c.Send(msg, options, &nav)
	}

	if c.Callback() != nil {
		return c.Edit(msg, options)
	}

	return c.Send(msg, options)
}

// OnEdit обрабатывает команду /edit
func (rc *ReminderCRUD) OnEdit(c tele.Context) error {
	args := strings.Fields(strings.TrimSpace(c.Message().Payload))
	if len(args) < 2 {
		return c.Send(texts.EditUsage)
	}

	num, err := getReminderNumber(args[0])
	if err != nil {
		return c.Send(texts.ErrWrongNumber)
	}

	reminders, err := rc.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send(texts.ErrGetReminders)
	}
	if num > len(reminders) {
		return c.Send(texts.ErrNoSuchReminder)
	}

	rem := reminders[num-1]

	// Если второй аргумент — время, то обновляем и время, и текст
	newTime := ""
	newText := ""
	if len(args) >= 3 && validator.IsTime(args[1]) {
		newTime = args[1]
		newText = strings.Join(args[2:], " ")
	} else {
		newText = strings.Join(args[1:], " ")
	}

	if newTime != "" {
		if !validator.IsTime(newTime) {
			return c.Send(texts.EditTimeFormat)
		}
		rem.NextTime = validator.NextTimeFromString(newTime, rem.NextTime)
	}
	if newText != "" {
		rem.Text = newText
	}

	if err := rc.Usecase.EditReminder(context.Background(), rem); err != nil {
		return c.Send(texts.ErrUpdateReminder)
	}

	return c.Send(texts.ReminderUpdated)
}

// handleReminderAction — общий шаблон для удаления, паузы и возобновления.
//
// Номер приходит из вывода /list, поэтому список запрашивается тем же запросом
// с той же сортировкой: иначе номер указал бы на другое напоминание.
func (rc *ReminderCRUD) handleReminderAction(
	c tele.Context,
	errMsg, successMsg string,
	do func(remID int64) error,
) error {
	num, err := getReminderNumber(strings.TrimSpace(c.Message().Payload))
	if err != nil {
		return c.Send(texts.ErrWrongNumber)
	}

	reminders, err := rc.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send(texts.ErrGetReminders)
	}
	if num > len(reminders) {
		return c.Send(texts.ErrNoSuchReminder)
	}

	if err := do(reminders[num-1].ID); err != nil {
		return c.Send(errMsg)
	}

	return c.Send(successMsg)
}

// OnDelete обрабатывает команду /delete
func (rc *ReminderCRUD) OnDelete(c tele.Context) error {
	return rc.handleReminderAction(c, texts.ErrDeleteReminder, texts.ReminderDeleted, func(remID int64) error {
		return rc.Usecase.DeleteReminder(context.Background(), remID)
	})
}

// OnPause обрабатывает команду /pause
func (rc *ReminderCRUD) OnPause(c tele.Context) error {
	return rc.handleReminderAction(c, texts.ErrPauseReminder, texts.ReminderPaused, func(remID int64) error {
		return rc.Usecase.PauseReminder(context.Background(), remID)
	})
}

// OnResume обрабатывает команду /resume
func (rc *ReminderCRUD) OnResume(c tele.Context) error {
	return rc.handleReminderAction(c, texts.ErrResumeReminder, texts.ReminderResumed, func(remID int64) error {
		return rc.Usecase.ResumeReminder(context.Background(), remID)
	})
}
