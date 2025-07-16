package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/pkg/validator"
	tele "gopkg.in/telebot.v4"
)

// Типы напоминаний как константы
const (
	ReminderTypeToday    = "today"
	ReminderTypeTomorrow = "tomorrow"
	ReminderTypeEveryDay = "everyday"
	ReminderTypeWeek     = "week"
	ReminderTypeNDays    = "ndays"
	ReminderTypeMonth    = "month"
	ReminderTypeYear     = "year"
	ReminderTypeDate     = "date"
	ReminderTypeMultiDay = "multiday"
)

func getAddReminderMessage(typ string) string {
	switch typ {
	case ReminderTypeToday:
		return "Во сколько напомнить сегодня? (например, 15:00)"
	case ReminderTypeTomorrow:
		return "Во сколько напомнить завтра? (например, 15:00)"
	case ReminderTypeEveryDay:
		return "Во сколько напоминать каждый день? (например, 09:00)"
	case ReminderTypeWeek:
		return "В какой день недели? (например: понедельник)"
	case ReminderTypeNDays:
		return "Через сколько дней повторять? Например: каждые 10 дней в 12:00"
	case ReminderTypeMonth:
		return "В какой день месяца и во сколько? Например: 5 число 12:00"
	case ReminderTypeYear:
		return "В какую дату и во сколько? Например: 13 июня 15:00"
	case ReminderTypeDate:
		return "Введите дату и время: например, 13.06.2025 15:00"
	case ReminderTypeMultiDay:
		return "Во сколько? (например: 09:00, 14:00, 20:00)"
	default:
		return "Неизвестный тип напоминания"
	}
}

func (h *Handler) getSession(chatID, userID int64) *session.AddReminderSession {
	sess := h.Session.Get(chatID, userID)
	if sess == nil {
		sess = &session.AddReminderSession{UserID: userID, ChatID: chatID, Step: session.StepType}
	}
	return sess
}

func (h *Handler) updateSession(sess *session.AddReminderSession) {
	h.Session.Set(sess)
}

func (h *Handler) HandleAddTypeCallback(c tele.Context, typ string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	sess.Type = typ
	if typ == ReminderTypeWeek {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send("Выберите день недели:", WeekdaysMenu())
	}
	if typ == ReminderTypeMonth {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send("Введите день месяца (1-31):")
	}
	if typ == ReminderTypeYear {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send("Введите дату в формате ДД.ММ:")
	}
	if typ == ReminderTypeNDays {
		sess.Step = session.StepDate
		h.updateSession(sess)
		return c.Send("Введите дату старта в формате ДД.ММ.ГГГГ:")
	}
	sess.Step = session.StepTime
	h.updateSession(sess)
	msg := getAddReminderMessage(typ)
	return c.Send(msg)
}

func (h *Handler) handleStepInterval(c tele.Context, sess *session.AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	slog.Info("[handleStepInterval] step=%v type=%v val='%s' date='%s' interval=%d", sess.Step, sess.Type, val, sess.Date, sess.Interval)
	switch sess.Type {
	case ReminderTypeWeek:
		weekday, ok := parseWeekday(val)
		if !ok {
			return c.Send("Пожалуйста, введите день недели (например, понедельник)")
		}
		sess.Interval = weekday
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval] set weekday=%d, next step=StepTime", weekday)
		return c.Send("Во сколько? (например: 09:00)")
	case ReminderTypeMonth:
		if !validator.IsInterval(val) {
			return c.Send("Пожалуйста, введите число месяца от 1 до 31")
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval] set month=%d, next step=StepTime", n)
		return c.Send("Во сколько? (например: 09:00)")
	case ReminderTypeYear:
		if !validator.IsDateDDMM(val) {
			return c.Send("Пожалуйста, введите дату в формате ДД.ММ (например, 13.06)")
		}
		sess.Date = val
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval] set year_date='%s', next step=StepTime", val)
		return c.Send("Во сколько? (например: 09:00)")
	case ReminderTypeNDays:
		if !validator.IsInterval(val) {
			return c.Send("Пожалуйста, введите интервал в днях (целое число > 0)")
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval] set ndays_interval=%d, next step=StepTime", n)
		return c.Send("Во сколько? (например: 09:00)")
	}
	return nil
}

func (h *Handler) handleStepDate(c tele.Context, sess *session.AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	slog.Info("[handleStepDate] step=%v type=%v val='%s' date='%s' interval=%d", sess.Step, sess.Type, val, sess.Date, sess.Interval)
	if sess.Type == ReminderTypeNDays {
		// Если дата уже есть, значит сейчас ожидается ввод интервала
		if sess.Date != "" && sess.Interval == 0 {
			if !validator.IsInterval(val) {
				return c.Send("Пожалуйста, введите интервал в днях (целое число > 0)")
			}
			n, _ := strconv.Atoi(val)
			sess.Interval = n
			sess.Step = session.StepTime
			h.updateSession(sess)
			slog.Info("[handleStepDate] set interval=%d, next step=StepTime", n)
			return c.Send("Во сколько? (например: 09:00)")
		}
		// Если дата ещё не введена (fallback, но по логике сюда не попадём)
		if !validator.IsDateDDMMYYYY(val) {
			return c.Send("Пожалуйста, введите дату старта в формате ДД.ММ.ГГГГ")
		}
		sess.Date = val
		sess.Step = session.StepInterval
		h.updateSession(sess)
		slog.Info("[handleStepDate] set date='%s', next step=StepInterval", val)
		return c.Send("Введите интервал в днях (целое число > 0):")
	}
	return nil
}

