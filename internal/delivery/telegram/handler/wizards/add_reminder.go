package wizards

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/ui"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/utils"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
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
)

// AddReminderWizard обрабатывает мастер добавления напоминаний
type AddReminderWizard struct {
	ReminderUsecase usecase.ReminderUsecase
	SessionManager  *session.SessionManager
	TimeCalculator  *utils.TimeCalculator
	UserUsecase     usecase.UserUsecase // добавлено
}

// NewAddReminderWizard создает новый экземпляр мастера
func NewAddReminderWizard(reminderUc usecase.ReminderUsecase, sessionMgr *session.SessionManager, userUc usecase.UserUsecase) *AddReminderWizard {
	return &AddReminderWizard{
		ReminderUsecase: reminderUc,
		SessionManager:  sessionMgr,
		TimeCalculator:  utils.NewTimeCalculator(),
		UserUsecase:     userUc, // добавлено
	}
}

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
	default:
		return texts.PromptUnknown
	}
}

func (w *AddReminderWizard) getSession(chatID, userID int64) *session.AddReminderSession {
	sess := w.SessionManager.Get(chatID, userID)
	if sess == nil {
		sess = &session.AddReminderSession{UserID: userID, ChatID: chatID, Step: session.StepType}
	}
	return sess
}

func (w *AddReminderWizard) updateSession(sess *session.AddReminderSession) {
	w.SessionManager.Set(sess)
}

// HandleAddTypeCallback обрабатывает выбор типа напоминания пользователем
func (w *AddReminderWizard) HandleAddTypeCallback(c tele.Context, typ string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)
	sess.Type = typ

	if typ == ReminderTypeWeek {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		_ = c.Delete()
		return c.Send(texts.PromptWeek, ui.WeekdaysMenu())
	}
	if typ == ReminderTypeMonth {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		_ = c.Delete()
		return c.Send(texts.ValidateEnterMonth)
	}
	if typ == ReminderTypeYear {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		_ = c.Delete()
		return c.Send(texts.ValidateEnterDateDDMM)
	}
	if typ == ReminderTypeNDays {
		sess.Step = session.StepDate
		w.updateSession(sess)
		_ = c.Delete()
		return c.Send(texts.ValidateEnterDate)
	}
	if typ == ReminderTypeDate {
		sess.Step = session.StepDate
		w.updateSession(sess)
		_ = c.Delete()
		return c.Send("Пожалуйста, введите дату и время в формате ДД.ММ.ГГГГ ЧЧ:ММ")
	}

	sess.Step = session.StepTime
	w.updateSession(sess)
	_ = c.Delete()
	msg := getAddReminderMessage(typ)
	return c.Send(msg)
}

// HandleAddWizardText обрабатывает текстовые шаги мастера добавления напоминания
func (w *AddReminderWizard) HandleAddWizardText(c tele.Context) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)
	if sess == nil {
		slog.Warn("[HandleAddWizardText] session is nil", "chatID", chatID, "userID", userID)
		return nil
	}

	slog.Info(
		"[HandleAddWizardText] called",
		"chatID", chatID,
		"userID", userID,
		"step", sess.Step,
		"type", sess.Type,
		"text", c.Text(),
	)

	switch sess.Step {
	case session.StepTime:
		return w.handleStepTime(c, sess)
	case session.StepInterval:
		return w.handleStepInterval(c, sess)
	case session.StepText:
		return w.handleStepText(c, sess)
	case session.StepDate:
		return w.handleStepDate(c, sess)
	}

	slog.Warn("[HandleAddWizardText] unknown step", "step", sess.Step, "type", sess.Type)
	return nil
}

func (w *AddReminderWizard) handleStepTime(c tele.Context, sess *session.AddReminderSession) error {
	t := strings.TrimSpace(c.Text())
	if !validator.IsTime(t) {
		return c.Send(texts.ValidateEnterTime)
	}
	sess.Time = t
	sess.Step = session.StepText
	w.updateSession(sess)
	return c.Send(texts.ValidateEnterText)
}

