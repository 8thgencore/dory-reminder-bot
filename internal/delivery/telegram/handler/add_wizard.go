package handler

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	tele "gopkg.in/telebot.v4"
)

func (h *Handler) HandleAddTypeCallback(c tele.Context, typ string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.Session.Get(chatID, userID)
	if sess == nil {
		sess = &AddReminderSession{UserID: userID, ChatID: chatID, Step: StepType}
	}
	sess.Type = typ
	sess.Step = StepTime
	h.Session.Set(sess)

	switch typ {
	case "today":
		return c.Send("Во сколько напомнить сегодня? (например, 15:00)")
	case "tomorrow":
		return c.Send("Во сколько напомнить завтра? (например, 15:00)")
	case "everyday":
		return c.Send("Во сколько напоминать каждый день? (например, 09:00)")
	case "week":
		return c.Send("В какой день недели и во сколько? Например: понедельник 10:00")
	case "ndays":
		return c.Send("Через сколько дней повторять? Например: каждые 10 дней в 12:00")
	case "month":
		return c.Send("В какой день месяца и во сколько? Например: 5 число 12:00")
	case "year":
		return c.Send("В какую дату и во сколько? Например: 13 июня 15:00")
	case "date":
		return c.Send("Введите дату и время: например, 13.06.2025 15:00")
	case "multiday":
		return c.Send("Сколько раз в день и во сколько? Например: 3 раза: 09:00, 14:00, 20:00")
	default:
		return c.Send("Неизвестный тип напоминания")
	}
}

func (h *Handler) HandleAddWizardText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := h.Session.Get(chatID, userID)
	if sess == nil {
		return nil
	}
	if sess.Step == StepTime {
		t := strings.TrimSpace(c.Text())
		if t == "" {
			return c.Send("Пожалуйста, введите время в формате 15:00")
		}
		sess.Time = t
		// Для некоторых типов нужен ещё один шаг (день недели, дата, интервал)
		switch sess.Type {
		case "week":
			sess.Step = StepInterval
			h.Session.Set(sess)
			return c.Send("Введите день недели (например, понедельник):")
		case "month":
			sess.Step = StepInterval
			h.Session.Set(sess)
			return c.Send("Введите число месяца (1-31):")
		case "year":
			sess.Step = StepDate
			h.Session.Set(sess)
			return c.Send("Введите дату в формате ДД.ММ (например, 13.06):")
		case "ndays":
			sess.Step = StepInterval
			h.Session.Set(sess)
			return c.Send("Введите интервал в днях (например, 10):")
		case "multiday":
			sess.Step = StepInterval
			h.Session.Set(sess)
			return c.Send("Введите время через запятую (например, 09:00, 14:00, 20:00):")
		default:
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		}
	}
	if sess.Step == StepInterval {
		val := strings.TrimSpace(c.Text())
		switch sess.Type {
		case "week":
			// День недели
			weekday, ok := parseWeekday(val)
			if !ok {
				return c.Send("Пожалуйста, введите день недели (например, понедельник)")
			}
			sess.Interval = weekday
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		case "month":
			// Число месяца
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 31 {
				return c.Send("Пожалуйста, введите число месяца от 1 до 31")
			}
			sess.Interval = n
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		case "year":
			// Дата в формате ДД.ММ
			if !regexp.MustCompile(`^\d{2}\.\d{2}$`).MatchString(val) {
				return c.Send("Пожалуйста, введите дату в формате ДД.ММ (например, 13.06)")
			}
			sess.Date = val
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		case "ndays":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return c.Send("Пожалуйста, введите интервал в днях (целое число > 0)")
			}
			sess.Interval = n
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		case "multiday":
			times := parseMultiTimes(val)
			if len(times) == 0 {
				return c.Send("Пожалуйста, введите хотя бы одно время через запятую (например, 09:00, 14:00)")
			}
			sess.Time = strings.Join(times, ",")
			sess.Step = StepText
			h.Session.Set(sess)
			return c.Send("Введите текст напоминания:")
		}
	}
	if sess.Step == StepText {
		txt := strings.TrimSpace(c.Text())
		if txt == "" {
			return c.Send("Пожалуйста, введите текст напоминания")
		}
		sess.Text = txt
		sess.Step = StepConfirm
		h.Session.Set(sess)
		// Создаём напоминание
		err := h.createReminderFromSession(sess)
		h.Session.Delete(chatID, userID)
		if err != nil {
			return c.Send("Ошибка при создании напоминания")
		}
		return c.Send("Напоминание создано!")
	}
	return nil
}

func (h *Handler) createReminderFromSession(sess *AddReminderSession) error {
	now := time.Now()
	var nextTime time.Time
	switch sess.Type {
	case "today":
		t, _ := time.Parse("15:04", sess.Time)
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if nextTime.Before(now) {
			nextTime = nextTime.Add(24 * time.Hour)
		}
	case "tomorrow":
		t, _ := time.Parse("15:04", sess.Time)
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).Add(24 * time.Hour)
	case "everyday":
		t, _ := time.Parse("15:04", sess.Time)
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if nextTime.Before(now) {
			nextTime = nextTime.Add(24 * time.Hour)
		}
	case "week":
		t, _ := time.Parse("15:04", sess.Time)
		days := (sess.Interval - int(now.Weekday()) + 7) % 7
		if days == 0 && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute())) {
			days = 7
		}
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).AddDate(0, 0, days)
	case "month":
		t, _ := time.Parse("15:04", sess.Time)
		month := now.Month()
		year := now.Year()
		if now.Day() > sess.Interval || (now.Day() == sess.Interval && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))) {
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
		nextTime = time.Date(year, month, sess.Interval, t.Hour(), t.Minute(), 0, 0, now.Location())
	case "year":
		t, _ := time.Parse("15:04", sess.Time)
		day, _ := strconv.Atoi(strings.Split(sess.Date, ".")[0])
		mon, _ := strconv.Atoi(strings.Split(sess.Date, ".")[1])
		year := now.Year()
		if now.Month() > time.Month(mon) || (now.Month() == time.Month(mon) && (now.Day() > day || (now.Day() == day && (now.Hour() > t.Hour() || (now.Hour() == t.Hour() && now.Minute() >= t.Minute()))))) {
			year++
		}
		nextTime = time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, now.Location())
	case "date":
		t, _ := time.Parse("15:04", sess.Time)
		parts := strings.Split(sess.Date, ".")
		if len(parts) == 3 {
			day, _ := strconv.Atoi(parts[0])
			mon, _ := strconv.Atoi(parts[1])
			year, _ := strconv.Atoi(parts[2])
			nextTime = time.Date(year, time.Month(mon), day, t.Hour(), t.Minute(), 0, 0, now.Location())
		}
	case "ndays":
		t, _ := time.Parse("15:04", sess.Time)
		nextTime = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if nextTime.Before(now) {
			nextTime = nextTime.Add(time.Duration(sess.Interval) * 24 * time.Hour)
		}
	case "multiday":
		times := strings.Split(sess.Time, ",")
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
	}

	rem := AddReminderSessionToReminder(sess, nextTime)
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

func AddReminderSessionToReminder(sess *AddReminderSession, nextTime time.Time) *domain.Reminder {
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
