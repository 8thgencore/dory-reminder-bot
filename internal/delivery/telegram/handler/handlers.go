package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	usecase_domain "github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
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

func (h *Handler) HandleStart(c tele.Context, userUc usecase.UserUsecase) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	username := c.Sender().Username
	firstName := c.Sender().FirstName
	lastName := c.Sender().LastName

	slog.Info("User started bot", "user_id", userID, "chat_id", chatID, "username", username)

	// Create or update user
	_, err := userUc.GetOrCreateUser(context.Background(), chatID, userID, username, firstName, lastName)
	if err != nil {
		slog.Error("Failed to create/update user", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при инициализации пользователя")
	}

	// Check if user has timezone set
	hasTZ, err := userUc.HasTimezone(context.Background(), chatID, userID)
	if err != nil {
		slog.Error("Failed to check user timezone", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при проверке настроек пользователя")
	}

	if !hasTZ {
		return c.Send(welcomeTextNoTZ)
	}

	return c.Send(welcomeText, &tele.SendOptions{
		ParseMode: tele.ModeMarkdown,
	}, h.GetMainMenu())
}

func (h *Handler) HandleHelp(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested help", "user_id", userID, "chat_id", chatID)

	return c.Send(helpText)
}

func (h *Handler) onAdd(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User started add reminder wizard", "user_id", userID, "chat_id", chatID, "chat_type", c.Chat().Type)

	// Check if user has timezone set
	hasTZ, err := h.UserUsecase.HasTimezone(context.Background(), chatID, userID)
	if err != nil {
		slog.Error("Failed to check user timezone", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при проверке настроек пользователя")
	}

	if !hasTZ {
		return c.Send("⚠️ Сначала установите часовой пояс командой /timezone")
	}

	if c.Message().Payload != "" {
		return c.Send("Для создания напоминания используйте мастер через /add без параметров.")
	}
	return c.Send("Выберите тип напоминания:", addMenu)
}

func (h *Handler) cbAddToday(c tele.Context) error    { return h.HandleAddTypeCallback(c, "today") }
func (h *Handler) cbAddTomorrow(c tele.Context) error { return h.HandleAddTypeCallback(c, "tomorrow") }
func (h *Handler) cbAddMultiDay(c tele.Context) error { return h.HandleAddTypeCallback(c, "multiday") }
func (h *Handler) cbAddEveryDay(c tele.Context) error { return h.HandleAddTypeCallback(c, "everyday") }
func (h *Handler) cbAddWeek(c tele.Context) error     { return h.HandleAddTypeCallback(c, "week") }
func (h *Handler) cbAddNDays(c tele.Context) error    { return h.HandleAddTypeCallback(c, "ndays") }
func (h *Handler) cbAddMonth(c tele.Context) error    { return h.HandleAddTypeCallback(c, "month") }
func (h *Handler) cbAddYear(c tele.Context) error     { return h.HandleAddTypeCallback(c, "year") }
func (h *Handler) cbAddDate(c tele.Context) error     { return h.HandleAddTypeCallback(c, "date") }

func (h *Handler) onText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.Session.Get(chatID, userID)

	if sess != nil && sess.Step == StepTimezone {
		return h.HandleTimezoneText(c)
	}

	// Check if user is in add wizard
	if sess != nil && (sess.Step == StepTime || sess.Step == StepText) {
		return h.HandleAddWizardText(c)
	}

	return nil // игнорировать, если не в мастере
}

// Пагинация: сколько напоминаний на страницу
const remindersPerPage = 10

// Добавляем вспомогательную функцию для отображения режима
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

// Обработчик для показа списка с пагинацией
func (h *Handler) onList(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested reminders list", "user_id", userID, "chat_id", chatID)

	reminders, err := h.Usecase.ListReminders(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to get reminders list", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if len(reminders) == 0 {
		slog.Info("User has no reminders", "user_id", userID, "chat_id", chatID)
		return c.Send("Нет напоминаний")
	}

	// Пагинация
	page := 0
	if c.Callback() != nil {
		// Если это callback, то читаем номер страницы из данных
		data := c.Callback().Data
		if strings.HasPrefix(data, "rem_page_") {
			p, err := strconv.Atoi(strings.TrimPrefix(data, "rem_page_"))
			if err == nil && p >= 0 {
				page = p
			}
		}
	}
	start := page * remindersPerPage
	end := start + remindersPerPage
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
		mode := formatRepeat(r)
		msg += fmt.Sprintf("%s %d. %s\n   📅 %s\n   🔁 %s\n\n", status, i+1, r.Text, r.NextTime.Format("02.01.2006 15:04"), mode)
	}

	// Кнопки пагинации
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

func formatReminder(idx int, r *usecase_domain.Reminder) string {
	return fmt.Sprintf("%d. %s (%s)", idx, r.Text, r.NextTime.Format("02.01.2006 15:04"))
}

func (h *Handler) onEdit(c tele.Context) error {
	return c.Send("Редактирование напоминания в разработке")
}

func (h *Handler) onDelete(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	arg := strings.TrimSpace(c.Message().Payload)

	if arg == "" {
		return c.Send("Формат: /delete <номер из списка>. Например: /delete 2")
	}
	num, err := strconv.Atoi(arg)
	if err != nil || num <= 0 {
		slog.Warn("Invalid delete reminder number", "user_id", userID, "chat_id", chatID, "input", arg)
		return c.Send("Ошибка: укажите корректный номер напоминания из списка")
	}

	reminders, err := h.Usecase.ListReminders(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to get reminders for deletion", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if num > len(reminders) {
		slog.Warn("Delete reminder number out of range", "user_id", userID, "chat_id", chatID, "number", num, "total", len(reminders))
		return c.Send("Нет напоминания с таким номером")
	}

	rem := reminders[num-1]
	err = h.Usecase.DeleteReminder(context.Background(), rem.ID)
	if err != nil {
		slog.Error("Failed to delete reminder", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "error", err)
		return c.Send("Ошибка при удалении напоминания")
	}

	slog.Info("Reminder deleted", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "text", rem.Text)
	return c.Send("🗑️ Напоминание удалено!")
}

func (h *Handler) onPause(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	arg := strings.TrimSpace(c.Message().Payload)

	if arg == "" {
		return c.Send("Формат: /pause <номер из списка>. Например: /pause 2")
	}
	num, err := strconv.Atoi(arg)
	if err != nil || num <= 0 {
		slog.Warn("Invalid pause reminder number", "user_id", userID, "chat_id", chatID, "input", arg)
		return c.Send("Ошибка: укажите корректный номер напоминания из списка")
	}

	reminders, err := h.Usecase.ListReminders(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to get reminders for pause", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if num > len(reminders) {
		slog.Warn("Pause reminder number out of range", "user_id", userID, "chat_id", chatID, "number", num, "total", len(reminders))
		return c.Send("Нет напоминания с таким номером")
	}

	rem := reminders[num-1]
	err = h.Usecase.PauseReminder(context.Background(), rem.ID)
	if err != nil {
		slog.Error("Failed to pause reminder", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "error", err)
		return c.Send("Ошибка при паузе напоминания")
	}

	slog.Info("Reminder paused", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "text", rem.Text)
	return c.Send("⏸️ Напоминание поставлено на паузу!")
}

func (h *Handler) onResume(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	arg := strings.TrimSpace(c.Message().Payload)

	if arg == "" {
		return c.Send("Формат: /resume <номер из списка>. Например: /resume 2")
	}
	num, err := strconv.Atoi(arg)
	if err != nil || num <= 0 {
		slog.Warn("Invalid resume reminder number", "user_id", userID, "chat_id", chatID, "input", arg)
		return c.Send("Ошибка: укажите корректный номер напоминания из списка")
	}

	reminders, err := h.Usecase.ListReminders(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to get reminders for resume", "user_id", userID, "chat_id", chatID, "error", err)
		return c.Send("Ошибка при получении списка напоминаний")
	}
	if num > len(reminders) {
		slog.Warn("Resume reminder number out of range", "user_id", userID, "chat_id", chatID, "number", num, "total", len(reminders))
		return c.Send("Нет напоминания с таким номером")
	}

	rem := reminders[num-1]
	err = h.Usecase.ResumeReminder(context.Background(), rem.ID)
	if err != nil {
		slog.Error("Failed to resume reminder", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "error", err)
		return c.Send("Ошибка при возобновлении напоминания")
	}

	slog.Info("Reminder resumed", "user_id", userID, "chat_id", chatID, "reminder_id", rem.ID, "text", rem.Text)
	return c.Send("▶️ Напоминание возобновлено!")
}

func (h *Handler) onTimezone(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested timezone setup", "user_id", userID, "chat_id", chatID)

	// Create session for timezone input
	session := &AddReminderSession{
		UserID: userID,
		ChatID: chatID,
		Step:   StepTimezone,
	}
	h.Session.Set(session)

	return c.Send("🌍 Введите ваш часовой пояс в формате IANA (например, Europe/Moscow, America/New_York, Asia/Tokyo):")
}
