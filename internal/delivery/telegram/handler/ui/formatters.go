package ui

import (
	"fmt"

	usecase_domain "github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// FormatRepeat форматирует режим повтора напоминания для отображения
func FormatRepeat(r *usecase_domain.Reminder) string {
	switch r.Repeat {
	case usecase_domain.RepeatNone:
		return "разово"
	case usecase_domain.RepeatEveryDay:
		return "ежедневно"
	case usecase_domain.RepeatEveryWeek:
		return "еженедельно"
	case usecase_domain.RepeatEveryMonth:
		return "ежемесячно"
	case usecase_domain.RepeatEveryNDays:
		return fmt.Sprintf("каждые %d дней", r.RepeatEvery)
	default:
		return "-"
	}
}
