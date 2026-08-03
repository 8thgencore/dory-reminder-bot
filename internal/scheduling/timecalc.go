// Package scheduling содержит расчёт времени срабатывания напоминаний.
//
// Пакет намеренно вынесен из слоя доставки: одну и ту же арифметику используют мастер
// добавления в Telegram, HTTP API Mini App и планировщик рассылки.
package scheduling

import (
	"errors"
	"fmt"
	"time"
)

// Ошибки расчёта времени.
var (
	// ErrInvalidDate возвращается, если строка даты не разбирается.
	ErrInvalidDate = errors.New("invalid date")
	// ErrInvalidInterval возвращается при неположительном интервале повтора.
	ErrInvalidInterval = errors.New("invalid interval")
	// ErrNotRepeating возвращается при попытке сдвинуть неповторяющееся напоминание.
	ErrNotRepeating = errors.New("reminder does not repeat")
)

// Функции расчёта принимают now уже в часовом поясе чата и возвращают время
// в том же поясе; перевод в UTC для хранения — обязанность вызывающего.

// NextToday вычисляет время для напоминания на сегодня.
// Если время сегодня уже прошло, переносит на завтра.
func NextToday(now, t time.Time) time.Time {
	nextTime := atClock(now, t)
	if !nextTime.After(now) {
		nextTime = nextTime.AddDate(0, 0, 1)
	}

	return nextTime
}

// NextTomorrow вычисляет время для напоминания на завтра.
func NextTomorrow(now, t time.Time) time.Time {
	return atClock(now, t).AddDate(0, 0, 1)
}

// NextWeekday вычисляет время ближайшего срабатывания в заданный день недели (0 — воскресенье).
func NextWeekday(now, t time.Time, weekday int) (time.Time, error) {
	if weekday < 0 || weekday > 6 {
		return time.Time{}, fmt.Errorf("%w: weekday %d is out of range 0..6", ErrInvalidDate, weekday)
	}

	candidate := atClock(now, t)
	days := (weekday - int(candidate.Weekday()) + 7) % 7
	candidate = candidate.AddDate(0, 0, days)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 7)
	}

	return candidate, nil
}

// NextMonthDay вычисляет время ближайшего срабатывания в заданное число месяца.
func NextMonthDay(now, t time.Time, dayOfMonth int) (time.Time, error) {
	if dayOfMonth < 1 || dayOfMonth > 31 {
		return time.Time{}, fmt.Errorf("%w: day of month %d is out of range 1..31", ErrInvalidDate, dayOfMonth)
	}

	candidate := dayInMonth(now.Year(), now.Month(), dayOfMonth, t, now.Location())
	if !candidate.After(now) {
		candidate = dayInMonth(now.Year(), now.Month()+1, dayOfMonth, t, now.Location())
	}

	return candidate, nil
}

// NextYearDay вычисляет время ближайшего срабатывания в заданный день и месяц.
// date — строка в формате ДД.ММ.
func NextYearDay(now, t time.Time, date string) (time.Time, error) {
	day, month, err := parseDayMonth(date)
	if err != nil {
		return time.Time{}, err
	}

	candidate := dayInMonth(now.Year(), month, day, t, now.Location())
	if !candidate.After(now) {
		candidate = dayInMonth(now.Year()+1, month, day, t, now.Location())
	}

	return candidate, nil
}

// AtDate вычисляет время разового напоминания на конкретную дату.
// date — строка в формате ДД.ММ.ГГГГ.
func AtDate(t time.Time, date string, loc *time.Location) (time.Time, error) {
	d, err := time.ParseInLocation(dateLayout, date, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q is not a valid DD.MM.YYYY date", ErrInvalidDate, date)
	}

	return time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}

// NextNDays вычисляет первое срабатывание напоминания «каждые N дней»,
// отсчитывая от даты старта.
func NextNDays(startTime, t time.Time, interval int) (time.Time, error) {
	if interval < 1 {
		return time.Time{}, fmt.Errorf("%w: interval must be at least 1, got %d", ErrInvalidInterval, interval)
	}

	return atClock(startTime, t).AddDate(0, 0, interval), nil
}
