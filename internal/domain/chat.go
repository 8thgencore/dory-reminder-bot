package domain

import "time"

// Chat описывает чат Telegram (private/group/supergroup/channel)
type Chat struct {
	ID        int64
	Type      string
	Name      string
	Username  string
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
