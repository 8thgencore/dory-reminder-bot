package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/pkg/timezone"
)

// ErrInvalidTimezone возвращается при попытке установить неизвестную IANA-зону.
var ErrInvalidTimezone = errors.New("invalid timezone")

// ChatUsecase описывает бизнес-логику работы с чатами
type ChatUsecase interface {
	GetOrCreateChat(ctx context.Context, chatID int64, chatType, title, username string) (*domain.Chat, error)
	HasTimezone(ctx context.Context, chatID int64) (bool, error)
	SetTimezone(ctx context.Context, chatID int64, timezone string) error
	ResolveChatID(ctx context.Context, chatID int64) (int64, error)
	MigrateChat(ctx context.Context, oldChatID, newChatID int64) error
	SetAvailable(ctx context.Context, chatID int64, available bool) error
	IsAvailable(ctx context.Context, chatID int64) (bool, error)
	// Get возвращает чат или repository.ErrChatNotFound.
	Get(ctx context.Context, chatID int64) (*domain.Chat, error)
	// Location возвращает часовой пояс чата, откатываясь к UTC, если он не задан или не читается.
	Location(ctx context.Context, chatID int64) *time.Location
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
	ch, err := u.chatRepo.GetByID(ctx, chatID)
	if err != nil && !errors.Is(err, repository.ErrChatNotFound) {
		return nil, err
	}

	now := time.Now()
	if ch == nil {
		ch = &domain.Chat{
			ID:        chatID,
			Type:      chatType,
			Name:      name,
			Username:  username,
			Available: true,
			CreatedAt: now,
			UpdatedAt: now,
		}
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
	if errors.Is(err, repository.ErrChatNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return ch.Timezone != "", nil
}

func (u *chatUsecase) SetTimezone(ctx context.Context, chatID int64, tz string) error {
	if !timezone.IsValidTimezone(tz) {
		return ErrInvalidTimezone
	}

	return u.chatRepo.UpdateTimezone(ctx, chatID, tz)
}

func (u *chatUsecase) Get(ctx context.Context, chatID int64) (*domain.Chat, error) {
	return u.chatRepo.GetByID(ctx, chatID)
}

func (u *chatUsecase) ResolveChatID(ctx context.Context, chatID int64) (int64, error) {
	return u.chatRepo.ResolveID(ctx, chatID)
}

func (u *chatUsecase) MigrateChat(ctx context.Context, oldChatID, newChatID int64) error {
	return u.chatRepo.Migrate(ctx, oldChatID, newChatID)
}

func (u *chatUsecase) SetAvailable(ctx context.Context, chatID int64, available bool) error {
	return u.chatRepo.SetAvailable(ctx, chatID, available)
}

func (u *chatUsecase) IsAvailable(ctx context.Context, chatID int64) (bool, error) {
	ch, err := u.chatRepo.GetByID(ctx, chatID)
	if errors.Is(err, repository.ErrChatNotFound) {
		// Старые базы могли содержать напоминание без строки chats. Пока Telegram
		// явно не сообщил обратное, такой чат считаем доступным.
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return ch.Available, nil
}

func (u *chatUsecase) Location(ctx context.Context, chatID int64) *time.Location {
	ch, err := u.chatRepo.GetByID(ctx, chatID)
	if err != nil || ch == nil || ch.Timezone == "" {
		return time.UTC
	}

	loc, err := time.LoadLocation(ch.Timezone)
	if err != nil {
		// Обычно означает отсутствие tzdata в образе — молчаливый откат в UTC сдвинул бы
		// все напоминания чата, поэтому логируем явно.
		slog.Error("Failed to load chat timezone, falling back to UTC",
			"chatID", chatID, "timezone", ch.Timezone, "error", err)

		return time.UTC
	}

	return loc
}
