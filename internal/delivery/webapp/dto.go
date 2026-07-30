package webapp

import (
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// Строковые обозначения повторов.
//
// Наружу отдаются строки, а не числовые значения domain.RepeatType: иначе перестановка
// констант в Go молча поменяла бы смысл сохранённых на клиенте значений.
const (
	repeatNone      = "none"
	repeatDaily     = "daily"
	repeatWeekly    = "weekly"
	repeatMonthly   = "monthly"
	repeatEveryDays = "every_n_days"
	repeatYearly    = "yearly"
)

// Типы чатов Telegram, используемые в API.
const (
	chatTypePrivate = "private"
	chatTypeGroup   = "group"
)

var repeatToAPI = map[domain.RepeatType]string{
	domain.RepeatNone:       repeatNone,
	domain.RepeatEveryDay:   repeatDaily,
	domain.RepeatEveryWeek:  repeatWeekly,
	domain.RepeatEveryMonth: repeatMonthly,
	domain.RepeatEveryNDays: repeatEveryDays,
	domain.RepeatEveryYear:  repeatYearly,
}

var apiToRepeat = map[string]domain.RepeatType{
	repeatNone:      domain.RepeatNone,
	repeatDaily:     domain.RepeatEveryDay,
	repeatWeekly:    domain.RepeatEveryWeek,
	repeatMonthly:   domain.RepeatEveryMonth,
	repeatEveryDays: domain.RepeatEveryNDays,
	repeatYearly:    domain.RepeatEveryYear,
}

// userDTO описывает пользователя Mini App.
type userDTO struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// chatDTO описывает чат, доступный пользователю.
type chatDTO struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	IsPublic bool   `json:"is_group"`
}

// meResponse — ответ GET /api/v1/me.
type meResponse struct {
	User         userDTO   `json:"user"`
	Chats        []chatDTO `json:"chats"`
	LaunchChatID int64     `json:"launch_chat_id,omitempty"`
}

// reminderDTO описывает напоминание в API.
//
// NextTime всегда в UTC (RFC 3339); локальное представление собирает клиент,
// используя Timezone чата.
type reminderDTO struct {
	ID          int64     `json:"id"`
	ChatID      int64     `json:"chat_id"`
	Text        string    `json:"text"`
	NextTime    time.Time `json:"next_time"`
	Repeat      string    `json:"repeat"`
	RepeatDays  []int     `json:"repeat_days"`
	RepeatEvery int       `json:"repeat_every"`
	Paused      bool      `json:"paused"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// reminderListResponse — ответ со списком напоминаний.
type reminderListResponse struct {
	Timezone  string        `json:"timezone"`
	Reminders []reminderDTO `json:"reminders"`
}

// reminderRequest — тело запроса на создание или изменение напоминания.
//
// Указатели позволяют отличить «поле не передано» от «передано нулевое значение»,
// что нужно для PATCH: пропущенные поля сохраняют текущее значение.
type reminderRequest struct {
	Text        *string `json:"text"`
	Time        *string `json:"time"`         // ЧЧ:ММ в часовом поясе чата
	Date        *string `json:"date"`         // ДД.ММ.ГГГГ, для разовых и «каждые N дней»
	Repeat      *string `json:"repeat"`       // строковое обозначение повтора
	RepeatDays  *[]int  `json:"repeat_days"`  // дни недели (0..6) или число месяца (1..31)
	RepeatEvery *int    `json:"repeat_every"` // интервал для every_n_days
	Paused      *bool   `json:"paused"`
}

// timezoneRequest — тело запроса на смену часового пояса.
type timezoneRequest struct {
	Timezone string `json:"timezone"`
}

func toReminderDTO(r *domain.Reminder) reminderDTO {
	days := r.RepeatDays
	if days == nil {
		// Клиенту удобнее пустой массив, чем null.
		days = []int{}
	}

	return reminderDTO{
		ID:          r.ID,
		ChatID:      r.ChatID,
		Text:        r.Text,
		NextTime:    r.NextTime.UTC(),
		Repeat:      repeatToAPI[r.Repeat],
		RepeatDays:  days,
		RepeatEvery: r.RepeatEvery,
		Paused:      r.Paused,
		CreatedAt:   r.CreatedAt.UTC(),
		UpdatedAt:   r.UpdatedAt.UTC(),
	}
}

func toChatDTO(c *domain.Chat) chatDTO {
	return chatDTO{
		ID:       c.ID,
		Type:     c.Type,
		Title:    c.Name,
		Username: c.Username,
		Timezone: c.Timezone,
		IsPublic: c.Type != chatTypePrivate,
	}
}

// parseRepeat переводит строковое обозначение повтора в доменное значение.
func parseRepeat(s string) (domain.RepeatType, error) {
	r, ok := apiToRepeat[s]
	if !ok {
		return 0, fmt.Errorf("unknown repeat type %q", s)
	}

	return r, nil
}
