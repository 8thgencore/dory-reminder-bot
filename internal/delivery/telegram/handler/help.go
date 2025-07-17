package handler

import (
	"log/slog"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/texts"
	tele "gopkg.in/telebot.v4"
)

// GetMainMenu возвращает главное меню бота.
func (h *Handler) GetMainMenu() *tele.ReplyMarkup {
	return mainMenu
}

// sendHelp универсальный обработчик help-меню.
func (h *Handler) sendHelp(c tele.Context, text string, menu ...*tele.ReplyMarkup) error {
	opts := &tele.SendOptions{ParseMode: tele.ModeMarkdown}
	if len(menu) > 0 {
		return c.Send(text, opts, menu[0])
	}
	return c.Send(text, opts)
}

// cbHelpAdd показывает справку по добавлению напоминаний.
func (h *Handler) cbHelpAdd(c tele.Context) error {
	slog.Info("User requested help for adding reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.sendHelp(c, texts.HelpAdd, addMenu)
}

// cbHelpList показывает справку по списку напоминаний.
func (h *Handler) cbHelpList(c tele.Context) error {
	slog.Info("User requested help for listing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.onList(c)
}

// cbHelpManage показывает справку по управлению напоминаниями.
func (h *Handler) cbHelpManage(c tele.Context) error {
	slog.Info("User requested help for managing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)
	return h.sendHelp(c, texts.HelpManage)
}
