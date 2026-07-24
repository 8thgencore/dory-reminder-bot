package scheduling

import (
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// berlin используется для проверок DST: в Europe/Berlin переход происходит в ночь
// на последнее воскресенье марта и октября.
func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	return loc
}

// at собирает время в указанном поясе.
func at(loc *time.Location, year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func TestAdvance_EveryDayKeepsWallClockAcrossDST(t *testing.T) {
	loc := berlin(t)

	tests := []struct {
		name string
		next time.Time
		now  time.Time
		want time.Time
	}{
		{
			name: "обычные сутки",
			next: at(loc, 2025, time.June, 10, 9, 0),
			now:  at(loc, 2025, time.June, 10, 9, 0),
			want: at(loc, 2025, time.June, 11, 9, 0),
		},
		{
			name: "переход на летнее время",
			next: at(loc, 2025, time.March, 29, 9, 0),
			now:  at(loc, 2025, time.March, 29, 9, 1),
			want: at(loc, 2025, time.March, 30, 9, 0),
		},
		{
			name: "переход на зимнее время",
			next: at(loc, 2025, time.October, 25, 9, 0),
			now:  at(loc, 2025, time.October, 25, 9, 1),
			want: at(loc, 2025, time.October, 26, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.Reminder{Repeat: domain.RepeatEveryDay, NextTime: tt.next.UTC()}

			got, err := Advance(r, tt.now, loc)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got.In(loc))
			// Стенное время обязано сохраниться, даже если длительность суток изменилась.
			assert.Equal(t, 9, got.In(loc).Hour())
			assert.Equal(t, 0, got.In(loc).Minute())
		})
	}
}

func TestAdvance_Weekly(t *testing.T) {
	loc := berlin(t)

	tests := []struct {
		name string
		days []int
		next time.Time
		want time.Time
	}{
		{
			name: "один день недели — ровно неделя",
			days: []int{2}, // вторник
			next: at(loc, 2025, time.June, 10, 9, 0),
			want: at(loc, 2025, time.June, 17, 9, 0),
		},
		{
			name: "несколько дней — ближайший следующий",
			days: []int{1, 3, 5}, // пн, ср, пт
			next: at(loc, 2025, time.June, 9, 9, 0),
			want: at(loc, 2025, time.June, 11, 9, 0),
		},
		{
			name: "несколько дней — переход через выходные",
			days: []int{1, 3, 5},
			next: at(loc, 2025, time.June, 13, 9, 0), // пятница
			want: at(loc, 2025, time.June, 16, 9, 0), // понедельник
		},
		{
			name: "пустой список — та же неделя",
			days: nil,
			next: at(loc, 2025, time.June, 10, 9, 0),
			want: at(loc, 2025, time.June, 17, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.Reminder{Repeat: domain.RepeatEveryWeek, RepeatDays: tt.days, NextTime: tt.next.UTC()}

			got, err := Advance(r, tt.next, loc)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got.In(loc))
		})
	}
}

func TestAdvance_MonthlyClampsShortMonths(t *testing.T) {
	loc := berlin(t)

	tests := []struct {
		name string
		day  int
		next time.Time
		want time.Time
	}{
		{
			name: "31 января не перескакивает в март",
			day:  31,
			next: at(loc, 2025, time.January, 31, 9, 0),
			want: at(loc, 2025, time.February, 28, 9, 0),
		},
		{
			name: "после обрезки возвращается исходное число",
			day:  31,
			next: at(loc, 2025, time.February, 28, 9, 0),
			want: at(loc, 2025, time.March, 31, 9, 0),
		},
		{
			name: "31 января високосного года",
			day:  31,
			next: at(loc, 2024, time.January, 31, 9, 0),
			want: at(loc, 2024, time.February, 29, 9, 0),
		},
		{
			name: "смена года",
			day:  15,
			next: at(loc, 2025, time.December, 15, 9, 0),
			want: at(loc, 2026, time.January, 15, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &domain.Reminder{
				Repeat:     domain.RepeatEveryMonth,
				RepeatDays: []int{tt.day},
				NextTime:   tt.next.UTC(),
			}

			got, err := Advance(r, tt.next, loc)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %s, got %s", tt.want, got.In(loc))
		})
	}
}

