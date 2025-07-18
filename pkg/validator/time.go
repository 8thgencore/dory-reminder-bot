package validator

import (
	"regexp"
	"time"
)

// IsTime проверяет, что строка соответствует формату HH:MM
func IsTime(s string) bool {
	matched, _ := regexp.MatchString(`^\d{2}:\d{2}$`, s)
	if !matched {
		return false
	}
	_, err := time.Parse("15:04", s)

	return err == nil
}

// NextTimeFromString возвращает time.Time для следующего срабатывания на основе строки времени и базовой даты
func NextTimeFromString(s string, base time.Time) time.Time {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return base
	}
	candidate := time.Date(base.Year(), base.Month(), base.Day(), t.Hour(), t.Minute(), 0, 0, base.Location())
	if candidate.Before(base) {
		candidate = candidate.Add(24 * time.Hour)
	}

	return candidate
}
