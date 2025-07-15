package handler

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/pkg/validator"
	tele "gopkg.in/telebot.v4"
)

func getAddReminderMessage(typ string) string {
	switch typ {
	case "today":
		return "Во сколько напомнить сегодня? (например, 15:00)"
	case "tomorrow":
		return "Во сколько напомнить завтра? (например, 15:00)"
	case "everyday":
		return "Во сколько напоминать каждый день? (например, 09:00)"
	case "week":
		return "В какой день недели? (например: понедельник)"
	case "ndays":
		return "Через сколько дней повторять? Например: каждые 10 дней в 12:00"
	case "month":
		return "В какой день месяца и во сколько? Например: 5 число 12:00"
	case "year":
		return "В какую дату и во сколько? Например: 13 июня 15:00"
	case "date":
		return "Введите дату и время: например, 13.06.2025 15:00"
	case "multiday":
		return "Во сколько? (например: 09:00, 14:00, 20:00)"
	default:
		return "Неизвестный тип напоминания"
	}
}

func (h *Handler) getSession(chatID, userID int64) *AddReminderSession {
	sess := h.Session.Get(chatID, userID)
	if sess == nil {
		sess = &AddReminderSession{UserID: userID, ChatID: chatID, Step: StepType}
	}
	return sess
}

func (h *Handler) updateSession(sess *AddReminderSession) {
	h.Session.Set(sess)
}

func (h *Handler) HandleAddTypeCallback(c tele.Context, typ string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	sess.Type = typ
	if typ == "week" {
		sess.Step = StepInterval
		h.updateSession(sess)
		return c.Send("Выберите день недели:", WeekdaysMenu())
	}
	sess.Step = StepTime
	h.updateSession(sess)
	msg := getAddReminderMessage(typ)
	return c.Send(msg)
}

func (h *Handler) handleStepTime(c tele.Context, sess *AddReminderSession) error {
	t := strings.TrimSpace(c.Text())
	if !validator.IsTime(t) && sess.Type != "multiday" {
		return c.Send("Пожалуйста, введите время в формате 15:00")
	}
	sess.Time = t
	sess.Step = StepText
	h.updateSession(sess)
	return c.Send("Введите текст напоминания:")
}

