package handler

import (
	"context"
	"testing"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/commands"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

type appContext struct {
	tele.Context
	chat   *tele.Chat
	sender *tele.User
}

func (c *appContext) Chat() *tele.Chat   { return c.chat }
func (c *appContext) Sender() *tele.User { return c.sender }
func (c *appContext) Send(any, ...any) error {
	return nil
}

type chatUsecaseSpy struct {
	usecase.ChatUsecase
	chatID   int64
	chatType string
	title    string
	username string
}

func (s *chatUsecaseSpy) GetOrCreateChat(
	_ context.Context,
	chatID int64,
	chatType, title, username string,
) (*domain.Chat, error) {
	s.chatID = chatID
	s.chatType = chatType
	s.title = title
	s.username = username

	return &domain.Chat{ID: chatID}, nil
}

type memberUsecaseSpy struct {
	usecase.MemberUsecase
	chatID int64
	userID int64
}

func (s *memberUsecaseSpy) Remember(_ context.Context, chatID, userID int64) error {
	s.chatID = chatID
	s.userID = userID

	return nil
}

func TestOnAppRemembersGroupBeforeSendingLink(t *testing.T) {
	const (
		groupID = int64(-1001234567890)
		userID  = int64(42)
	)

	chatUC := &chatUsecaseSpy{}
	memberUC := &memberUsecaseSpy{}
	h := &Handler{
		ChatUC:         chatUC,
		MemberUC:       memberUC,
		WebAppCommands: commands.NewWebAppCommands(config.WebAppConfig{}, "test_bot"),
	}
	ctx := &appContext{
		chat:   &tele.Chat{ID: groupID, Type: tele.ChatSuperGroup, Title: "Команда"},
		sender: &tele.User{ID: userID},
	}

	require.NoError(t, h.onApp(ctx))
	require.Equal(t, groupID, chatUC.chatID)
	require.Equal(t, string(tele.ChatSuperGroup), chatUC.chatType)
	require.Equal(t, "Команда", chatUC.title)
	require.Equal(t, groupID, memberUC.chatID)
	require.Equal(t, userID, memberUC.userID)
}
