package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
)

// UserUsecase определяет бизнес-логику для работы с пользователями.
type UserUsecase interface {
	GetOrCreateUser(ctx context.Context, chatID, userID int64, username, firstName, lastName string) (*domain.User, error)
	HasTimezone(ctx context.Context, chatID, userID int64) (bool, error)
	SetTimezone(ctx context.Context, chatID, userID int64, timezone string) error
}

type userUsecase struct {
	userRepo repository.UserRepository
}

// NewUserUsecase создает новый UserUsecase.
func NewUserUsecase(userRepo repository.UserRepository) UserUsecase {
	return &userUsecase{userRepo: userRepo}
}

func (u *userUsecase) GetOrCreateUser(
	ctx context.Context,
	chatID, userID int64,
	username, firstName, lastName string,
) (*domain.User, error) {
	slog.Info("[GetOrCreateUser] called", "chatID", chatID, "userID", userID, "username", username)

	// Сначала пытаемся найти пользователя в текущем чате
	user, err := u.userRepo.GetByChatAndUser(ctx, chatID, userID)
	if err != nil {
		slog.Error("[GetOrCreateUser] failed to get user", "chatID", chatID, "userID", userID, "error", err)
		return nil, err
	}

	if user != nil {
		slog.Info("[GetOrCreateUser] user found, updating", "user", user)
		// Пользователь найден в текущем чате, обновляем информацию
		user.Username = username
		user.FirstName = firstName
		user.LastName = lastName
		user.UpdatedAt = time.Now()
		err = u.userRepo.Update(ctx, user)
		if err != nil {
			slog.Error("[GetOrCreateUser] failed to update user", "chatID", chatID, "userID", userID, "error", err)
			return nil, err
		}

		return user, nil
	}

	slog.Info("[GetOrCreateUser] user not found, creating new", "chatID", chatID, "userID", userID)
	// Пользователь не найден в текущем чате, создаем нового
	now := time.Now()
	user = &domain.User{
		ChatID:    chatID,
		ID:        userID,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
		Timezone:  "",
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = u.userRepo.Create(ctx, user)
	if err != nil {
		slog.Error("[GetOrCreateUser] failed to create user", "chatID", chatID, "userID", userID, "error", err)
		return nil, err
	}

	slog.Info("[GetOrCreateUser] user created successfully", "user", user)

	return user, nil
}

func (u *userUsecase) HasTimezone(ctx context.Context, chatID, userID int64) (bool, error) {
	user, err := u.userRepo.GetByChatAndUser(ctx, chatID, userID)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}

	return user.Timezone != "", nil
}

func (u *userUsecase) SetTimezone(ctx context.Context, chatID, userID int64, timezone string) error {
	return u.userRepo.UpdateTimezone(ctx, chatID, userID, timezone)
}