func (h *Handler) handleStepInterval(c tele.Context, sess *AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	switch sess.Type {
	case "week":
		weekday, ok := parseWeekday(val)
		if !ok {
			return c.Send("Пожалуйста, введите день недели (например, понедельник)")
		}
		sess.Interval = weekday
		sess.Step = StepTime
		h.updateSession(sess)
		return c.Send("Во сколько? (например: 09:00)")
	case "month":
		// Число месяца
		if !validator.IsInterval(val) {
			return c.Send("Пожалуйста, введите число месяца от 1 до 31")
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = StepText
		h.updateSession(sess)
		return c.Send("Введите текст напоминания:")
	case "year":
		// Дата в формате ДД.ММ
		if !validator.IsDateDDMM(val) {
			return c.Send("Пожалуйста, введите дату в формате ДД.ММ (например, 13.06)")
		}
		sess.Date = val
		sess.Step = StepText
		h.updateSession(sess)
		return c.Send("Введите текст напоминания:")
	case "ndays":
		// Интервал в днях
		if !validator.IsInterval(val) {
			return c.Send("Пожалуйста, введите интервал в днях (целое число > 0)")
		}
		n, _ := strconv.Atoi(val)
		sess.Interval = n
		sess.Step = StepText
		h.updateSession(sess)
		return c.Send("Введите текст напоминания:")
	case "multiday":
		times := parseMultiTimes(val)
		if len(times) == 0 {
			return c.Send("Пожалуйста, введите хотя бы одно время через запятую (например, 09:00, 14:00)")
		}
		sess.Time = strings.Join(times, ",")
		sess.Step = StepText
		h.updateSession(sess)
		return c.Send("Введите текст напоминания:")
	}
	return nil
}

func (h *Handler) handleStepText(c tele.Context, sess *AddReminderSession) error {
	txt := strings.TrimSpace(c.Text())
	if !validator.IsNotEmpty(txt) {
		return c.Send("Пожалуйста, введите текст напоминания")
	}
	sess.Text = txt
	sess.Step = StepConfirm
	h.updateSession(sess)
	return h.handleStepConfirm(c, sess)
}

func (h *Handler) handleStepConfirm(c tele.Context, sess *AddReminderSession) error {
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
	switch sess.Step {
	case StepTime:
		return h.handleStepTime(c, sess)
	case StepInterval:
		return h.handleStepInterval(c, sess)
	case StepText:
		return h.handleStepText(c, sess)
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
func convertSessionToReminder(sess *AddReminderSession, nextTime time.Time) *domain.Reminder {
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
	case "everyday":
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

func parseMultiTimes(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if matched, _ := regexp.MatchString(`^\d{2}:\d{2}$`, p); matched {
			res = append(res, p)
		}
	}
	return res
}

func (h *Handler) createReminderFromSession(sess *AddReminderSession) error {
	now := time.Now()
	var nextTime time.Time
	t, _ := time.Parse("15:04", sess.Time)
	switch sess.Type {
	case "today":
		nextTime = getNextTimeToday(now, t)
	case "tomorrow":
		nextTime = getNextTimeTomorrow(now, t)
	case "everyday":
		nextTime = getNextTimeEveryDay(now, t)
	case "week":
		nextTime = getNextTimeWeek(now, t, sess.Interval)
	case "month":
		nextTime = getNextTimeMonth(now, t, sess.Interval)
	case "year":
		nextTime = getNextTimeYear(now, t, sess.Date)
	case "date":
		nextTime = getNextTimeDate(t, sess.Date)
	case "ndays":
		nextTime = getNextTimeNDays(now, t, sess.Interval)
	case "multiday":
		times := strings.Split(sess.Time, ",")
		nextTime = getNextTimeMultiDay(now, times)
	}

	rem := convertSessionToReminder(sess, nextTime)
	// Для week/month/year/date/multiday сохраняем доп. параметры
	if sess.Type == "week" {
		rem.Repeat = domain.RepeatEveryWeek
		rem.RepeatDays = []int{sess.Interval}
	} else if sess.Type == "month" {
		rem.Repeat = domain.RepeatEveryMonth
		rem.RepeatDays = []int{sess.Interval}
	} else if sess.Type == "year" {
		rem.Repeat = domain.RepeatNone // раз в год — можно расширить
	} else if sess.Type == "date" {
		rem.Repeat = domain.RepeatNone
	} else if sess.Type == "ndays" {
		rem.Repeat = domain.RepeatEveryNDays
		rem.RepeatEvery = sess.Interval
	} else if sess.Type == "multiday" {
		rem.Repeat = domain.RepeatEveryDay
		// RepeatDays можно использовать для хранения времён, если нужно
	}

	return h.Usecase.AddReminder(context.Background(), rem)
}

// Добавить обработчик inline-кнопок для дней недели
func (h *Handler) HandleWeekdayCallback(c tele.Context) error {
	c.Respond()
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.getSession(chatID, userID)
	if sess == nil || sess.Type != "week" || sess.Step != StepInterval {
		return c.Send("Ошибка: не ожидается выбор дня недели.")
	}
	data := c.Callback().Data
	if !strings.HasPrefix(data, "weekday_") {
		return c.Send("Ошибка: неверный формат дня недели.")
	}
	weekdayStr := strings.TrimPrefix(data, "weekday_")
	weekday, err := strconv.Atoi(weekdayStr)
	if err != nil || weekday < 0 || weekday > 6 {
		return c.Send("Ошибка: неверный день недели.")
	}
	sess.Interval = weekday
	sess.Step = StepTime
	h.updateSession(sess)
	return c.Send("Во сколько? (например: 09:00)")
}
