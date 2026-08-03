package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/commands"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

type appContext struct {
	tele.Context
	chat          *tele.Chat
	sender        *tele.User
	migrationFrom int64
	migrationTo   int64
	memberUpdate  *tele.ChatMemberUpdate
	respondCalls  int
	respondErr    error
}

func (c *appContext) Chat() *tele.Chat   { return c.chat }
func (c *appContext) Sender() *tele.User { return c.sender }
func (c *appContext) Migration() (int64, int64) {
	return c.migrationFrom, c.migrationTo
}
func (c *appContext) ChatMember() *tele.ChatMemberUpdate { return c.memberUpdate }
func (c *appContext) Send(any, ...any) error {
	return nil
}
func (c *appContext) Respond(...*tele.CallbackResponse) error {
	c.respondCalls++
	return c.respondErr
}

type chatUsecaseSpy struct {
	chatID    int64
	chatType  string
	title     string
	username  string
	available bool
	migrated  [2]int64
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

func (s *chatUsecaseSpy) SetAvailable(_ context.Context, _ int64, available bool) error {
	s.available = available
	return nil
}

func (s *chatUsecaseSpy) MigrateChat(_ context.Context, oldChatID, newChatID int64) error {
	s.migrated = [2]int64{oldChatID, newChatID}
	return nil
}

type memberUsecaseSpy struct {
	chatID       int64
	userID       int64
	launchChatID int64
	launchUserID int64
}

func (s *memberUsecaseSpy) Remember(_ context.Context, chatID, userID int64) error {
	s.chatID = chatID
	s.userID = userID

	return nil
}

func (s *memberUsecaseSpy) RememberWebAppLaunch(_ context.Context, chatID, userID int64) error {
	s.launchChatID = chatID
	s.launchUserID = userID

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
	require.True(t, chatUC.available)
	require.Equal(t, groupID, memberUC.chatID)
	require.Equal(t, userID, memberUC.userID)
	require.Equal(t, groupID, memberUC.launchChatID)
	require.Equal(t, userID, memberUC.launchUserID)
}

func TestOnMigrationMovesChatState(t *testing.T) {
	const (
		oldChatID = int64(-5342885594)
		newChatID = int64(-1004314310091)
	)
	chatUC := &chatUsecaseSpy{}
	h := &Handler{ChatUC: chatUC}

	require.NoError(t, h.onMigration(&appContext{
		migrationFrom: oldChatID,
		migrationTo:   newChatID,
	}))
	assert.Equal(t, [2]int64{oldChatID, newChatID}, chatUC.migrated)
}

func TestOnMyChatMemberTracksBotAvailability(t *testing.T) {
	const chatID = int64(-1004314310091)
	chatUC := &chatUsecaseSpy{}
	h := &Handler{ChatUC: chatUC}
	ctx := &appContext{
		chat: &tele.Chat{ID: chatID, Type: tele.ChatSuperGroup, Title: "Команда"},
		memberUpdate: &tele.ChatMemberUpdate{
			NewChatMember: &tele.ChatMember{Role: tele.Kicked},
		},
	}

	require.NoError(t, h.onMyChatMember(ctx))
	assert.False(t, chatUC.available)

	ctx.memberUpdate.NewChatMember.Role = tele.Member
	require.NoError(t, h.onMyChatMember(ctx))
	assert.True(t, chatUC.available)
	assert.Equal(t, chatID, chatUC.chatID)
	assert.Equal(t, "Команда", chatUC.title)
}

func TestCallbackAckRespondsOnceAndStillRunsHandler(t *testing.T) {
	expectedErr := errors.New("callback already expired")
	ctx := &appContext{respondErr: expectedErr}
	called := false

	err := (&Handler{}).withCallbackAck(func(tele.Context) error {
		called = true
		return nil
	})(ctx)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 1, ctx.respondCalls)
}
