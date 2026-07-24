package scheduling

import (
	"fmt"
	"time"
)

// dayMonthLayout и dateLayout — форматы дат, которые вводит пользователь.
const (
	dayMonthLayout = "02.01"
	dateLayout     = "02.01.2006"
	// leapYearSuffix подставляется при разборе ДД.ММ, чтобы 29.02 считалось валидной датой.
	leapYearSuffix = ".2024"
)

// atClock берёт дату из base и время из clock, сохраняя часовой пояс base.
func atClock(base, clock time.Time) time.Time {
	return time.Date(
		base.Year(), base.Month(), base.Day(),
		clock.Hour(), clock.Minute(), 0, 0,
		base.Location(),
	)
}

// stepDays сдвигает время на n дней вперёд, заново привязывая стенные часы и минуты.
//
// Именно из-за этого нельзя складывать с 24*time.Hour: на переходе на летнее время
// в сутках 23 или 25 часов, и напоминание уползало бы на час каждые полгода.
func stepDays(t time.Time, n int) time.Time {
	return atClock(t.AddDate(0, 0, n), t)
}

// daysInMonth возвращает число дней в месяце. Нулевой день следующего месяца — последний
// день текущего, чем time.Date и пользуется.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dayInMonth собирает дату из года, месяца и числа, обрезая число до длины месяца.
//
// Обрезка, а не перенос: time.Date для 31 февраля вернул бы 3 марта, и ежемесячное
// напоминание «31-го числа» уезжало бы в следующий месяц. month вне 1..12 нормализуется,
// поэтому вызывающий может смело передавать now.Month()+1.
func dayInMonth(year int, month time.Month, day int, clock time.Time, loc *time.Location) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	if maxDay := daysInMonth(first.Year(), first.Month()); day > maxDay {
		day = maxDay
	}

	return time.Date(first.Year(), first.Month(), day, clock.Hour(), clock.Minute(), 0, 0, loc)
}

// parseDayMonth разбирает строку ДД.ММ.
func parseDayMonth(s string) (int, time.Month, error) {
	if len(s) != len(dayMonthLayout) {
		return 0, 0, fmt.Errorf("%w: %q is not a valid DD.MM date", ErrInvalidDate, s)
	}

	t, err := time.Parse(dateLayout, s+leapYearSuffix)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %q is not a valid DD.MM date", ErrInvalidDate, s)
	}

	return t.Day(), t.Month(), nil
}

// nextWeekday возвращает ближайший после t день из списка дней недели (0 — воскресенье).
// Пустой список означает «тот же день недели через неделю».
func nextWeekday(t time.Time, days []int) time.Time {
	if len(days) == 0 {
		return stepDays(t, 7)
	}

	best := 0
	for _, d := range days {
		if d < 0 || d > 6 {
			continue
		}
		// Сдвиг всегда в 1..7: результат обязан быть строго позже t, поэтому совпадение
		// с текущим днём недели означает полную неделю, а не ноль.
		delta := (d - int(t.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		if best == 0 || delta < best {
			best = delta
		}
	}
	if best == 0 {
		return stepDays(t, 7)
	}

	return stepDays(t, best)
}
