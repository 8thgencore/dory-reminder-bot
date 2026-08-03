package webapp

import (
	"fmt"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/scheduling"
	"github.com/8thgencore/dory-reminder-bot/pkg/validator"
)

// timeLayout — формат времени суток, который присылает клиент.
const timeLayout = "15:04"

// applyRequest накладывает поля запроса на напоминание и пересчитывает время срабатывания.
//
// Все поля запроса — указатели, поэтому один и тот же код обслуживает и POST (напоминание
// пустое, обязательные поля проверяются), и PATCH (пропущенные поля сохраняют значение).
func (s *server) applyRequest(rem *domain.Reminder, req reminderRequest, loc *time.Location) error {
	if req.Text != nil {
		rem.Text = *req.Text
	}
	if req.Paused != nil {
		rem.Paused = *req.Paused
	}
	if req.RepeatDays != nil {
		rem.RepeatDays = *req.RepeatDays
	}
	if req.RepeatEvery != nil {
		rem.RepeatEvery = *req.RepeatEvery
	}
	if req.Repeat != nil {
		repeat, err := parseRepeat(*req.Repeat)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrInvalidRepeat, err)
		}
		rem.Repeat = repeat
	}

	// Время срабатывания пересчитывается, только если клиент прислал что-то влияющее
	// на расписание. Иначе PATCH с одним лишь paused сдвинул бы ближайший запуск.
	if !affectsSchedule(req) {
		return nil
	}

	clock, err := resolveClock(req, rem, loc)
	if err != nil {
		return err
	}

	next, err := s.firstOccurrence(rem, req, clock, loc)
	if err != nil {
		return err
	}
	rem.NextTime = next.UTC()

	return nil
}

// affectsSchedule сообщает, влияет ли запрос на расписание.
func affectsSchedule(req reminderRequest) bool {
	return req.Time != nil || req.Date != nil || req.Repeat != nil ||
		req.RepeatDays != nil || req.RepeatEvery != nil
}

// resolveClock определяет время суток: из запроса или из уже сохранённого напоминания.
func resolveClock(req reminderRequest, rem *domain.Reminder, loc *time.Location) (time.Time, error) {
	if req.Time == nil {
		if rem.NextTime.IsZero() {
			return time.Time{}, fmt.Errorf("%w: time is required", scheduling.ErrInvalidDate)
		}

		return rem.NextTime.In(loc), nil
	}

	clock, err := time.ParseInLocation(timeLayout, *req.Time, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q is not a valid HH:MM time", scheduling.ErrInvalidDate, *req.Time)
	}

	return clock, nil
}

// firstOccurrence вычисляет ближайшее срабатывание для выбранного типа повтора.
func (s *server) firstOccurrence(
	rem *domain.Reminder,
	req reminderRequest,
	clock time.Time,
	loc *time.Location,
) (time.Time, error) {
	now := time.Now().In(loc)

	switch rem.Repeat {
	case domain.RepeatNone:
		if req.Date == nil {
			return time.Time{}, fmt.Errorf("%w: date is required for one-time reminders", scheduling.ErrInvalidDate)
		}

		return scheduling.AtDate(clock, *req.Date, loc)

	case domain.RepeatEveryDay:
		return scheduling.NextToday(now, clock), nil

	case domain.RepeatEveryWeek:
		return s.earliestWeekday(now, clock, rem.RepeatDays)

	case domain.RepeatEveryMonth:
		if len(rem.RepeatDays) == 0 {
			return time.Time{}, fmt.Errorf("%w: day of month is required", scheduling.ErrInvalidDate)
		}

		return scheduling.NextMonthDay(now, clock, rem.RepeatDays[0])

	case domain.RepeatEveryNDays:
		return s.firstNDaysOccurrence(rem, req, now, clock, loc)

	case domain.RepeatEveryYear:
		if req.Date == nil {
			return time.Time{}, fmt.Errorf("%w: date is required for yearly reminders", scheduling.ErrInvalidDate)
		}
		dayMonth, err := toDayMonth(*req.Date)
		if err != nil {
			return time.Time{}, err
		}

		return scheduling.NextYearDay(now, clock, dayMonth)
	}

	return time.Time{}, fmt.Errorf("%w: unsupported repeat type", domain.ErrInvalidRepeat)
}

// earliestWeekday выбирает ближайший из выбранных дней недели.
func (s *server) earliestWeekday(now, clock time.Time, days []int) (time.Time, error) {
	if len(days) == 0 {
		return time.Time{}, fmt.Errorf("%w: at least one weekday is required", scheduling.ErrInvalidDate)
	}

	var earliest time.Time
	for _, day := range days {
		candidate, err := scheduling.NextWeekday(now, clock, day)
		if err != nil {
			return time.Time{}, err
		}
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}

	return earliest, nil
}

// firstNDaysOccurrence определяет первый запуск повтора «каждые N дней».
//
// Дата старта необязательна: без неё отсчёт начинается с ближайшего наступления
// указанного времени, что для Mini App естественнее, чем обязательный ввод даты.
func (s *server) firstNDaysOccurrence(
	rem *domain.Reminder,
	req reminderRequest,
	now, clock time.Time,
	loc *time.Location,
) (time.Time, error) {
	if rem.RepeatEvery < 1 {
		return time.Time{}, fmt.Errorf("%w: repeat_every must be at least 1", scheduling.ErrInvalidInterval)
	}

	if req.Date == nil {
		return scheduling.NextToday(now, clock), nil
	}

	start, err := validator.ParseDateDDMMYYYY(*req.Date, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q is not a valid DD.MM.YYYY date", scheduling.ErrInvalidDate, *req.Date)
	}

	next, err := scheduling.NextNDays(start, clock, rem.RepeatEvery)
	if err != nil {
		return time.Time{}, err
	}

	// Дата старта может быть в прошлом: пользователь вправе задать её задним числом,
	// чтобы попасть в нужный цикл. Догоняем до первого срабатывания в будущем,
	// иначе напоминание прилетело бы сразу после сохранения.
	for !next.After(now) {
		next = next.AddDate(0, 0, rem.RepeatEvery)
	}

	return next, nil
}

// toDayMonth отбрасывает год у даты ДД.ММ.ГГГГ: ежегодный повтор задаётся днём и месяцем.
func toDayMonth(date string) (string, error) {
	if validator.IsDateDDMM(date) {
		return date, nil
	}
	if !validator.IsDateDDMMYYYY(date) {
		return "", fmt.Errorf("%w: %q is not a valid date", scheduling.ErrInvalidDate, date)
	}

	return date[:5], nil
}