func (w *AddReminderWizard) handleStepInterval(c tele.Context, sess *session.AddReminderSession) error {
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
		w.updateSession(sess)
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
		w.updateSession(sess)
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
		w.updateSession(sess)
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
		w.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_ndays_interval", n, "next_step", "StepTime")
		return c.Send(texts.PromptEveryDay)
	}
	return nil
}

func (w *AddReminderWizard) handleStepDate(c tele.Context, sess *session.AddReminderSession) error {
	val := strings.TrimSpace(c.Text())
	slog.Info(
		"[handleStepDate] called",
		"step", sess.Step,
		"type", sess.Type,
		"val", val,
		"date", sess.Date,
		"interval", sess.Interval,
	)

	if sess.Type == ReminderTypeNDays {
		if sess.Date != "" && sess.Interval == 0 {
			if !validator.IsInterval(val) {
				slog.Warn("[handleStepDate] NDays: invalid interval", "val", val)
				return c.Send(texts.ValidateEnterInterval)
			}
			n, _ := strconv.Atoi(val)
			sess.Interval = n
			sess.Step = session.StepTime
			w.updateSession(sess)
			slog.Info("[handleStepDate] NDays: set_interval", "interval", n, "next_step", "StepTime")
			return c.Send(texts.PromptEveryDay)
		}
		if !validator.IsDateDDMMYYYY(val) {
			slog.Warn("[handleStepDate] NDays: invalid date", "val", val)
			return c.Send(texts.ValidateEnterDate)
		}
		sess.Date = val
		sess.Step = session.StepInterval
		w.updateSession(sess)
		slog.Info("[handleStepDate] NDays: set_date", "date", val, "next_step", "StepInterval")
		return c.Send(texts.ValidateEnterInterval)
	}

	// Новый вариант для ReminderTypeDate: дата и время одним сообщением
	if sess.Type == ReminderTypeDate {
		parts := strings.Fields(val)
		if len(parts) != 2 || !validator.IsDateDDMMYYYY(parts[0]) || !validator.IsTime(parts[1]) {
			slog.Warn("[handleStepDate] Date: invalid date/time", "val", val)
			return c.Send("Пожалуйста, введите дату и время в формате ДД.ММ.ГГГГ ЧЧ:ММ")
		}
		sess.Date = parts[0]
		sess.Time = parts[1]
		sess.Step = session.StepText
		w.updateSession(sess)
		slog.Info("[handleStepDate] Date: set_date_time", "date", sess.Date, "time", sess.Time, "next_step", "StepText")
		return c.Send(texts.ValidateEnterText)
	}

	slog.Warn("[handleStepDate] unknown type", "type", sess.Type)
	return nil
}

func (w *AddReminderWizard) handleStepText(c tele.Context, sess *session.AddReminderSession) error {
	txt := strings.TrimSpace(c.Text())
	if !validator.IsNotEmpty(txt) {
		return c.Send(texts.ValidateEnterText)
	}
	sess.Text = txt
	sess.Step = session.StepConfirm
	w.updateSession(sess)
	return w.handleStepConfirm(c, sess)
}

func (w *AddReminderWizard) handleStepConfirm(c tele.Context, sess *session.AddReminderSession) error {
	err := w.createReminderFromSession(sess)
	w.SessionManager.Delete(sess.ChatID, sess.UserID)
	if err != nil {
		return c.Send(texts.ErrCreateReminder)
	}
	return c.Send("Напоминание создано!")
}

