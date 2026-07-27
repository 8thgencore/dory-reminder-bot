package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

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

// Ограничения на данные напоминания.
const (
	// MaxTextLen ограничивает длину текста: он уходит в сообщение Telegram (лимит 4096 символов)
	// вместе с оформлением, и без верхней границы одно напоминание может забить всю выдачу.
	MaxTextLen = 500
	// MaxRepeatEvery — верхняя граница интервала «каждые N дней».
	MaxRepeatEvery = 365
	// MaxRemindersPerChat ограничивает число напоминаний в одном чате.
	MaxRemindersPerChat = 100
)

// Ошибки валидации напоминания.
var (
	// ErrEmptyText возвращается, если после очистки текст оказался пустым.
	ErrEmptyText = errors.New("reminder text cannot be empty")
	// ErrTextTooLong возвращается при превышении MaxTextLen.
	ErrTextTooLong = fmt.Errorf("reminder text cannot exceed %d characters", MaxTextLen)
	// ErrInvalidChatID возвращается при нулевом идентификаторе чата.
	ErrInvalidChatID = errors.New("invalid chat ID")
	// ErrInvalidRepeat возвращается при неизвестном или несогласованном типе повтора.
	ErrInvalidRepeat = errors.New("invalid repeat configuration")
	// ErrTooManyReminders возвращается при превышении MaxRemindersPerChat.
	ErrTooManyReminders = fmt.Errorf("chat cannot have more than %d reminders", MaxRemindersPerChat)
)

// IsValid сообщает, входит ли значение в известный диапазон типов повтора.
func (r RepeatType) IsValid() bool {
	return r >= RepeatNone && r <= RepeatEveryYear
}

// Reminder описывает напоминание пользователя.
type Reminder struct {
	ID          int64
	ChatID      int64
	Text        string
	NextTime    time.Time
	Repeat      RepeatType
	RepeatDays  []int // для дней недели/месяца
	RepeatEvery int   // для N дней
	Paused      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Normalize приводит поля к каноническому виду: чистит текст и обнуляет параметры повтора,
// не относящиеся к выбранному типу.
//
// Без этого в repeat_every оседает мусор (мастер добавления писал туда день недели),
// а Advance читает поля, которые к текущему типу повтора отношения не имеют.
func (r *Reminder) Normalize() {
	r.Text = sanitizeText(r.Text)
	r.NextTime = r.NextTime.UTC()

	switch r.Repeat {
	case RepeatEveryWeek, RepeatEveryMonth:
		r.RepeatEvery = 0
	case RepeatEveryNDays:
		r.RepeatDays = nil
	case RepeatNone, RepeatEveryDay, RepeatEveryYear:
		r.RepeatDays = nil
		r.RepeatEvery = 0
	}
}

// Validate проверяет напоминание перед сохранением.
func (r *Reminder) Validate() error {
	if r.ChatID == 0 {
		return ErrInvalidChatID
	}
	if r.Text == "" {
		return ErrEmptyText
	}
	if len([]rune(r.Text)) > MaxTextLen {
		return ErrTextTooLong
	}
	if !r.Repeat.IsValid() {
		return fmt.Errorf("%w: unknown repeat type %d", ErrInvalidRepeat, r.Repeat)
	}
	if r.NextTime.IsZero() {
		return fmt.Errorf("%w: next time is not set", ErrInvalidRepeat)
	}

	switch r.Repeat {
	case RepeatEveryWeek:
		for _, d := range r.RepeatDays {
			if d < 0 || d > 6 {
				return fmt.Errorf("%w: weekday %d is out of range 0..6", ErrInvalidRepeat, d)
			}
		}
	case RepeatEveryMonth:
		for _, d := range r.RepeatDays {
			if d < 1 || d > 31 {
				return fmt.Errorf("%w: day of month %d is out of range 1..31", ErrInvalidRepeat, d)
			}
		}
	case RepeatEveryNDays:
		if r.RepeatEvery < 1 || r.RepeatEvery > MaxRepeatEvery {
			return fmt.Errorf("%w: interval %d is out of range 1..%d",
				ErrInvalidRepeat, r.RepeatEvery, MaxRepeatEvery)
		}
	case RepeatNone, RepeatEveryDay, RepeatEveryYear:
		// Дополнительных параметров нет.
	}

	return nil
}

// sanitizeText удаляет управляющие символы и лишние пробелы по краям.
//
// Переводы строк оставляем: многострочные напоминания — нормальный сценарий.
func sanitizeText(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, s)

	return strings.TrimSpace(cleaned)
}
