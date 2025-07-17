package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/texts"
	usecase_domain "github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/8thgencore/dory-reminder-bot/pkg/validator"
	tele "gopkg.in/telebot.v4"
)

var (
	welcomeTextNoTZ = "Привет! Я бот-напоминалка. 🌍\n\nСначала установите ваш часовой пояс командой /timezone"
	welcomeText     = `🤖 *Dory Reminder Bot*

Привет! Я бот для создания и управления напоминаниями.`
	helpText = `*Справка по командам:*

/help - справка по командам
/add - добавить напоминание
/list - список напоминаний
/edit - редактировать напоминание
/delete - удалить напоминание
/pause - поставить на паузу
/resume - возобновить напоминание
/timezone - установить часовой пояс`
)

// checkTimezone проверяет, установлен ли таймзона у пользователя.
func (h *Handler) checkTimezone(c tele.Context) (bool, error) {
	return h.UserUsecase.HasTimezone(context.Background(), c.Chat().ID, c.Sender().ID)
}

// getReminders возвращает список напоминаний для чата.
func (h *Handler) getReminders(chatID int64) ([]*usecase_domain.Reminder, error) {
	return h.Usecase.ListReminders(context.Background(), chatID)
}

// getReminderNumber возвращает номер напоминания из строки аргумента.
func getReminderNumber(arg string) (int, error) {
	num, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("Некорректный номер")
	}
	return num, nil
}

// HandleStart обрабатывает команду /start.
func (h *Handler) HandleStart(c tele.Context, userUc usecase.UserUsecase) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	username := c.Sender().Username
	firstName := c.Sender().FirstName
	lastName := c.Sender().LastName

	slog.Info("User started bot", "user_id", userID, "chat_id", chatID, "username", username)

	_, err := userUc.GetOrCreateUser(context.Background(), chatID, userID, username, firstName, lastName)
	if err != nil {
		return c.Send(texts.ErrInitUser)
	}
	hasTZ, err := userUc.HasTimezone(context.Background(), chatID, userID)
	if err != nil {
		return c.Send(texts.ErrCheckSettings)
	}
	if !hasTZ {
		return c.Send(texts.WelcomeTextNoTZ)
	}
	return c.Send(texts.WelcomeText, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, h.GetMainMenu())
}

// HandleHelp обрабатывает команду /help.
func (h *Handler) HandleHelp(c tele.Context) error {
	slog.Info("User requested help", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return c.Send(texts.HelpText)
}

func (h *Handler) onAdd(c tele.Context) error {
	hasTZ, err := h.checkTimezone(c)
	if err != nil {
		return c.Send(texts.ErrCheckSettings)
	}
	if !hasTZ {
		return c.Send("⚠️ Сначала установите часовой пояс командой /timezone")
	}
	if c.Message().Payload != "" {
		return c.Send("Для создания напоминания используйте мастер через /add без параметров.")
	}
	return c.Send(texts.AddTypePrompt)
}

// Коллбэки для типов напоминаний
func (h *Handler) cbAddToday(c tele.Context) error    { return h.HandleAddTypeCallback(c, "today") }
func (h *Handler) cbAddTomorrow(c tele.Context) error { return h.HandleAddTypeCallback(c, "tomorrow") }
func (h *Handler) cbAddMultiDay(c tele.Context) error { return h.HandleAddTypeCallback(c, "multiday") }
func (h *Handler) cbAddEveryDay(c tele.Context) error { return h.HandleAddTypeCallback(c, "everyday") }
func (h *Handler) cbAddWeek(c tele.Context) error     { return h.HandleAddTypeCallback(c, "week") }
func (h *Handler) cbAddNDays(c tele.Context) error    { return h.HandleAddTypeCallback(c, "ndays") }
func (h *Handler) cbAddMonth(c tele.Context) error    { return h.HandleAddTypeCallback(c, "month") }
func (h *Handler) cbAddYear(c tele.Context) error     { return h.HandleAddTypeCallback(c, "year") }
func (h *Handler) cbAddDate(c tele.Context) error     { return h.HandleAddTypeCallback(c, "date") }

// Обработка текстовых сообщений (мастер добавления/таймзона)
func (h *Handler) onText(c tele.Context) error {
	sess := h.Session.Get(c.Chat().ID, c.Sender().ID)
	if sess != nil && sess.Step == session.StepTimezone {
		return h.HandleTimezoneText(c)
	}
	if sess != nil && (sess.Step == session.StepTime || sess.Step == session.StepText || sess.Step == session.StepInterval) {
		return h.HandleAddWizardText(c)
	}
	return nil
}

// Пагинация: сколько напоминаний на страницу
const remindersPerPage = 10

// Форматирование режима повтора
func formatRepeat(r *usecase_domain.Reminder) string {
	switch r.Repeat {
	case usecase_domain.RepeatNone:
		return "разово"
	case usecase_domain.RepeatEveryDay:
		return "ежедневно"
	case usecase_domain.RepeatEveryWeek:
		return "еженедельно"
	case usecase_domain.RepeatEveryMonth:
		return "ежемесячно"
	case usecase_domain.RepeatEveryNDays:
		return fmt.Sprintf("каждые %d дней", r.RepeatEvery)
	default:
		return "-"
	}
}

// Список напоминаний с пагинацией
func (h *Handler) onList(c tele.Context) error {
	reminders, err := h.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send(texts.ErrGetReminders)
	}
	if len(reminders) == 0 {
		return c.Send(texts.ErrNoReminders)
	}
	page := 0
	if cb := c.Callback(); cb != nil && strings.HasPrefix(cb.Data, "rem_page_") {
		if p, err := strconv.Atoi(strings.TrimPrefix(cb.Data, "rem_page_")); err == nil && p >= 0 {
			page = p
		}
	}
	start, end := page*remindersPerPage, (page+1)*remindersPerPage
	if end > len(reminders) {
		end = len(reminders)
	}
	msg := "📋 Ваши напоминания:\n\n"
	for i := start; i < end; i++ {
		r := reminders[i]
		status := "✅"
		if r.Paused {
			status = "⏸️"
		}
		msg += fmt.Sprintf("%s %d. %s\n   📅 %s\n   🔁 %s\n\n", status, i+1, r.Text, r.NextTime.Format("02.01.2006 15:04"), formatRepeat(r))
	}
	var nav tele.ReplyMarkup
	rows := []tele.Row{}
	if start > 0 {
		rows = append(rows, nav.Row(nav.Data("⬅ Назад", "rem_page_"+strconv.Itoa(page-1))))
	}
	if end < len(reminders) {
		rows = append(rows, nav.Row(nav.Data("Далее ➡", "rem_page_"+strconv.Itoa(page+1))))
	}
	if len(rows) > 0 {
		nav.Inline(rows...)
		if c.Callback() != nil {
			return c.Edit(msg, &nav)
		}
		return c.Send(msg, &nav)
	}
	if c.Callback() != nil {
		return c.Edit(msg)
	}
	return c.Send(msg)
}

