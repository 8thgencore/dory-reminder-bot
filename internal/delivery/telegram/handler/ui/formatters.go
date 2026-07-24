package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// Константы для типов повторов
const (
	repeatOnce    = "разово"
	repeatDaily   = "ежедневно"
	repeatWeekly  = "еженедельно"
	repeatMonthly = "ежемесячно"
	repeatYearly  = "ежегодно"
)

// weekdayNames — названия дней недели по индексу time.Weekday (0 — воскресенье).
var weekdayNames = [...]string{
	"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота",
}

// FormatRepeat форматирует режим повтора напоминания для отображения.
func FormatRepeat(r *domain.Reminder) string {
	switch r.Repeat {
	case domain.RepeatNone:
		return repeatOnce

	case domain.RepeatEveryDay:
		return repeatDaily

	case domain.RepeatEveryWeek:
		// Напоминание может повторяться в нескольких днях недели: через Mini App
		// их выбирают списком, и показать нужно все.
		if names := weekdayList(r.RepeatDays); names != "" {
			return fmt.Sprintf("%s (%s)", repeatWeekly, names)
		}

		return repeatWeekly

	case domain.RepeatEveryMonth:
		if len(r.RepeatDays) > 0 {
			return fmt.Sprintf("%s (%d-го числа)", repeatMonthly, r.RepeatDays[0])
		}

		return repeatMonthly

	case domain.RepeatEveryYear:
		return repeatYearly

	case domain.RepeatEveryNDays:
		return fmt.Sprintf("каждые %d дней", r.RepeatEvery)

	default:
		return "-"
	}
}

// FormatStatus форматирует статус напоминания
func FormatStatus(paused bool) string {
	if paused {
		return "🔴 Приостановлено"
	}

	// Активные напоминания статус не показывают: он был бы шумом в каждой строке.
	return ""
}

// FormatTime форматирует время напоминания в указанном часовом поясе
func FormatTime(nextTime time.Time, loc *time.Location) string {
	return nextTime.In(loc).Format("02.01.2006 в 15:04")
}

// weekdayList собирает названия дней недели через запятую.
func weekdayList(days []int) string {
	names := make([]string, 0, len(days))
	for _, day := range days {
		if day >= 0 && day < len(weekdayNames) {
			names = append(names, weekdayNames[day])
		}
	}

	return strings.Join(names, ", ")
}
