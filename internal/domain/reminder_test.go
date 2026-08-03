package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validReminder() *Reminder {
	return &Reminder{
		ChatID:   42,
		Text:     "напомнить",
		NextTime: time.Date(2026, time.July, 31, 9, 0, 0, 0, time.UTC),
		Repeat:   RepeatNone,
	}
}

func TestReminderNormalize(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	reminder := &Reminder{
		ChatID:      42,
		Text:        " \x00напомнить\n ",
		NextTime:    time.Date(2026, time.July, 31, 12, 0, 0, 0, loc),
		Repeat:      RepeatEveryNDays,
		RepeatDays:  []int{1, 2},
		RepeatEvery: 5,
	}

	reminder.Normalize()

	assert.Equal(t, "напомнить", reminder.Text)
	assert.Equal(t, time.UTC, reminder.NextTime.Location())
	assert.Equal(t, time.Date(2026, time.July, 31, 9, 0, 0, 0, time.UTC), reminder.NextTime)
	assert.Nil(t, reminder.RepeatDays)
	assert.Equal(t, 5, reminder.RepeatEvery)
}

func TestReminderValidate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Reminder)
		want   error
	}{
		{
			name:   "valid",
			change: func(*Reminder) {},
		},
		{
			name:   "missing chat",
			change: func(r *Reminder) { r.ChatID = 0 },
			want:   ErrInvalidChatID,
		},
		{
			name:   "empty text",
			change: func(r *Reminder) { r.Text = "" },
			want:   ErrEmptyText,
		},
		{
			name:   "text too long",
			change: func(r *Reminder) { r.Text = strings.Repeat("я", MaxTextLen+1) },
			want:   ErrTextTooLong,
		},
		{
			name:   "missing time",
			change: func(r *Reminder) { r.NextTime = time.Time{} },
			want:   ErrInvalidRepeat,
		},
		{
			name: "invalid weekday",
			change: func(r *Reminder) {
				r.Repeat = RepeatEveryWeek
				r.RepeatDays = []int{7}
			},
			want: ErrInvalidRepeat,
		},
		{
			name: "invalid month day",
			change: func(r *Reminder) {
				r.Repeat = RepeatEveryMonth
				r.RepeatDays = []int{32}
			},
			want: ErrInvalidRepeat,
		},
		{
			name: "invalid interval",
			change: func(r *Reminder) {
				r.Repeat = RepeatEveryNDays
				r.RepeatEvery = MaxRepeatEvery + 1
			},
			want: ErrInvalidRepeat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reminder := validReminder()
			tt.change(reminder)

			err := reminder.Validate()
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.want)
		})
	}
}
