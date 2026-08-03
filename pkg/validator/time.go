package validator

import (
	"time"
)

const timeLayout = "15:04"

// IsTime проверяет, что строка соответствует формату HH:MM
func IsTime(s string) bool {
	if len(s) != len(timeLayout) {
		return false
	}
	_, err := time.Parse(timeLayout, s)

	return err == nil
}
