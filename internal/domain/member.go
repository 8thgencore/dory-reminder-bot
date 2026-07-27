package domain

import "time"

// ChatMember связывает пользователя Telegram с чатом, в котором его видел бот.
//
// Запись носит справочный характер: она нужна, чтобы показать пользователю список его чатов
// в Mini App. Право на конкретный чат всегда перепроверяется через Bot API.
type ChatMember struct {
	ChatID   int64
	UserID   int64
	LastSeen time.Time
}
