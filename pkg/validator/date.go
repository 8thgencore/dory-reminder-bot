package validator

import (
	"regexp"
	"strconv"
)

// IsDateDDMM проверяет дату в формате ДД.ММ
func IsDateDDMM(s string) bool {
	matched, _ := regexp.MatchString(`^\d{2}\.\d{2}$`, s)
	return matched
}

// IsDateDDMMYYYY проверяет дату в формате ДД.ММ.ГГГГ
func IsDateDDMMYYYY(s string) bool {
	matched, _ := regexp.MatchString(`^\d{2}\.\d{2}\.\d{4}$`, s)
	return matched
}

// IsInterval проверяет, что строка — целое число > 0
func IsInterval(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0
}

// IsNotEmpty проверяет, что строка не пустая (для текста напоминания)
func IsNotEmpty(s string) bool {
	return len([]rune(s)) > 0
}
