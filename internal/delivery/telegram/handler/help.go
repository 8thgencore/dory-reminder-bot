package handler

import (
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

// Главное меню
func (h *Handler) GetMainMenu() *tele.ReplyMarkup {
	return mainMenu
}

// Универсальный обработчик help-меню
func (h *Handler) sendHelp(c tele.Context, text string, menu ...*tele.ReplyMarkup) error {
	opts := &tele.SendOptions{ParseMode: tele.ModeMarkdown}
	if len(menu) > 0 {
		return c.Send(text, opts, menu[0])
	}
	return c.Send(text, opts)
}

func (h *Handler) cbHelpAdd(c tele.Context) error {
	slog.Info("User requested help for adding reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.sendHelp(c, "➕ *Добавление напоминаний*\n\nИспользуйте мастер для создания напоминания. Выберите тип напоминания ниже и следуйте инструкциям.\n\n*Доступные типы:*\n• **Сегодня** - напоминание сегодня в указанное время\n• **Завтра** - напоминание завтра в указанное время\n• **Ежедневно** - каждый день в указанное время\n• **Раз в неделю** - в выбранный день недели\n• **Раз в месяц** - в выбранное число месяца\n• **Раз в несколько дней** - с указанным интервалом\n• **Раз в год** - в указанную дату\n• **Выбрать дату** - разовое напоминание в конкретную дату\n• **Несколько раз в день** - несколько напоминаний в день\n\n*Примеры:*\n• Сегодня → 15:00 → Позвонить маме\n• Ежедневно → 09:00 → Принять таблетку", addMenu)
}

func (h *Handler) cbHelpList(c tele.Context) error {
	slog.Info("User requested help for listing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.onList(c)
}

func (h *Handler) cbHelpManage(c tele.Context) error {
	slog.Info("User requested help for managing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.sendHelp(c, "⚙️ *Управление напоминаниями*\n\n*Команды:*\n• `/delete <номер>` - удалить напоминание\n• `/pause <номер>` - поставить на паузу\n• `/resume <номер>` - возобновить напоминание\n\n*Примеры:*\n• `/delete 2` - удалить напоминание №2\n• `/pause 1` - поставить на паузу напоминание №1\n• `/resume 1` - возобновить напоминание №1\n\n*Примечания:*\n• Номера напоминаний можно посмотреть командой `/list`\n• На паузе напоминания не срабатывают, но сохраняются\n• Удалённые напоминания восстановить нельзя")
}
