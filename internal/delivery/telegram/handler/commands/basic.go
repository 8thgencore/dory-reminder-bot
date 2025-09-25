package commands

import (
	"context"
	"log/slog"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// BasicCommands содержит обработчики базовых команд
type BasicCommands struct {
	ChatUsecase usecase.ChatUsecase
	GetMainMenu func() *tele.ReplyMarkup
}

// NewBasicCommands создает новый экземпляр BasicCommands
func NewBasicCommands(chatUc usecase.ChatUsecase, getMainMenu func() *tele.ReplyMarkup) *BasicCommands {
	return &BasicCommands{
		ChatUsecase: chatUc,
		GetMainMenu: getMainMenu,
	}
}

// HandleStart обрабатывает команду /start
func (bc *BasicCommands) HandleStart(c tele.Context) error {
	chatID := c.Chat().ID

	// Upsert chat on start
	name := c.Chat().Title
	if name == "" && c.Chat().FirstName != "" {
		name = c.Chat().FirstName
	}
	username := c.Chat().Username
	slog.Info("Start", "chat_id", chatID, "name", name, "username", username)

	_, err := bc.ChatUsecase.GetOrCreateChat(context.Background(), chatID, string(c.Chat().Type), name, username)
	if err != nil {
		slog.Error("Failed to upsert chat", "chat_id", chatID, "error", err)
		return c.Send(texts.ErrInitUser)
	}

	hasTZ, err := bc.ChatUsecase.HasTimezone(context.Background(), chatID)
	if err != nil {
		slog.Error("Failed to check timezone", "chat_id", chatID, "error", err)
		return c.Send(texts.ErrCheckSettings)
	}

	if !hasTZ {
		return c.Send(texts.WelcomeTextNoTZ)
	}

	return c.Send(texts.WelcomeText, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, bc.GetMainMenu())
}

// HandleHelp обрабатывает команду /help
func (bc *BasicCommands) HandleHelp(c tele.Context) error {
	slog.Info("User requested help", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)

	return c.Send(texts.HelpText, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}