func TestAdvance_Yearly(t *testing.T) {
	loc := berlin(t)

	t.Run("обычная дата", func(t *testing.T) {
		next := at(loc, 2025, time.June, 10, 9, 0)
		r := &domain.Reminder{Repeat: domain.RepeatEveryYear, NextTime: next.UTC()}

		got, err := Advance(r, next, loc)
		require.NoError(t, err)
		assert.True(t, at(loc, 2026, time.June, 10, 9, 0).Equal(got))
	})

	t.Run("29 февраля обрезается до 28-го", func(t *testing.T) {
		next := at(loc, 2024, time.February, 29, 9, 0)
		r := &domain.Reminder{Repeat: domain.RepeatEveryYear, NextTime: next.UTC()}

		got, err := Advance(r, next, loc)
		require.NoError(t, err)
		assert.True(t, at(loc, 2025, time.February, 28, 9, 0).Equal(got))
	})
}

func TestAdvance_EveryNDays(t *testing.T) {
	loc := berlin(t)
	next := at(loc, 2025, time.June, 10, 9, 0)

	r := &domain.Reminder{Repeat: domain.RepeatEveryNDays, RepeatEvery: 3, NextTime: next.UTC()}

	got, err := Advance(r, next, loc)
	require.NoError(t, err)
	assert.True(t, at(loc, 2025, time.June, 13, 9, 0).Equal(got))
}

// Бот мог простоять неделю: Advance обязан догнать пропущенные срабатывания за один вызов
// и вернуть время строго в будущем.
func TestAdvance_CatchesUpAfterDowntime(t *testing.T) {
	loc := berlin(t)

	r := &domain.Reminder{
		Repeat:   domain.RepeatEveryDay,
		NextTime: at(loc, 2025, time.June, 1, 9, 0).UTC(),
	}
	now := at(loc, 2025, time.June, 10, 12, 0)

	got, err := Advance(r, now, loc)
	require.NoError(t, err)

	assert.True(t, got.After(now), "next time must be in the future")
	assert.True(t, at(loc, 2025, time.June, 11, 9, 0).Equal(got), "got %s", got.In(loc))
}

func TestAdvance_Errors(t *testing.T) {
	loc := time.UTC
	now := time.Date(2025, time.June, 10, 9, 0, 0, 0, loc)

	tests := []struct {
		name     string
		reminder *domain.Reminder
		wantErr  error
	}{
		{
			name:     "разовое напоминание не повторяется",
			reminder: &domain.Reminder{Repeat: domain.RepeatNone, NextTime: now},
			wantErr:  ErrNotRepeating,
		},
		{
			name:     "нулевой интервал",
			reminder: &domain.Reminder{Repeat: domain.RepeatEveryNDays, RepeatEvery: 0, NextTime: now},
			wantErr:  ErrInvalidInterval,
		},
		{
			name:     "неизвестный тип повтора",
			reminder: &domain.Reminder{Repeat: domain.RepeatType(42), NextTime: now},
			wantErr:  domain.ErrInvalidRepeat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Advance(tt.reminder, now, loc)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// nil-пояс не должен приводить к панике: планировщик подставляет пояс чата,
// который может не загрузиться.
func TestAdvance_NilLocationFallsBackToUTC(t *testing.T) {
	next := time.Date(2025, time.June, 10, 9, 0, 0, 0, time.UTC)
	r := &domain.Reminder{Repeat: domain.RepeatEveryDay, NextTime: next}

	got, err := Advance(r, next, nil)
	require.NoError(t, err)
	assert.True(t, time.Date(2025, time.June, 11, 9, 0, 0, 0, time.UTC).Equal(got))
}