func (h *Handler) handleStepTime(c tele.Context, sess *session.AddReminderSession) error {
	t := strings.TrimSpace(c.Text())
	if !validator.IsTime(t) && sess.Type != ReminderTypeMultiDay {
		return c.Send("Пожалуйста, введите время в формате 15:00")
	}
	sess.Time = t
	sess.Step = session.StepText
	h.updateSession(sess)
	return c.Send("Введите текст напоминания:")
}

func (h *Handler) handleStepText(c tele.Context, sess *session.AddReminderSession) error {
	txt := strings.TrimSpace(c.Text())
	if !validator.IsNotEmpty(txt) {
		return c.Send("Пожалуйста, введите текст напоминания")
	}
	sess.Text = txt
	sess.Step = session.StepConfirm
	h.updateSession(sess)
	return h.handleStepConfirm(c, sess)
}

func (h *Handler) handleStepConfirm(c tele.Context, sess *session.AddReminderSession) error {
	// Создаём напоминание
	err := h.createReminderFromSession(sess)
	h.Session.Delete(sess.ChatID, sess.UserID)
	if err != nil {
		return c.Send("Ошибка при создании напоминания")
	}
	return c.Send("Напоминание создано!")
}

func (h *Handler) HandleAddWizardText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess == nil {
		return nil
	}
	logMsg := "[HandleAddWizardText] chatID=%d userID=%d step=%v type=%v text='%s'"
	slog.Info(
		logMsg,
		"chatID", chatID,
		"userID", userID,
		"step", sess.Step,
		"type", sess.Type,
		"text", c.Text(),
	)
	switch sess.Step {
	case session.StepTime:
		return h.handleStepTime(c, sess)
	case session.StepInterval:
		return h.handleStepInterval(c, sess)
	case session.StepText:
		return h.handleStepText(c, sess)
	case session.StepDate:
		return h.handleStepDate(c, sess)
	}
	return nil
}

func getNextTimeToday(now time.Time, t time.Time) time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if nextTime.Before(now) {
		nextTime = nextTime.Add(24 * time.Hour)
	}
	return nextTime
}

func getNextTimeTomorrow(now time.Time, t time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).Add(24 * time.Hour)
}

func getNextTimeEveryDay(now time.Time, t time.Time) time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if nextTime.Before(now) {
		nextTime = nextTime.Add(24 * time.Hour)
	}
	return nextTime
}

func getNextTimeWeek(now time.Time, t time.Time, interval int) time.Time {
	days := (interval - int(now.Weekday()) + 7) % 7
	if days == 0 && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute())) {
		days = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).AddDate(0, 0, days)
}

func getNextTimeMonth(now time.Time, t time.Time, interval int) time.Time {
	month := now.Month()
	year := now.Year()
	if now.Day() > interval || (now.Day() == interval && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))) {
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	return time.Date(year, month, interval, t.Hour(), t.Minute(), 0, 0, now.Location())
}

func getNextTimeYear(now time.Time, t time.Time, date string) time.Time {
	day, _ := strconv.Atoi(strings.Split(date, ".")[0])
	mon, _ := strconv.Atoi(strings.Split(date, ".")[1])
	year := now.Year()
	if now.Month() > time.Month(mon) || (now.Month() == time.Month(mon) && (now.Day() > day || (now.Day() == day && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))))) {
		year++
	}
	return time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, now.Location())
}

func getNextTimeDate(t time.Time, date string) time.Time {
	parts := strings.Split(date, ".")
	if len(parts) == 3 {
		day, _ := strconv.Atoi(parts[0])
		mon, _ := strconv.Atoi(parts[1])
		year, _ := strconv.Atoi(parts[2])
		return time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, time.Now().Location())
	}
	return time.Time{}
}

func getNextTimeNDays(now time.Time, t time.Time, interval int) time.Time {
	nextTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if nextTime.Before(now) {
		nextTime = nextTime.Add(time.Duration(interval) * 24 * time.Hour)
	}
	return nextTime
}

func getNextTimeMultiDay(now time.Time, times []string) time.Time {
	var nextTime time.Time
	for _, ts := range times {
		t, _ := time.Parse("15:04", strings.TrimSpace(ts))
		candidate := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if candidate.After(now) && (nextTime.IsZero() || candidate.Before(nextTime)) {
			nextTime = candidate
		}
	}
	if nextTime.IsZero() {
		// все времена на сегодня прошли — берём первое на завтра
		t, _ := time.Parse("15:04", strings.TrimSpace(times[0]))
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).Add(24 * time.Hour)
	}
	return nextTime
}

