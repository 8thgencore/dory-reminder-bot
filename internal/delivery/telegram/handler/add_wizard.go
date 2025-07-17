package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/texts"
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
		return texts.PromptToday
	case ReminderTypeTomorrow:
		return texts.PromptTomorrow
	case ReminderTypeEveryDay:
		return texts.PromptEveryDay
	case ReminderTypeWeek:
		return texts.PromptWeek
	case ReminderTypeNDays:
		return texts.PromptNDays
	case ReminderTypeMonth:
		return texts.PromptMonth
	case ReminderTypeYear:
		return texts.PromptYear
	case ReminderTypeDate:
		return texts.PromptDate
	case ReminderTypeMultiDay:
		return texts.PromptMultiDay
	default:
		return texts.PromptUnknown
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

// HandleAddTypeCallback обрабатывает выбор типа напоминания пользователем.
func (h *Handler) HandleAddTypeCallback(c tele.Context, typ string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	sess.Type = typ
	if typ == ReminderTypeWeek {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send(texts.PromptWeek, WeekdaysMenu())
	}
	if typ == ReminderTypeMonth {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send(texts.ValidateEnterMonth)
	}
	if typ == ReminderTypeYear {
		sess.Step = session.StepInterval
		h.updateSession(sess)
		return c.Send(texts.ValidateEnterDateDDMM)
	}
	if typ == ReminderTypeNDays {
		sess.Step = session.StepDate
		h.updateSession(sess)
		return c.Send(texts.ValidateEnterDate)
	}
	sess.Step = session.StepTime
	h.updateSession(sess)
	msg := getAddReminderMessage(typ)
	return c.Send(msg)
}

