package usecase

import (
	"context"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
)

// MemberUsecase описывает бизнес-логику связей "пользователь — чат".
//
// Эти связи нужны только для того, чтобы показать пользователю список его чатов в Mini App.
// Право на конкретный чат проверяется отдельно, через Bot API.
type MemberUsecase interface {
	// Remember фиксирует, что пользователь виден боту в этом чате.
	Remember(ctx context.Context, chatID, userID int64) error
	// RememberWebAppLaunch фиксирует группу, из которой пользователь вызвал /app.
	RememberWebAppLaunch(ctx context.Context, chatID, userID int64) error
	// ListChats возвращает чаты, в которых бот видел пользователя.
	ListChats(ctx context.Context, userID int64) ([]*domain.Chat, error)
	// RecentWebAppLaunch возвращает последнюю группу запуска не старше since.
	RecentWebAppLaunch(ctx context.Context, userID int64, since time.Time) (*domain.Chat, error)
}

type memberUsecase struct {
	repo repository.MemberRepository
}

// NewMemberUsecase создает новый MemberUsecase.
func NewMemberUsecase(repo repository.MemberRepository) MemberUsecase {
	return &memberUsecase{repo: repo}
}

func (u *memberUsecase) Remember(ctx context.Context, chatID, userID int64) error {
	return u.repo.Upsert(ctx, chatID, userID)
}

func (u *memberUsecase) RememberWebAppLaunch(ctx context.Context, chatID, userID int64) error {
	return u.repo.RememberWebAppLaunch(ctx, chatID, userID)
}

func (u *memberUsecase) ListChats(ctx context.Context, userID int64) ([]*domain.Chat, error) {
	return u.repo.ListChatsByUser(ctx, userID)
}

func (u *memberUsecase) RecentWebAppLaunch(
	ctx context.Context,
	userID int64,
	since time.Time,
) (*domain.Chat, error) {
	return u.repo.RecentWebAppLaunch(ctx, userID, since)
}
