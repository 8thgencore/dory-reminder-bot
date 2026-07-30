// Package telegramapi классифицирует ответы Telegram, которые требуют изменения
// локального состояния чата, а не обычного повторения запроса.
package telegramapi

import (
	"errors"

	tele "gopkg.in/telebot.v4"
)

// MigratedTo извлекает новый ID супергруппы из ответа migrate_to_chat_id.
func MigratedTo(err error) (int64, bool) {
	var groupErr tele.GroupError
	if !errors.As(err, &groupErr) || groupErr.MigratedTo == 0 {
		return 0, false
	}

	return groupErr.MigratedTo, true
}

// IsBotUnavailable отличает окончательное отсутствие бота в чате от временных
// сетевых и серверных ошибок.
func IsBotUnavailable(err error) bool {
	return errors.Is(err, tele.ErrKickedFromGroup) ||
		errors.Is(err, tele.ErrKickedFromSuperGroup) ||
		errors.Is(err, tele.ErrKickedFromChannel) ||
		errors.Is(err, tele.ErrNotChannelMember)
}
