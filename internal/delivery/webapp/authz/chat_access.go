// Package authz решает, может ли пользователь Mini App работать с конкретным чатом.
package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/infrastructure/telegramapi"
	tele "gopkg.in/telebot.v4"
)

// ErrForbidden возвращается, когда пользователь не имеет доступа к чату.
var ErrForbidden = errors.New("access to chat is forbidden")

// ErrBotUnavailable уточняет, что доступ закрыт для всех пользователей, потому
// что самого бота больше нет в чате.
var ErrBotUnavailable = errors.New("bot is unavailable in chat")

// defaultCacheTTL — срок жизни закэшированного запрета.
//
// Разрешения намеренно не кэшируются: выход или удаление из группы должны отзывать
// доступ уже на следующем запросе.
const defaultCacheTTL = 5 * time.Minute

// memberChecker — часть API бота, нужная для проверки членства.
type memberChecker interface {
	ChatMemberOf(chat, user tele.Recipient) (*tele.ChatMember, error)
}

type chatLifecycle interface {
	ResolveChatID(ctx context.Context, chatID int64) (int64, error)
	MigrateChat(ctx context.Context, oldChatID, newChatID int64) error
	SetAvailable(ctx context.Context, chatID int64, available bool) error
	IsAvailable(ctx context.Context, chatID int64) (bool, error)
}

type identityLifecycle struct{}

func (identityLifecycle) ResolveChatID(_ context.Context, chatID int64) (int64, error) {
	return chatID, nil
}

func (identityLifecycle) MigrateChat(context.Context, int64, int64) error { return nil }
func (identityLifecycle) SetAvailable(context.Context, int64, bool) error { return nil }
func (identityLifecycle) IsAvailable(context.Context, int64) (bool, error) {
	return true, nil
}

type cacheKey struct {
	userID int64
	chatID int64
}

// Access проверяет права пользователя на чат через Telegram Bot API.
type Access struct {
	bot   memberChecker
	chats chatLifecycle
	ttl   time.Duration
	now   func() time.Time

	mu     sync.RWMutex
	denied map[cacheKey]time.Time
}

// New создает проверку доступа поверх клиента бота.
func New(bot memberChecker, lifecycle ...chatLifecycle) *Access {
	chats := chatLifecycle(identityLifecycle{})
	if len(lifecycle) > 0 && lifecycle[0] != nil {
		chats = lifecycle[0]
	}

	return &Access{
		bot:    bot,
		chats:  chats,
		ttl:    defaultCacheTTL,
		now:    time.Now,
		denied: make(map[cacheKey]time.Time),
	}
}

