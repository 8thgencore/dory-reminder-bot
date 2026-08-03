package validator

import (
	"strconv"
	"time"
)

// Форматы дат, которые вводит пользователь.
const (
	dayMonthLayout     = "02.01"
	dayMonthYearLayout = "02.01.2006"
)

// MaxRepeatEvery — верхняя граница интервала «каждые N дней».
// Дублирует domain.MaxRepeatEvery, чтобы pkg/ оставался независимым от internal/.
const MaxRepeatEvery = 365

// IsDateDDMM проверяет дату в формате ДД.ММ
//
// Проверка идёт разбором, а не регулярным выражением: 32.13 обязано отсеиваться.
// Год для разбора не важен, но 29.02 должно оставаться допустимым, поэтому подставляем
// високосный.
func IsDateDDMM(s string) bool {
	if len(s) != len(dayMonthLayout) {
		return false
	}
	_, err := time.Parse(dayMonthYearLayout, s+".2024")

	return err == nil
}

// IsDateDDMMYYYY проверяет дату в формате ДД.ММ.ГГГГ
func IsDateDDMMYYYY(s string) bool {
	if len(s) != len(dayMonthYearLayout) {
		return false
	}
	_, err := time.Parse(dayMonthYearLayout, s)

	return err == nil
}

// ParseDateDDMMYYYY разбирает дату ДД.ММ.ГГГГ в указанном часовом поясе.
func ParseDateDDMMYYYY(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(dayMonthYearLayout, s, loc)
}

// ParseInterval разбирает интервал в диапазоне [1, MaxRepeatEvery].
func ParseInterval(s string) (int, bool) {
	n, err := strconv.Atoi(s)

	return n, err == nil && n >= 1 && n <= MaxRepeatEvery
}

// ParseDayOfMonth разбирает число месяца в диапазоне [1, 31].
func ParseDayOfMonth(s string) (int, bool) {
	n, err := strconv.Atoi(s)

	return n, err == nil && n >= 1 && n <= 31
}
