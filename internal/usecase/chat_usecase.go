package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
)

// ChatUsecase описывает бизнес-логику работы с чатами
type ChatUsecase interface {
	GetOrCreateChat(ctx context.Context, chatID int64, chatType, title, username string) (*domain.Chat, error)
	HasTimezone(ctx context.Context, chatID int64) (bool, error)
	SetTimezone(ctx context.Context, chatID int64, timezone string) error
	Get(ctx context.Context, chatID int64) (*domain.Chat, error)
}

type chatUsecase struct {
	chatRepo repository.ChatRepository
}

// NewChatUsecase создает новый ChatUsecase.
func NewChatUsecase(chatRepo repository.ChatRepository) ChatUsecase {
	return &chatUsecase{chatRepo: chatRepo}
}

func (u *chatUsecase) GetOrCreateChat(
	ctx context.Context,
	chatID int64,
	chatType, name, username string,
) (*domain.Chat, error) {
	slog.Info("[ChatUC.GetOrCreateChat] called", "chatID", chatID)
	ch, err := u.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if ch == nil {
		ch = &domain.Chat{ID: chatID, Type: chatType, Name: name, Username: username, CreatedAt: now, UpdatedAt: now}
	} else {
		ch.Type = chatType
		ch.Name = name
		ch.Username = username
		ch.UpdatedAt = now
	}
	if err := u.chatRepo.Upsert(ctx, ch); err != nil {
		return nil, err
	}

	return ch, nil
}

func (u *chatUsecase) HasTimezone(ctx context.Context, chatID int64) (bool, error) {
	ch, err := u.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return false, err
	}
	if ch == nil {
		return false, nil
	}

	return ch.Timezone != "", nil
}

func (u *chatUsecase) SetTimezone(ctx context.Context, chatID int64, timezone string) error {
	return u.chatRepo.UpdateTimezone(ctx, chatID, timezone)
}

func (u *chatUsecase) Get(ctx context.Context, chatID int64) (*domain.Chat, error) {
	return u.chatRepo.GetByID(ctx, chatID)
}
