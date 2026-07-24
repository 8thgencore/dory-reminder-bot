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

// ParseDayMonth разбирает дату ДД.ММ и возвращает день и месяц.
func ParseDayMonth(s string) (day int, month time.Month, err error) {
	t, err := time.Parse(dayMonthYearLayout, s+".2024")
	if err != nil {
		return 0, 0, err
	}

	return t.Day(), t.Month(), nil
}

// IsInterval проверяет, что строка — целое число в диапазоне [1, MaxRepeatEvery]
func IsInterval(s string) bool {
	n, err := strconv.Atoi(s)

	return err == nil && n >= 1 && n <= MaxRepeatEvery
}

// IsDayOfMonth проверяет, что строка — число месяца в диапазоне [1, 31]
func IsDayOfMonth(s string) bool {
	n, err := strconv.Atoi(s)

	return err == nil && n >= 1 && n <= 31
}

// IsNotEmpty проверяет, что строка не пустая (для текста напоминания)
func IsNotEmpty(s string) bool {
	return len([]rune(s)) > 0
}
