package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/pkg/timezone"
	tele "gopkg.in/telebot.v4"
)

// Удаляю все кнопки и меню timezone

func (h *Handler) HandleTimezoneText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	tz := strings.TrimSpace(c.Text())

	if !timezone.IsValidTimezone(tz) {
		return c.Send("❌ Неизвестный или невалидный часовой пояс. Введите в формате IANA, например: Europe/Moscow, America/New_York, Asia/Tokyo. Список поддерживаемых: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones")
	}

	err := h.UserUsecase.SetTimezone(context.Background(), chatID, userID, tz)
	if err != nil {
		slog.Error("Failed to set custom timezone", "user_id", userID, "chat_id", chatID, "timezone", tz, "error", err)
		return c.Send("Ошибка при установке часового пояса")
	}

	h.Session.Delete(chatID, userID)

	slog.Info("Custom timezone set", "user_id", userID, "chat_id", chatID, "timezone", tz)
	// Показываем приветствие и help-меню сразу после установки таймзоны
	helpText := "🤖 *Dory Reminder Bot*\n\nПривет! Я бот для создания и управления напоминаниями.\n\nВыберите раздел справки:"
	return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, h.GetMainMenu())
}
