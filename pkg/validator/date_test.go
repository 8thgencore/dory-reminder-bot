package validator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDateDDMM(t *testing.T) {
	valid := []string{"01.01", "13.06", "29.02", "31.12"}
	for _, s := range valid {
		assert.True(t, IsDateDDMM(s), "expected %q to be valid", s)
	}

	// Регулярное выражение ^\d{2}\.\d{2}$ пропускало все эти значения.
	invalid := []string{"32.13", "00.00", "13.13", "32.01", "1.1", "13.6", "", "abc", "13.06.2025"}
	for _, s := range invalid {
		assert.False(t, IsDateDDMM(s), "expected %q to be invalid", s)
	}
}

func TestIsDateDDMMYYYY(t *testing.T) {
	valid := []string{"01.01.2025", "29.02.2024", "31.12.2099"}
	for _, s := range valid {
		assert.True(t, IsDateDDMMYYYY(s), "expected %q to be valid", s)
	}

	invalid := []string{
		"32.13.2025", // ни дня, ни месяца такого нет
		"29.02.2025", // 2025 не високосный
		"31.04.2025", // в апреле 30 дней
		"00.00.0000",
		"1.1.2025",
		"13.06",
		"",
	}
	for _, s := range invalid {
		assert.False(t, IsDateDDMMYYYY(s), "expected %q to be invalid", s)
	}
}

func TestParseDateDDMMYYYY(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	got, err := ParseDateDDMMYYYY("13.06.2025", loc)
	require.NoError(t, err)
	assert.Equal(t, 2025, got.Year())
	assert.Equal(t, time.June, got.Month())
	assert.Equal(t, 13, got.Day())
	assert.Equal(t, loc, got.Location())

	_, err = ParseDateDDMMYYYY("32.13.2025", loc)
	assert.Error(t, err)
}

func TestParseDayMonth(t *testing.T) {
	day, month, err := ParseDayMonth("13.06")
	require.NoError(t, err)
	assert.Equal(t, 13, day)
	assert.Equal(t, time.June, month)

	_, _, err = ParseDayMonth("32.13")
	assert.Error(t, err)
}

func TestIsInterval(t *testing.T) {
	for _, s := range []string{"1", "10", "365"} {
		assert.True(t, IsInterval(s), "expected %q to be valid", s)
	}

	// Верхняя граница появилась вместе с проверкой: без неё принималось 999999999.
	for _, s := range []string{"0", "-1", "366", "999999999", "", "abc", "1.5"} {
		assert.False(t, IsInterval(s), "expected %q to be invalid", s)
	}
}

func TestIsDayOfMonth(t *testing.T) {
	for _, s := range []string{"1", "15", "31"} {
		assert.True(t, IsDayOfMonth(s), "expected %q to be valid", s)
	}

	for _, s := range []string{"0", "32", "-1", "", "abc"} {
		assert.False(t, IsDayOfMonth(s), "expected %q to be invalid", s)
	}
}

func TestIsTime(t *testing.T) {
	for _, s := range []string{"00:00", "09:30", "23:59"} {
		assert.True(t, IsTime(s), "expected %q to be valid", s)
	}

	for _, s := range []string{"24:00", "25:00", "12:60", "9:30", "", "abc"} {
		assert.False(t, IsTime(s), "expected %q to be invalid", s)
	}
}

func TestIsNotEmpty(t *testing.T) {
	assert.True(t, IsNotEmpty("текст"))
	assert.False(t, IsNotEmpty(""))
}
