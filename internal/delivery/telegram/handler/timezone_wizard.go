package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/texts"
	"github.com/8thgencore/dory-reminder-bot/pkg/timezone"
	tele "gopkg.in/telebot.v4"
)

// HandleTimezoneText обрабатывает ввод пользователем часового пояса.
func (h *Handler) HandleTimezoneText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	tz := strings.TrimSpace(c.Text())

	if !timezone.IsValidTimezone(tz) {
		return c.Send(texts.UnknownTimezone)
	}

	err := h.UserUsecase.SetTimezone(context.Background(), chatID, userID, tz)
	if err != nil {
		slog.Error("Failed to set custom timezone", "user_id", userID, "chat_id", chatID, "timezone", tz, "error", err)
		return c.Send("Ошибка при установке часового пояса")
	}

	h.Session.Delete(chatID, userID)

	slog.Info("Custom timezone set", "user_id", userID, "chat_id", chatID, "timezone", tz)
	// Показываем приветствие и help-меню сразу после установки таймзоны
	return c.Send(texts.HelpMainMenu, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, h.GetMainMenu())
}
