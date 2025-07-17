package utils

import (
	"strconv"
	"strings"
	"time"
)

// TimeCalculator содержит логику расчета времени для напоминаний
type TimeCalculator struct{}

// NewTimeCalculator создает новый экземпляр TimeCalculator
func NewTimeCalculator() *TimeCalculator {
	return &TimeCalculator{}
}

// GetNextTimeToday вычисляет время для напоминания на сегодня
func (tc *TimeCalculator) GetNextTimeToday(now time.Time, t time.Time) time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if nextTime.Before(now) {
		nextTime = nextTime.Add(24 * time.Hour)
	}
	return nextTime
}

// GetNextTimeTomorrow вычисляет время для напоминания на завтра
func (tc *TimeCalculator) GetNextTimeTomorrow(now time.Time, t time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).Add(24 * time.Hour)
}

// GetNextTimeEveryDay вычисляет время для ежедневного напоминания
func (tc *TimeCalculator) GetNextTimeEveryDay(now time.Time, t time.Time) time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if nextTime.Before(now) {
		nextTime = nextTime.Add(24 * time.Hour)
	}
	return nextTime
}

// GetNextTimeWeek вычисляет время для еженедельного напоминания
func (tc *TimeCalculator) GetNextTimeWeek(now time.Time, t time.Time, interval int) time.Time {
	days := (interval - int(now.Weekday()) + 7) % 7
	if days == 0 && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute())) {
		days = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).AddDate(0, 0, days)
}

// GetNextTimeMonth вычисляет время для ежемесячного напоминания
func (tc *TimeCalculator) GetNextTimeMonth(now time.Time, t time.Time, interval int) time.Time {
	month := now.Month()
	year := now.Year()
	if now.Day() > interval || (now.Day() == interval && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))) {
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return time.Date(year, month, interval, t.Hour(), t.Minute(), 0, 0, now.Location())
}

// GetNextTimeYear вычисляет время для ежегодного напоминания
func (tc *TimeCalculator) GetNextTimeYear(now time.Time, t time.Time, date string) time.Time {
	day, _ := strconv.Atoi(strings.Split(date, ".")[0])
	mon, _ := strconv.Atoi(strings.Split(date, ".")[1])
	year := now.Year()
	if now.Month() > time.Month(mon) || (now.Month() == time.Month(mon) && (now.Day() > day || (now.Day() == day && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))))) {
		year++
	}
	return time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, now.Location())
}

// GetNextTimeDate вычисляет время для разового напоминания на конкретную дату
func (tc *TimeCalculator) GetNextTimeDate(t time.Time, date string) time.Time {
	parts := strings.Split(date, ".")
	if len(parts) == 3 {
		day, _ := strconv.Atoi(parts[0])
		mon, _ := strconv.Atoi(parts[1])
		year, _ := strconv.Atoi(parts[2])
		return time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, time.Now().Location())
	}
	return time.Time{}
}

// GetNextTimeNDays вычисляет время для напоминания каждые N дней
func (tc *TimeCalculator) GetNextTimeNDays(startTime time.Time, t time.Time, interval int) time.Time {
	nextTime := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), t.Hour(), t.Minute(), 0, 0, startTime.Location())
	return nextTime.AddDate(0, 0, interval)
}
