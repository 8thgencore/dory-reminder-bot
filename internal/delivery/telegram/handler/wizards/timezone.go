package wizards

import (
	"context"
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/8thgencore/dory-reminder-bot/pkg/timezone"
	tele "gopkg.in/telebot.v4"
)

// TimezoneWizard обрабатывает мастер настройки часового пояса
type TimezoneWizard struct {
	UserUsecase    usecase.UserUsecase
	SessionManager *session.SessionManager
	GetMainMenu    func() *tele.ReplyMarkup
}

// NewTimezoneWizard создает новый экземпляр мастера настройки часового пояса
func NewTimezoneWizard(userUc usecase.UserUsecase, sessionMgr *session.SessionManager, getMainMenu func() *tele.ReplyMarkup) *TimezoneWizard {
	return &TimezoneWizard{
		UserUsecase:    userUc,
		SessionManager: sessionMgr,
		GetMainMenu:    getMainMenu,
	}
}

// OnTimezone обрабатывает команду /timezone
func (tw *TimezoneWizard) OnTimezone(c tele.Context) error {
	tw.SessionManager.Set(&session.AddReminderSession{
		UserID: c.Sender().ID,
		ChatID: c.Chat().ID,
		Step:   session.StepTimezone,
	})
	return c.Send("🌍 Введите ваш часовой пояс в формате IANA (например, Europe/Moscow, America/New_York, Asia/Tokyo):")
}

// HandleTimezoneText обрабатывает ввод пользователем часового пояса
func (tw *TimezoneWizard) HandleTimezoneText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	tz := strings.TrimSpace(c.Text())

	if !timezone.IsValidTimezone(tz) {
		return c.Send(texts.UnknownTimezone)
	}

	err := tw.UserUsecase.SetTimezone(context.Background(), chatID, userID, tz)
	if err != nil {
		slog.Error("Failed to set custom timezone", "user_id", userID, "chat_id", chatID, "timezone", tz, "error", err)
		return c.Send("Ошибка при установке часового пояса")
	}

	tw.SessionManager.Delete(chatID, userID)

	slog.Info("Custom timezone set", "user_id", userID, "chat_id", chatID, "timezone", tz)
	// Показываем приветствие и help-меню сразу после установки таймзоны
	return c.Send(texts.HelpMainMenu, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, tw.GetMainMenu())
}