func (w *AddReminderWizard) createReminderFromSession(sess *session.AddReminderSession) error {
	now := time.Now()
	var nextTime time.Time

	slog.Info("[createReminderFromSession] before calculation", "type", sess.Type, "date", sess.Date, "time", sess.Time, "interval", sess.Interval)

	// Получаем пользователя и его таймзону
	user, err := w.UserUsecase.GetOrCreateUser(context.Background(), sess.ChatID, sess.UserID, "", "", "")
	loc := time.Local
	if err == nil && user != nil && user.Timezone != "" {
		if l, err := time.LoadLocation(user.Timezone); err == nil {
			loc = l
		}
	}

	t, err := time.ParseInLocation("15:04", sess.Time, loc)
	if err != nil {
		slog.Warn("[createReminderFromSession] failed to parse time", "sess.Time", sess.Time, "err", err)
	}

	switch sess.Type {
	case ReminderTypeToday:
		nextTime = w.TimeCalculator.GetNextTimeToday(now.In(loc), t)
	case ReminderTypeTomorrow:
		nextTime = w.TimeCalculator.GetNextTimeTomorrow(now.In(loc), t)
	case ReminderTypeEveryDay:
		nextTime = w.TimeCalculator.GetNextTimeEveryDay(now.In(loc), t)
	case ReminderTypeWeek:
		nextTime = w.TimeCalculator.GetNextTimeWeek(now.In(loc), t, sess.Interval)
	case ReminderTypeMonth:
		nextTime = w.TimeCalculator.GetNextTimeMonth(now.In(loc), t, sess.Interval)
	case ReminderTypeYear:
		nextTime = w.TimeCalculator.GetNextTimeYear(now.In(loc), t, sess.Date)
	case ReminderTypeDate:
		nextTime = w.TimeCalculator.GetNextTimeDate(t, sess.Date)
	case ReminderTypeNDays:
		startTime, err := time.ParseInLocation("02.01.2006", sess.Date, loc)
		if err != nil {
			slog.Warn("[createReminderFromSession] failed to parse date", "sess.Date", sess.Date, "err", err)
		}
		nextTime = w.TimeCalculator.GetNextTimeNDays(startTime, t, sess.Interval)
	}

	slog.Info("[createReminderFromSession] calculated nextTime", "nextTime", nextTime, "sess.Date", sess.Date, "sess.Time", sess.Time)

	rem := convertSessionToReminderWithTZ(sess, nextTime, user.Timezone)

	// Для week/month/year/date сохраняем доп. параметры
	switch sess.Type {
	case ReminderTypeWeek:
		rem.Repeat = domain.RepeatEveryWeek
		rem.RepeatDays = []int{sess.Interval}
	case ReminderTypeMonth:
		rem.Repeat = domain.RepeatEveryMonth
		rem.RepeatDays = []int{sess.Interval}
	case ReminderTypeYear:
		rem.Repeat = domain.RepeatEveryYear
	case ReminderTypeDate:
		rem.Repeat = domain.RepeatNone
	case ReminderTypeNDays:
		rem.Repeat = domain.RepeatEveryNDays
		rem.RepeatEvery = sess.Interval
	}

	slog.Info("[createReminderFromSession] final reminder", "reminder", rem)

	return w.ReminderUsecase.AddReminder(context.Background(), rem)
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
	weekdays := map[string]int{
		"воскресенье": 0, "понедельник": 1, "вторник": 2, "среда": 3,
		"четверг": 4, "пятница": 5, "суббота": 6,
	}
	s = strings.ToLower(strings.TrimSpace(s))
	idx, ok := weekdays[s]
	return idx, ok
}

// HandleWeekdayCallback обрабатывает inline-кнопки для дней недели
func (w *AddReminderWizard) HandleWeekdayCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	slog.Info("HandleWeekdayCallback", "callback_data", data)

	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)

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
	w.updateSession(sess)
	return c.Send(texts.PromptEveryDay)
}

// HandleMonthCallback обрабатывает inline-кнопки для месяцев
func (w *AddReminderWizard) HandleMonthCallback(c tele.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	slog.Info("HandleMonthCallback", "callback_data", data)

	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)

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
	w.updateSession(sess)
	return c.Send(texts.ValidateEnterText)
}

// convertSessionToReminderWithTZ converts an AddReminderSession to a domain Reminder с учетом таймзоны
func convertSessionToReminderWithTZ(sess *session.AddReminderSession, nextTime time.Time, tz string) *domain.Reminder {
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
		Timezone:    tz,
	}
}