func (h *Handler) onEdit(c tele.Context) error {
	args := strings.Fields(strings.TrimSpace(c.Message().Payload))
	if len(args) < 2 {
		return c.Send("Формат: /edit <номер> <новый текст> или /edit <номер> <время> <новый текст>")
	}
	num, err := getReminderNumber(args[0])
	if err != nil {
		return c.Send("Ошибка: укажите корректный номер напоминания из списка")
	}
	reminders, err := h.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if num > len(reminders) {
		return c.Send("Нет напоминания с таким номером")
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
			return c.Send("Время должно быть в формате 15:00")
		}
		rem.NextTime = validator.NextTimeFromString(newTime, rem.NextTime)
	}
	if newText != "" {
		rem.Text = newText
	}
	err = h.Usecase.EditReminder(context.Background(), rem)
	if err != nil {
		return c.Send("Ошибка при обновлении напоминания")
	}
	return c.Send("Напоминание обновлено!")
}

// Удаление, пауза, возобновление — общий шаблон
func (h *Handler) handleReminderAction(c tele.Context, action string, do func(remID int64) error) error {
	arg := strings.TrimSpace(c.Message().Payload)
	num, err := getReminderNumber(arg)
	if err != nil {
		return c.Send("Ошибка: укажите корректный номер напоминания из списка")
	}
	reminders, err := h.getReminders(c.Chat().ID)
	if err != nil {
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if num > len(reminders) {
		return c.Send("Нет напоминания с таким номером")
	}
	rem := reminders[num-1]
	if err := do(rem.ID); err != nil {
		return c.Send(fmt.Sprintf("Ошибка при %s напоминания", action))
	}
	return c.Send(fmt.Sprintf("%s Напоминание %s!", map[string]string{"delete": "🗑️", "pause": "⏸️", "resume": "▶️"}[action], map[string]string{"delete": "удалено", "pause": "поставлено на паузу", "resume": "возобновлено"}[action]))
}

func (h *Handler) onDelete(c tele.Context) error {
	return h.handleReminderAction(c, "delete", func(remID int64) error {
		return h.Usecase.DeleteReminder(context.Background(), remID)
	})
}

func (h *Handler) onPause(c tele.Context) error {
	return h.handleReminderAction(c, "pause", func(remID int64) error {
		return h.Usecase.PauseReminder(context.Background(), remID)
	})
}

func (h *Handler) onResume(c tele.Context) error {
	return h.handleReminderAction(c, "resume", func(remID int64) error {
		return h.Usecase.ResumeReminder(context.Background(), remID)
	})
}

func (h *Handler) onTimezone(c tele.Context) error {
	h.Session.Set(&session.AddReminderSession{
		UserID: c.Sender().ID,
		ChatID: c.Chat().ID,
		Step:   session.StepTimezone,
	})
	return c.Send("🌍 Введите ваш часовой пояс в формате IANA (например, Europe/Moscow, America/New_York, Asia/Tokyo):")
}
