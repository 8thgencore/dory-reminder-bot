package scheduling

import (
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
)

// maxAdvanceSteps ограничивает догоняющий цикл.
//
// Планировщик обязан догнать пропущенные срабатывания после долгого простоя, но битые данные
// в базе не должны превращать это в бесконечный цикл. Запаса хватает на ~55 лет ежедневных
// напоминаний.
const maxAdvanceSteps = 20_000

// Advance возвращает следующее время срабатывания повторяющегося напоминания,
// строго более позднее, чем after.
//
// Вся арифметика выполняется в часовом поясе чата с сохранением стенных часов и минут:
// «каждый день в 9:00» обязано оставаться девятью утра и после перевода часов.
// Результат возвращается в UTC — в этом виде время хранится в базе.
func Advance(r *domain.Reminder, after time.Time, loc *time.Location) (time.Time, error) {
	if r.Repeat == domain.RepeatNone {
		return time.Time{}, fmt.Errorf("%w: reminder %d", ErrNotRepeating, r.ID)
	}
	if !r.Repeat.IsValid() {
		return time.Time{}, fmt.Errorf("%w: unknown repeat type %d", domain.ErrInvalidRepeat, r.Repeat)
	}
	if r.Repeat == domain.RepeatEveryNDays && r.RepeatEvery < 1 {
		return time.Time{}, fmt.Errorf("%w: interval must be at least 1, got %d", ErrInvalidInterval, r.RepeatEvery)
	}
	if loc == nil {
		loc = time.UTC
	}

	next := r.NextTime.In(loc)
	deadline := after.In(loc)

	for steps := 0; !next.After(deadline); steps++ {
		if steps >= maxAdvanceSteps {
			return time.Time{}, fmt.Errorf("%w: reminder %d did not converge after %d steps",
				ErrInvalidInterval, r.ID, maxAdvanceSteps)
		}
		next = advanceOnce(r, next, loc)
	}

	return next.UTC(), nil
}

// advanceOnce делает ровно один шаг повтора от next.
func advanceOnce(r *domain.Reminder, next time.Time, loc *time.Location) time.Time {
	switch r.Repeat {
	case domain.RepeatEveryDay:
		return stepDays(next, 1)

	case domain.RepeatEveryWeek:
		return nextWeekday(next, r.RepeatDays)

	case domain.RepeatEveryMonth:
		// RepeatDays хранит исходное число месяца. Опираться на next.Day() нельзя:
		// оно могло быть обрезано коротким месяцем, и «31-го числа» после февраля
		// навсегда превратилось бы в «28-го».
		day := next.Day()
		if len(r.RepeatDays) > 0 {
			day = r.RepeatDays[0]
		}

		return dayInMonth(next.Year(), next.Month()+1, day, next, loc)

	case domain.RepeatEveryNDays:
		return stepDays(next, r.RepeatEvery)

	case domain.RepeatEveryYear:
		// 29 февраля в невисокосном году обрезается до 28-го и таким и остаётся:
		// восстановить исходное число из next уже нельзя.
		return dayInMonth(next.Year()+1, next.Month(), next.Day(), next, loc)

	case domain.RepeatNone:
		// Отсеивается вызывающим; ветка нужна для полноты switch.
		return next
	}

	return next
}
