package commands

import (
	"context"
	"log/slog"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// BasicCommands содержит обработчики базовых команд
type BasicCommands struct {
	UserUsecase usecase.UserUsecase
	GetMainMenu func() *tele.ReplyMarkup
}

// NewBasicCommands создает новый экземпляр BasicCommands
func NewBasicCommands(userUc usecase.UserUsecase, getMainMenu func() *tele.ReplyMarkup) *BasicCommands {
	return &BasicCommands{
		UserUsecase: userUc,
		GetMainMenu: getMainMenu,
	}
}

// HandleStart обрабатывает команду /start
func (bc *BasicCommands) HandleStart(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	username := c.Sender().Username
	firstName := c.Sender().FirstName
	lastName := c.Sender().LastName

	slog.Info("User started bot", "user_id", userID, "chat_id", chatID, "username", username)

	_, err := bc.UserUsecase.GetOrCreateUser(context.Background(), chatID, userID, username, firstName, lastName)
	if err != nil {
		return c.Send(texts.ErrInitUser)
	}

	hasTZ, err := bc.UserUsecase.HasTimezone(context.Background(), chatID, userID)
	if err != nil {
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
	return c.Send(texts.HelpText)
}
