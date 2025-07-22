package domain

import "time"

// RepeatType определяет тип повтора напоминания.
type RepeatType int

// Возможные типы повтора напоминания.
const (
	RepeatNone       RepeatType = iota // без повтора
	RepeatEveryDay                     // ежедневно
	RepeatEveryWeek                    // еженедельно
	RepeatEveryMonth                   // ежемесячно
	RepeatEveryNDays                   // каждые N дней
	RepeatEveryYear                    // ежегодно
)

// Reminder описывает напоминание пользователя.
type Reminder struct {
	ID          int64
	ChatID      int64
	UserID      int64
	Text        string
	NextTime    time.Time
	Repeat      RepeatType
	RepeatDays  []int // для дней недели/месяца
	RepeatEvery int   // для N дней
	Paused      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
