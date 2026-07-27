package commands

import (
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

// SetCommands устанавливает команды бота в Telegram.
//
// withWebApp добавляет /app: предлагать команду, когда Mini App выключен, значит
// показывать пользователю неработающий пункт меню.
func SetCommands(bot *tele.Bot, log *slog.Logger, withWebApp bool) {
	commands := []tele.Command{
		{Text: "start", Description: "Запустить бота"},
		{Text: "help", Description: "Справка"},
		{Text: "add", Description: "Добавить напоминание"},
		{Text: "list", Description: "Список напоминаний"},
		{Text: "edit", Description: "Редактировать напоминание"},
		{Text: "delete", Description: "Удалить напоминание"},
		{Text: "pause", Description: "Поставить на паузу"},
		{Text: "resume", Description: "Возобновить"},
		{Text: "timezone", Description: "Установить часовой пояс"},
	}

	if withWebApp {
		commands = append(commands, tele.Command{Text: "app", Description: "Открыть приложение"})
	}

	if err := bot.SetCommands(commands); err != nil {
		log.Error("Failed to set bot commands", "error", err)
	}
}
