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
	ChatUsecase    usecase.ChatUsecase
	SessionManager *session.Manager
	GetMainMenu    func() *tele.ReplyMarkup
	BotName        string
}

// NewTimezoneWizard создает новый экземпляр мастера настройки часового пояса
func NewTimezoneWizard(
	chatUc usecase.ChatUsecase,
	sessionMgr *session.Manager,
	getMainMenu func() *tele.ReplyMarkup,
	botName string,
) *TimezoneWizard {
	return &TimezoneWizard{
		ChatUsecase:    chatUc,
		SessionManager: sessionMgr,
		GetMainMenu:    getMainMenu,
		BotName:        botName,
	}
}

func (tw *TimezoneWizard) withGroupHint(c tele.Context, msg string) string {
	if c.Chat().Type != "private" && msg != texts.PromptUnknown {
		return msg + "\n\nЧтобы бот увидел ваш ответ, добавьте в конце @" + tw.BotName
	}
	return msg
}

// OnTimezone обрабатывает команду /timezone
func (tw *TimezoneWizard) OnTimezone(c tele.Context) error {
	tw.SessionManager.Set(&session.AddReminderSession{
		UserID: c.Sender().ID,
		ChatID: c.Chat().ID,
		Step:   session.StepTimezone,
	})

	return c.Send(tw.withGroupHint(c, texts.SetTimezonePrompt))
}

// HandleTimezoneText обрабатывает ввод пользователем часового пояса
func (tw *TimezoneWizard) HandleTimezoneText(c tele.Context, botName string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID

	// Убираем упоминание бота из текста, если оно есть
	tz := strings.TrimSpace(c.Text())
	tz = strings.ReplaceAll(tz, "@"+botName, "")
	tz = strings.TrimSpace(tz)

	if !timezone.IsValidTimezone(tz) {
		return c.Send(texts.UnknownTimezone)
	}

	// В единой модели работаем с таймзоной чата
	hadTimezone, err := tw.ChatUsecase.HasTimezone(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to check timezone", "user_id", userID, "chat_id", chatID, "error", err)
		hadTimezone = false // считаем что таймзоны не было
	}

	err = tw.ChatUsecase.SetTimezone(context.Background(), chatID, tz)
	if err != nil {
		slog.Error("Failed to set custom timezone", "user_id", userID, "chat_id", chatID, "timezone", tz, "error", err)
		return c.Send("Ошибка при установке часового пояса")
	}

	tw.SessionManager.Delete(chatID, userID)

	slog.Info("Custom timezone set", "chat_id", chatID, "timezone", tz)

	// Показываем сообщение об успешной установке
	successMsg := "✅ Часовой пояс успешно установлен: " + tz

	// Приветственное сообщение показываем только при первой установке
	if !hadTimezone {
		return c.Send(successMsg+"\n\n"+texts.HelpMainMenu, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, tw.GetMainMenu())
	}

	return c.Send(successMsg)
}
