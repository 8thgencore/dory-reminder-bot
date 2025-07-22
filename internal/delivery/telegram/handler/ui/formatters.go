package ui

import (
	"fmt"
	"time"

	usecase_domain "github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// Константы для типов повторов
const (
	repeatOnce    = "разово"
	repeatDaily   = "ежедневно"
	repeatWeekly  = "еженедельно"
	repeatMonthly = "ежемесячно"
	repeatYearly  = "ежегодно"
)

// FormatRepeat форматирует режим повтора напоминания для отображения
func FormatRepeat(r *usecase_domain.Reminder) string {
	switch r.Repeat {
	case usecase_domain.RepeatNone:
		return repeatOnce
	case usecase_domain.RepeatEveryDay:
		return repeatDaily
	case usecase_domain.RepeatEveryWeek:
		if len(r.RepeatDays) > 0 {
			weekday := r.RepeatDays[0]
			weekdayName := getWeekdayName(weekday)
			return fmt.Sprintf("%s (%s)", repeatWeekly, weekdayName)
		}

		return repeatWeekly
	case usecase_domain.RepeatEveryMonth:
		if len(r.RepeatDays) > 0 {
			return fmt.Sprintf("%s (%d-го числа)", repeatMonthly, r.RepeatDays[0])
		}
		return repeatMonthly
	case usecase_domain.RepeatEveryYear:
		return repeatYearly
	case usecase_domain.RepeatEveryNDays:
		return fmt.Sprintf("каждые %d дней", r.RepeatEvery)
	default:
		return "-"
	}
}

// FormatRepeatWithDetails форматирует режим повтора с дополнительными деталями
func FormatRepeatWithDetails(r *usecase_domain.Reminder, _ *time.Location) string {
	switch r.Repeat {
	case usecase_domain.RepeatNone:
		return repeatOnce
	case usecase_domain.RepeatEveryDay:
		return repeatDaily
	case usecase_domain.RepeatEveryWeek:
		if len(r.RepeatDays) > 0 {
			weekday := r.RepeatDays[0]
			weekdayName := getWeekdayName(weekday)
			return fmt.Sprintf("%s (%s)", repeatWeekly, weekdayName)
		}

		return repeatWeekly
	case usecase_domain.RepeatEveryMonth:
		if len(r.RepeatDays) > 0 {
			return fmt.Sprintf("%s (%d-го числа)", repeatMonthly, r.RepeatDays[0])
		}
		return repeatMonthly
	case usecase_domain.RepeatEveryYear:
		return repeatYearly
	case usecase_domain.RepeatEveryNDays:
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
	return "" // Активные напоминания не показывают статус
}

// FormatTime форматирует время напоминания в указанном часовом поясе
func FormatTime(nextTime time.Time, loc *time.Location) string {
	nextTimeInTZ := nextTime.In(loc)
	return nextTimeInTZ.Format("02.01.2006 в 15:04")
}

// getWeekdayName возвращает название дня недели на русском
func getWeekdayName(weekday int) string {
	weekdays := map[int]string{
		0: "воскресенье",
		1: "понедельник",
		2: "вторник",
		3: "среда",
		4: "четверг",
		5: "пятница",
		6: "суббота",
	}
	if name, ok := weekdays[weekday]; ok {
		return name
	}

	return "неизвестно"
}