// convertSessionToReminder converts an AddReminderSession to a domain Reminder
func convertSessionToReminder(sess *session.AddReminderSession, nextTime time.Time) *domain.Reminder {
	return &domain.Reminder{
		ChatID:      sess.ChatID,
		UserID:      sess.UserID,
		Text:        sess.Text,
		NextTime:    nextTime,
		Repeat:      typeToRepeat(sess.Type),
		RepeatEvery: sess.Interval,
		Paused:      false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Timezone:    "local",
	}
}

// typeToRepeat converts a string reminder type to a domain RepeatType
func typeToRepeat(typ string) domain.RepeatType {
	switch typ {
	case ReminderTypeEveryDay:
		return domain.RepeatEveryDay
	default:
		return domain.RepeatNone
	}
}

func parseWeekday(s string) (int, bool) {
	weekdays := map[string]int{"воскресенье": 0, "понедельник": 1, "вторник": 2, "среда": 3, "четверг": 4, "пятница": 5, "суббота": 6}
	s = strings.ToLower(strings.TrimSpace(s))
	idx, ok := weekdays[s]
	return idx, ok
}

func (h *Handler) createReminderFromSession(sess *session.AddReminderSession) error {
	now := time.Now()
	var nextTime time.Time
	t, _ := time.Parse("15:04", sess.Time)
	switch sess.Type {
	case ReminderTypeToday:
		nextTime = getNextTimeToday(now, t)
	case ReminderTypeTomorrow:
		nextTime = getNextTimeTomorrow(now, t)
	case ReminderTypeEveryDay:
		nextTime = getNextTimeEveryDay(now, t)
	case ReminderTypeWeek:
		nextTime = getNextTimeWeek(now, t, sess.Interval)
	case ReminderTypeMonth:
		nextTime = getNextTimeMonth(now, t, sess.Interval)
	case ReminderTypeYear:
		nextTime = getNextTimeYear(now, t, sess.Date)
	case ReminderTypeDate:
		nextTime = getNextTimeDate(t, sess.Date)
	case ReminderTypeNDays:
		nextTime = getNextTimeNDays(now, t, sess.Interval)
	case ReminderTypeMultiDay:
		times := strings.Split(sess.Time, ",")
		nextTime = getNextTimeMultiDay(now, times)
	}

	rem := convertSessionToReminder(sess, nextTime)
	// Для week/month/year/date/multiday сохраняем доп. параметры
	if sess.Type == ReminderTypeWeek {
		rem.Repeat = domain.RepeatEveryWeek
		rem.RepeatDays = []int{sess.Interval}
	} else if sess.Type == ReminderTypeMonth {
		rem.Repeat = domain.RepeatEveryMonth
		rem.RepeatDays = []int{sess.Interval}
	} else if sess.Type == ReminderTypeYear {
		rem.Repeat = domain.RepeatEveryYear
	} else if sess.Type == ReminderTypeDate {
		rem.Repeat = domain.RepeatNone
	} else if sess.Type == ReminderTypeNDays {
		rem.Repeat = domain.RepeatEveryNDays
		rem.RepeatEvery = sess.Interval
	} else if sess.Type == ReminderTypeMultiDay {
		rem.Repeat = domain.RepeatEveryDay
		// RepeatDays можно использовать для хранения времён, если нужно
	}

	return h.Usecase.AddReminder(context.Background(), rem)
}

// Добавить обработчик inline-кнопок для дней недели
func (h *Handler) HandleWeekdayCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)

	slog.Info("HandleWeekdayCallback", "callback_data", data)
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess != nil {
		slog.Info("Session state", "type", sess.Type, "step", sess.Step)
	}
	c.Respond()
	if sess == nil || sess.Type != ReminderTypeWeek || sess.Step != session.StepInterval {
		return c.Send("Ошибка: не ожидается выбор дня недели.")
	}
	if !strings.HasPrefix(data, "weekday_") {
		return c.Send("Ошибка: неверный формат дня недели.")
	}
	weekdayStr := strings.TrimSpace(strings.TrimPrefix(data, "weekday_"))
	weekday, err := strconv.Atoi(weekdayStr)
	if err != nil || weekday < 0 || weekday > 6 {
		return c.Send("Ошибка: неверный день недели.")
	}
	sess.Interval = weekday
	sess.Step = session.StepTime
	h.updateSession(sess)
	return c.Send("Во сколько? (например: 09:00)")
}

// Обработчик inline-кнопок для месяцев
func (h *Handler) HandleMonthCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	slog.Info("HandleMonthCallback", "callback_data", data)
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess == nil || sess.Type != ReminderTypeMonth || sess.Step != session.StepInterval {
		return c.Send("Ошибка: не ожидается выбор месяца.")
	}
	if !strings.HasPrefix(data, "month_") {
		return c.Send("Ошибка: неверный формат месяца.")
	}
	monthStr := strings.TrimSpace(strings.TrimPrefix(data, "month_"))
	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		return c.Send("Ошибка: неверный месяц.")
	}
	sess.Interval = month
	sess.Step = session.StepText
	h.updateSession(sess)
	return c.Send("Введите текст напоминания:")
}
