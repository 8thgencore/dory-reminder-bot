package scheduling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clock собирает время суток для передачи в калькулятор.
func clock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func TestGetNextTimeToday(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC

	t.Run("время ещё не наступило", func(t *testing.T) {
		now := at(loc, 2025, time.June, 10, 8, 0)
		got := tc.GetNextTimeToday(now, clock(9, 30))
		assert.True(t, at(loc, 2025, time.June, 10, 9, 30).Equal(got))
	})

	t.Run("время уже прошло — переносится на завтра", func(t *testing.T) {
		now := at(loc, 2025, time.June, 10, 10, 0)
		got := tc.GetNextTimeToday(now, clock(9, 30))
		assert.True(t, at(loc, 2025, time.June, 11, 9, 30).Equal(got))
	})

	t.Run("ровно текущая минута считается прошедшей", func(t *testing.T) {
		now := at(loc, 2025, time.June, 10, 9, 30)
		got := tc.GetNextTimeToday(now, clock(9, 30))
		assert.True(t, at(loc, 2025, time.June, 11, 9, 30).Equal(got))
	})
}

func TestGetNextTimeWeek(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC
	// 10 июня 2025 — вторник.
	now := at(loc, 2025, time.June, 10, 8, 0)

	tests := []struct {
		name    string
		weekday int
		at      time.Time
		want    time.Time
	}{
		{"сегодня, время ещё впереди", 2, clock(9, 0), at(loc, 2025, time.June, 10, 9, 0)},
		{"сегодня, время уже прошло", 2, clock(7, 0), at(loc, 2025, time.June, 17, 7, 0)},
		{"позже на этой неделе", 5, clock(9, 0), at(loc, 2025, time.June, 13, 9, 0)},
		{"на следующей неделе", 1, clock(9, 0), at(loc, 2025, time.June, 16, 9, 0)},
		{"воскресенье", 0, clock(9, 0), at(loc, 2025, time.June, 15, 9, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tc.GetNextTimeWeek(now, tt.at, tt.weekday)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got)
		})
	}

	t.Run("день недели вне диапазона", func(t *testing.T) {
		_, err := tc.GetNextTimeWeek(now, clock(9, 0), 7)
		assert.ErrorIs(t, err, ErrInvalidDate)
	})
}

func TestGetNextTimeMonth(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC

	tests := []struct {
		name string
		now  time.Time
		day  int
		want time.Time
	}{
		{
			name: "число ещё не наступило",
			now:  at(loc, 2025, time.June, 10, 8, 0),
			day:  15,
			want: at(loc, 2025, time.June, 15, 9, 0),
		},
		{
			name: "число уже прошло — следующий месяц",
			now:  at(loc, 2025, time.June, 20, 8, 0),
			day:  15,
			want: at(loc, 2025, time.July, 15, 9, 0),
		},
		{
			// Раньше time.Date(2025, February, 31) молча превращалось в 3 марта.
			name: "31-е число в коротком месяце обрезается",
			now:  at(loc, 2025, time.February, 1, 8, 0),
			day:  31,
			want: at(loc, 2025, time.February, 28, 9, 0),
		},
		{
			name: "смена года",
			now:  at(loc, 2025, time.December, 20, 8, 0),
			day:  5,
			want: at(loc, 2026, time.January, 5, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tc.GetNextTimeMonth(tt.now, clock(9, 0), tt.day)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got)
		})
	}

	t.Run("число вне диапазона", func(t *testing.T) {
		_, err := tc.GetNextTimeMonth(at(loc, 2025, time.June, 1, 8, 0), clock(9, 0), 32)
		assert.ErrorIs(t, err, ErrInvalidDate)
	})
}

func TestGetNextTimeYear(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC

	tests := []struct {
		name string
		now  time.Time
		date string
		want time.Time
	}{
		{
			name: "дата ещё впереди",
			now:  at(loc, 2025, time.June, 10, 8, 0),
			date: "01.09",
			want: at(loc, 2025, time.September, 1, 9, 0),
		},
		{
			name: "дата уже прошла — следующий год",
			now:  at(loc, 2025, time.June, 10, 8, 0),
			date: "01.03",
			want: at(loc, 2026, time.March, 1, 9, 0),
		},
		{
			name: "29 февраля в невисокосном году обрезается",
			now:  at(loc, 2025, time.January, 10, 8, 0),
			date: "29.02",
			want: at(loc, 2025, time.February, 28, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tc.GetNextTimeYear(tt.now, clock(9, 0), tt.date)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got)
		})
	}

	// Раньше здесь была паника: strings.Split(date, ".")[1] на строке без точки.
	t.Run("кривая дата возвращает ошибку, а не панику", func(t *testing.T) {
		for _, bad := range []string{"", "abc", "13", "32.13", "1.1.2025"} {
			_, err := tc.GetNextTimeYear(at(loc, 2025, time.June, 10, 8, 0), clock(9, 0), bad)
			assert.ErrorIs(t, err, ErrInvalidDate, "input %q", bad)
		}
	})
}

func TestGetNextTimeDate(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC

	t.Run("корректная дата", func(t *testing.T) {
		got, err := tc.GetNextTimeDate(clock(9, 0), "13.06.2025", loc)
		require.NoError(t, err)
		assert.True(t, at(loc, 2025, time.June, 13, 9, 0).Equal(got))
	})

	// Раньше при нераспознанной дате возвращалось нулевое время, которое молча
	// сохранялось в базу и срабатывало немедленно.
	t.Run("кривая дата возвращает ошибку", func(t *testing.T) {
		for _, bad := range []string{"", "13.06", "32.13.2025", "13/06/2025"} {
			_, err := tc.GetNextTimeDate(clock(9, 0), bad, loc)
			assert.ErrorIs(t, err, ErrInvalidDate, "input %q", bad)
		}
	})
}

func TestGetNextTimeNDays(t *testing.T) {
	tc := NewTimeCalculator()
	loc := time.UTC

	t.Run("отсчёт от даты старта", func(t *testing.T) {
		got, err := tc.GetNextTimeNDays(at(loc, 2025, time.June, 10, 0, 0), clock(9, 0), 5)
		require.NoError(t, err)
		assert.True(t, at(loc, 2025, time.June, 15, 9, 0).Equal(got))
	})

	t.Run("неположительный интервал", func(t *testing.T) {
		_, err := tc.GetNextTimeNDays(at(loc, 2025, time.June, 10, 0, 0), clock(9, 0), 0)
		assert.ErrorIs(t, err, ErrInvalidInterval)
	})
}