// handleStepInterval обрабатывает шаг выбора интервала (день недели, месяц, дата и т.д.).
func (h *Handler) handleStepInterval(c tele.Context, sess *session.AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	slog.Info(
		"[handleStepInterval]",
		"step", sess.Step,
		"type", sess.Type,
		"val", val,
		"date", sess.Date,
		"interval", sess.Interval,
	)
	switch sess.Type {
	case ReminderTypeWeek:
		weekday, ok := parseWeekday(val)
		if !ok {
			return c.Send(texts.ValidateEnterWeekday)
		}
		sess.Interval = weekday
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_weekday", weekday, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}
		return c.Send(texts.PromptEveryDay)
	case ReminderTypeMonth:
		if !validator.IsInterval(val) {
			return c.Send(texts.ValidateEnterMonth)
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_month", n, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}
		return c.Send(texts.PromptEveryDay)
	case ReminderTypeYear:
		if !validator.IsDateDDMM(val) {
			return c.Send(texts.ValidateEnterDateDDMM)
		}
		sess.Date = val
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_year_date", val, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}
		return c.Send(texts.PromptEveryDay)
	case ReminderTypeNDays:
		if !validator.IsInterval(val) {
			return c.Send(texts.ValidateEnterInterval)
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = session.StepTime
		h.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_ndays_interval", n, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}
		return c.Send(texts.PromptEveryDay)
	}
	return nil
}

// handleStepDate обрабатывает шаг выбора даты для напоминания с интервалом в N дней.
func (h *Handler) handleStepDate(c tele.Context, sess *session.AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	slog.Info(
		"[handleStepDate]",
		"step", sess.Step,
		"type", sess.Type,
		"val", val,
		"date", sess.Date,
		"interval", sess.Interval,
	)
	if sess.Type == ReminderTypeNDays {
		if sess.Date != "" && sess.Interval == 0 {
			if !validator.IsInterval(val) {
				return c.Send(texts.ValidateEnterInterval)
			}
			n, _ := strconv.Atoi(val)
			sess.Interval = n
			sess.Step = session.StepTime
			h.updateSession(sess)
			slog.Info("[handleStepDate]", "set_interval", n, "next_step", "StepTime")
			return c.Send(texts.PromptEveryDay)
		}
		if !validator.IsDateDDMMYYYY(val) {
			return c.Send(texts.ValidateEnterDate)
		}
		sess.Date = val
		sess.Step = session.StepInterval
		h.updateSession(sess)
		slog.Info("[handleStepDate]", "set_date", val, "next_step", "StepInterval")
		return c.Send(texts.ValidateEnterInterval)
	}
	return nil
}

func (h *Handler) handleStepTime(c tele.Context, sess *session.AddReminderSession) error {
	t := strings.TrimSpace(c.Text())
	if !validator.IsTime(t) && sess.Type != ReminderTypeMultiDay {
		return c.Send(texts.ValidateEnterTime)
	}
	sess.Time = t
	sess.Step = session.StepText
	h.updateSession(sess)
	return c.Send(texts.ValidateEnterText)
}

// handleStepText обрабатывает шаг ввода текста напоминания.
func (h *Handler) handleStepText(c tele.Context, sess *session.AddReminderSession) error {
	txt := strings.TrimSpace(c.Text())
	if !validator.IsNotEmpty(txt) {
		return c.Send(texts.ValidateEnterText)
	}
	sess.Text = txt
	sess.Step = session.StepConfirm
	h.updateSession(sess)
	return h.handleStepConfirm(c, sess)
}

// handleStepConfirm завершает создание напоминания и очищает сессию.
func (h *Handler) handleStepConfirm(c tele.Context, sess *session.AddReminderSession) error {
	err := h.createReminderFromSession(sess)
	h.Session.Delete(sess.ChatID, sess.UserID)
	if err != nil {
		return c.Send(texts.ErrCreateReminder)
	}
	return c.Send("Напоминание создано!")
}

// HandleAddWizardText обрабатывает текстовые шаги мастера добавления напоминания.
func (h *Handler) HandleAddWizardText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess == nil {
		return nil
	}
	slog.Info(
		"[HandleAddWizardText]",
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

// HandleWeekdayCallback обрабатывает inline-кнопки для дней недели.
func (h *Handler) HandleWeekdayCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	slog.Info("HandleWeekdayCallback", "callback_data", data)
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess != nil {
		slog.Info("Session state", "type", sess.Type, "step", sess.Step)
	}
	if err := c.Respond(); err != nil {
		slog.Error("c.Respond error", "err", err)
	}
	if sess == nil || sess.Type != ReminderTypeWeek || sess.Step != session.StepInterval {
		return c.Send(texts.ErrUnknownDay)
	}
	if !strings.HasPrefix(data, "weekday_") {
		return c.Send(texts.ErrUnknownDay)
	}
	weekdayStr := strings.TrimSpace(strings.TrimPrefix(data, "weekday_"))
	weekday, err := strconv.Atoi(weekdayStr)
	if err != nil || weekday < 0 || weekday > 6 {
		return c.Send(texts.ErrUnknownDay)
	}
	sess.Interval = weekday
	sess.Step = session.StepTime
	h.updateSession(sess)
	return c.Send(texts.PromptEveryDay)
}

// HandleMonthCallback обрабатывает inline-кнопки для месяцев.
func (h *Handler) HandleMonthCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	slog.Info("HandleMonthCallback", "callback_data", data)
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if err := c.Respond(); err != nil {
		slog.Error("c.Respond error", "err", err)
	}
	if sess == nil || sess.Type != ReminderTypeMonth || sess.Step != session.StepInterval {
		return c.Send(texts.ErrUnknownMonth)
	}
	if !strings.HasPrefix(data, "month_") {
		return c.Send(texts.ErrUnknownMonth)
	}
	monthStr := strings.TrimSpace(strings.TrimPrefix(data, "month_"))
	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		return c.Send(texts.ErrUnknownMonth)
	}
	sess.Interval = month
	sess.Step = session.StepText
	h.updateSession(sess)
	return c.Send(texts.ValidateEnterText)
}
