package domain

import "time"

// User описывает пользователя Telegram-бота.
type User struct {
	ID        int64
	ChatID    int64
	Username  string
	FirstName string
	LastName  string
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