// Check проверяет, что пользователь вправе управлять напоминаниями чата.
//
// chatID приходит из пути HTTP-запроса, и доверять ему нельзя: initData подтверждает
// личность пользователя, но ничего не говорит о том, к какому чату он обращается.
// Единственный источник истины здесь — Bot API.
func (a *Access) Resolve(ctx context.Context, userID, chatID int64) (int64, error) {
	if userID == 0 || chatID == 0 {
		return 0, ErrForbidden
	}

	// В личном чате идентификатор чата совпадает с идентификатором пользователя,
	// так что обращение к Bot API не нужно.
	if chatID == userID {
		return chatID, nil
	}

	resolvedID, err := a.chats.ResolveChatID(ctx, chatID)
	if err != nil {
		return 0, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		available, err := a.chats.IsAvailable(ctx, resolvedID)
		if err != nil {
			return 0, err
		}
		if !available {
			return 0, fmt.Errorf("%w: %w", ErrForbidden, ErrBotUnavailable)
		}

		key := cacheKey{userID: userID, chatID: resolvedID}
		if a.deniedCached(key) {
			return 0, ErrForbidden
		}

		allowed, err := a.fetch(ctx, userID, resolvedID)
		if err != nil {
			if migratedTo, migrated := telegramapi.MigratedTo(err); migrated && attempt == 0 {
				if err := a.chats.MigrateChat(ctx, resolvedID, migratedTo); err != nil {
					return 0, err
				}

				slog.Info(
					"Migrated Telegram chat",
					"old_chat_id", resolvedID,
					"new_chat_id", migratedTo,
				)
				resolvedID = migratedTo
				continue
			}

			if telegramapi.IsBotUnavailable(err) {
				if stateErr := a.chats.SetAvailable(ctx, resolvedID, false); stateErr != nil {
					return 0, fmt.Errorf("mark unavailable chat: %w", stateErr)
				}
				slog.Warn("Telegram bot is unavailable in chat", "chat_id", resolvedID, "error", err)

				return 0, fmt.Errorf("%w: %w", ErrForbidden, ErrBotUnavailable)
			}

			// Ответ Bot API не получен — трактуем как запрет и ничего не кэшируем,
			// иначе сетевой сбой открыл бы доступ к чужому чату.
			slog.Warn(
				"Failed to check chat membership",
				"user_id", userID,
				"chat_id", resolvedID,
				"error", err,
			)

			return 0, fmt.Errorf("%w: membership check failed", ErrForbidden)
		}

		if !allowed {
			// Запрет можно безопасно кэшировать: устаревшее решение лишь временно
			// задержит доступ недавно вступившему пользователю. Разрешения не кэшируем,
			// чтобы выход или удаление из группы отзывали доступ на следующем запросе.
			a.storeDenial(key)
			return 0, ErrForbidden
		}

		return resolvedID, nil
	}

	return 0, fmt.Errorf("%w: repeated chat migration", ErrForbidden)
}

// Check сохраняет прежний интерфейс для мест, которым не нужен актуальный ID.
func (a *Access) Check(ctx context.Context, userID, chatID int64) error {
	_, err := a.Resolve(ctx, userID, chatID)

	return err
}

// Forget сбрасывает закэшированное решение — например, после выхода пользователя из чата.
func (a *Access) Forget(userID, chatID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.denied, cacheKey{userID: userID, chatID: chatID})
}

func (a *Access) deniedCached(key cacheKey) bool {
	a.mu.RLock()
	expiresAt, found := a.denied[key]
	a.mu.RUnlock()

	return found && !a.now().After(expiresAt)
}

func (a *Access) storeDenial(key cacheKey) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Раз в записи чистим протухшее: кэш ограничен числом пар «пользователь — чат»,
	// но без уборки он рос бы вместе с каждым новым чатом навсегда.
	if len(a.denied) > 0 && len(a.denied)%256 == 0 {
		a.evictExpiredLocked()
	}

	a.denied[key] = a.now().Add(a.ttl)
}

func (a *Access) evictExpiredLocked() {
	now := a.now()
	for key, expiresAt := range a.denied {
		if now.After(expiresAt) {
			delete(a.denied, key)
		}
	}
}

func (a *Access) fetch(ctx context.Context, userID, chatID int64) (bool, error) {
	// telebot не принимает context, поэтому вызов выполняется в горутине, а отмена
	// контекста освобождает обработчик, не дожидаясь ответа Telegram.
	type result struct {
		member *tele.ChatMember
		err    error
	}
	done := make(chan result, 1)

	go func() {
		member, err := a.bot.ChatMemberOf(&tele.Chat{ID: chatID}, &tele.User{ID: userID})
		done <- result{member: member, err: err}
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case res := <-done:
		if res.err != nil {
			return false, res.err
		}

		return isActiveMember(res.member), nil
	}
}

// isActiveMember сообщает, состоит ли пользователь в чате сейчас.
func isActiveMember(member *tele.ChatMember) bool {
	if member == nil {
		return false
	}

	switch member.Role {
	case tele.Creator, tele.Administrator, tele.Member, tele.Restricted:
		return true
	case tele.Left, tele.Kicked:
		return false
	default:
		return false
	}
}
