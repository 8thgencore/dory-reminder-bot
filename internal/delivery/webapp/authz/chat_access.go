// Package authz решает, может ли пользователь Mini App работать с конкретным чатом.
package authz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
)

// ErrForbidden возвращается, когда пользователь не имеет доступа к чату.
var ErrForbidden = errors.New("access to chat is forbidden")

// defaultCacheTTL — срок жизни закэшированного запрета.
//
// Разрешения намеренно не кэшируются: выход или удаление из группы должны отзывать
// доступ уже на следующем запросе.
const defaultCacheTTL = 5 * time.Minute

// memberChecker — часть API бота, нужная для проверки членства.
type memberChecker interface {
	ChatMemberOf(chat, user tele.Recipient) (*tele.ChatMember, error)
}

type cacheKey struct {
	userID int64
	chatID int64
}

// Access проверяет права пользователя на чат через Telegram Bot API.
type Access struct {
	bot memberChecker
	ttl time.Duration
	now func() time.Time

	mu     sync.RWMutex
	denied map[cacheKey]time.Time
}

// New создает проверку доступа поверх клиента бота.
func New(bot memberChecker) *Access {
	return &Access{
		bot:    bot,
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
func (a *Access) Check(ctx context.Context, userID, chatID int64) error {
	if userID == 0 || chatID == 0 {
		return ErrForbidden
	}

	// В личном чате идентификатор чата совпадает с идентификатором пользователя,
	// так что обращение к Bot API не нужно.
	if chatID == userID {
		return nil
	}

	key := cacheKey{userID: userID, chatID: chatID}
	if a.deniedCached(key) {
		return ErrForbidden
	}

	allowed, err := a.fetch(ctx, userID, chatID)
	if err != nil {
		// Ответ Bot API не получен — трактуем как запрет и ничего не кэшируем,
		// иначе сетевой сбой открыл бы доступ к чужому чату.
		slog.Warn("Failed to check chat membership", "user_id", userID, "chat_id", chatID, "error", err)

		return fmt.Errorf("%w: membership check failed", ErrForbidden)
	}

	if !allowed {
		// Запрет можно безопасно кэшировать: устаревшее решение лишь временно
		// задержит доступ недавно вступившему пользователю. Разрешения не кэшируем,
		// чтобы выход или удаление из группы отзывали доступ на следующем запросе.
		a.storeDenial(key)
		return ErrForbidden
	}

	return nil
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
