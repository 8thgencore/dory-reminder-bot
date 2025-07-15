package domain

import "time"

type RepeatType int

const (
	RepeatNone RepeatType = iota
	RepeatEveryDay
	RepeatEveryWeek
	RepeatEveryMonth
	RepeatEveryNDays
)

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
	Timezone    string
}
