package handler

import (
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

func (h *Handler) GetMainMenu() *tele.ReplyMarkup {
	return mainMenu
}

func (h *Handler) cbHelpAdd(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested help for adding reminders", "user_id", userID, "chat_id", chatID)

	helpText := "➕ *Добавление напоминаний*\n\n" +
		"Используйте мастер для создания напоминания. Выберите тип напоминания ниже и следуйте инструкциям.\n\n" +
		"*Доступные типы:*\n" +
		"• **Сегодня** - напоминание сегодня в указанное время\n" +
		"• **Завтра** - напоминание завтра в указанное время\n" +
		"• **Ежедневно** - каждый день в указанное время\n" +
		"• **Раз в неделю** - в выбранный день недели\n" +
		"• **Раз в месяц** - в выбранное число месяца\n" +
		"• **Раз в несколько дней** - с указанным интервалом\n" +
		"• **Раз в год** - в указанную дату\n" +
		"• **Выбрать дату** - разовое напоминание в конкретную дату\n" +
		"• **Несколько раз в день** - несколько напоминаний в день\n\n" +
		"*Примеры:*\n" +
		"• Сегодня → 15:00 → Позвонить маме\n" +
		"• Ежедневно → 09:00 → Принять таблетку"

	return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, addMenu)
}

func (h *Handler) cbHelpList(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested help for listing reminders", "user_id", userID, "chat_id", chatID)

	// Показываем сразу список напоминаний
	return h.onList(c)
}

func (h *Handler) cbHelpManage(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	slog.Info("User requested help for managing reminders", "user_id", userID, "chat_id", chatID)

	helpText := "⚙️ *Управление напоминаниями*\n\n" +
		"*Команды:*\n" +
		"• `/delete <номер>` - удалить напоминание\n" +
		"• `/pause <номер>` - поставить на паузу\n" +
		"• `/resume <номер>` - возобновить напоминание\n\n" +
		"*Примеры:*\n" +
		"• `/delete 2` - удалить напоминание №2\n" +
		"• `/pause 1` - поставить на паузу напоминание №1\n" +
		"• `/resume 1` - возобновить напоминание №1\n\n" +
		"*Примечания:*\n" +
		"• Номера напоминаний можно посмотреть командой `/list`\n" +
		"• На паузе напоминания не срабатывают, но сохраняются\n" +
		"• Удалённые напоминания восстановить нельзя"

	return c.Send(helpText, &tele.SendOptions{
		ParseMode: tele.ModeMarkdown,
	})
}
