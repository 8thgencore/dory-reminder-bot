package wizards

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/timecalc"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/ui"
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
	SessionManager  *session.Manager
	TimeCalculator  *timecalc.TimeCalculator
	UserUsecase     usecase.UserUsecase // добавлено
}

// NewAddReminderWizard создает новый экземпляр мастера
// NewAddReminderWizard создает новый экземпляр мастера
func NewAddReminderWizard(reminderUc usecase.ReminderUsecase, sessionMgr *session.Manager,
	userUc usecase.UserUsecase,
) *AddReminderWizard {
	return &AddReminderWizard{
		ReminderUsecase: reminderUc,
		SessionManager:  sessionMgr,
		TimeCalculator:  timecalc.NewTimeCalculator(),
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
func (w *AddReminderWizard) HandleAddWizardText(c tele.Context, botName string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)
	if sess == nil {
		slog.Warn("[HandleAddWizardText] session is nil", "chatID", chatID, "userID", userID)
		return nil
	}

	// Убираем упоминание бота из текста, если оно есть
	text := c.Text()
	text = strings.ReplaceAll(text, "@"+botName, "")
	text = strings.TrimSpace(text)

	slog.Info(
		"[HandleAddWizardText] called",
		"chatID", chatID,
		"userID", userID,
		"chatType", c.Chat().Type,
		"step", sess.Step,
		"type", sess.Type,
		"text", text,
		"originalText", c.Text(),
		"isReply", c.Message().ReplyTo != nil,
		"isMention", strings.Contains(c.Text(), "@"+botName),
	)

	switch sess.Step {
	case session.StepTime:
		return w.handleStepTimeWithText(c, sess, text)
	case session.StepInterval:
		return w.handleStepIntervalWithText(c, sess, text)
	case session.StepText:
		return w.handleStepTextWithText(c, sess, text)
	case session.StepDate:
		return w.handleStepDateWithText(c, sess, text)
	}

	slog.Warn("[HandleAddWizardText] unknown step", "step", sess.Step, "type", sess.Type)

	return nil
}

// Добавляем вспомогательные методы для обработки текста без упоминания бота
func (w *AddReminderWizard) handleStepTimeWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	if !validator.IsTime(text) {
		return c.Send(texts.ValidateEnterTime)
	}
	sess.Time = text
	sess.Step = session.StepText
	w.updateSession(sess)

	return c.Send(texts.ValidateEnterText)
}

func (w *AddReminderWizard) handleStepIntervalWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Info(
		"[handleStepInterval]",
		"step", sess.Step,
		"type", sess.Type,
		"val", text,
		"date", sess.Date,
		"interval", sess.Interval,
	)

	switch sess.Type {
	case ReminderTypeWeek:
		weekday, ok := parseWeekday(text)
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
		if !validator.IsInterval(text) {
			return c.Send(texts.ValidateEnterMonth)
		}
		n, _ := strconv.Atoi(text)
		sess.Interval = n
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_month", n, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}

		return c.Send(texts.PromptEveryDay)
	case ReminderTypeYear:
		if !validator.IsDateDDMM(text) {
			return c.Send(texts.ValidateEnterDateDDMM)
		}
		sess.Date = text
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_year_date", text, "next_step", "StepTime")
		if err := c.Respond(); err != nil {
			slog.Error("c.Respond error", "err", err)
		}

		return c.Send(texts.PromptEveryDay)
	case ReminderTypeNDays:
		if !validator.IsInterval(text) {
			return c.Send(texts.ValidateEnterInterval)
		}
		n, _ := strconv.Atoi(text)
		sess.Interval = n
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Info("[handleStepInterval]", "set_ndays_interval", n, "next_step", "StepTime")

		return c.Send(texts.PromptEveryDay)
	}

	return nil
}

func (w *AddReminderWizard) handleStepTextWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Info("[handleStepTextWithText] called", "text", text, "chatID", sess.ChatID, "userID", sess.UserID)

	if !validator.IsNotEmpty(text) {
		slog.Warn("[handleStepTextWithText] empty text", "text", text)
		return c.Send(texts.ValidateEnterText)
	}
	sess.Text = text
	sess.Step = session.StepConfirm
	w.updateSession(sess)

	slog.Info("[handleStepTextWithText] text set, moving to confirm", "text", text, "step", sess.Step)

	return w.handleStepConfirm(c, sess)
}

func (w *AddReminderWizard) handleStepDateWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Info(
		"[handleStepDate] called",
		"step", sess.Step,
		"type", sess.Type,
		"val", text,
		"date", sess.Date,
		"interval", sess.Interval,
	)

	if sess.Type == ReminderTypeNDays {
		if sess.Date != "" && sess.Interval == 0 {
			if !validator.IsInterval(text) {
				slog.Warn("[handleStepDate] NDays: invalid interval", "val", text)
				return c.Send(texts.ValidateEnterInterval)
			}
			n, _ := strconv.Atoi(text)
			sess.Interval = n
			sess.Step = session.StepTime
			w.updateSession(sess)
			slog.Info("[handleStepDate] NDays: set_interval", "interval", n, "next_step", "StepTime")

			return c.Send(texts.PromptEveryDay)
		}
		if !validator.IsDateDDMMYYYY(text) {
			slog.Warn("[handleStepDate] NDays: invalid date", "val", text)
			return c.Send(texts.ValidateEnterDate)
		}
		sess.Date = text
		sess.Step = session.StepInterval
		w.updateSession(sess)
		slog.Info("[handleStepDate] NDays: set_date", "date", text, "next_step", "StepInterval")

		return c.Send(texts.ValidateEnterInterval)
	}

	// Новый вариант для ReminderTypeDate: дата и время одним сообщением
	if sess.Type == ReminderTypeDate {
		parts := strings.Fields(text)
		if len(parts) != 2 || !validator.IsDateDDMMYYYY(parts[0]) || !validator.IsTime(parts[1]) {
			slog.Warn("[handleStepDate] Date: invalid date/time", "val", text)
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

func (w *AddReminderWizard) handleStepConfirm(c tele.Context, sess *session.AddReminderSession) error {
	slog.Info("[handleStepConfirm] called", "session", sess)

	err := w.createReminderFromSession(sess)
	w.SessionManager.Delete(sess.ChatID, sess.UserID)
	if err != nil {
		slog.Error("[handleStepConfirm] failed to create reminder", "error", err, "session", sess)
		return c.Send(texts.ErrCreateReminder)
	}

	slog.Info("[handleStepConfirm] reminder created successfully")

	return c.Send("Напоминание создано!")
}

func (w *AddReminderWizard) createReminderFromSession(sess *session.AddReminderSession) error {
	now := time.Now()
	var nextTime time.Time

	slog.Info("[createReminderFromSession] before calculation", "type", sess.Type, "date", sess.Date,
		"time", sess.Time, "interval", sess.Interval, "chatID", sess.ChatID, "userID", sess.UserID)

	// Получаем пользователя и его таймзону
	user, err := w.UserUsecase.GetOrCreateUser(context.Background(), sess.ChatID, sess.UserID, "", "", "")
	if err != nil {
		slog.Error("[createReminderFromSession] failed to get/create user",
			"chatID", sess.ChatID, "userID", sess.UserID, "error", err)
		return err
	}

	if user != nil {
		slog.Info("[createReminderFromSession] user retrieved", "user", user, "timezone", user.Timezone)
	}

	loc := time.Local
	if user != nil && user.Timezone != "" {
		if l, err := time.LoadLocation(user.Timezone); err == nil {
			loc = l
			slog.Info("[createReminderFromSession] using timezone", "timezone", user.Timezone)
		} else {
			slog.Warn("[createReminderFromSession] failed to load timezone", "timezone", user.Timezone, "error", err)
		}
	}

	t, err := time.ParseInLocation("15:04", sess.Time, loc)
	if err != nil {
		slog.Warn("[createReminderFromSession] failed to parse time", "sess.Time", sess.Time, "err", err)
		return err
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
		nextTime = w.TimeCalculator.GetNextTimeDate(t, sess.Date, loc)
	case ReminderTypeNDays:
		startTime, err := time.ParseInLocation("02.01.2006", sess.Date, loc)
		if err != nil {
			slog.Warn("[createReminderFromSession] failed to parse date", "sess.Date", sess.Date, "err", err)
			return err
		}
		nextTime = w.TimeCalculator.GetNextTimeNDays(startTime, t, sess.Interval)
	}

	slog.Info("[createReminderFromSession] calculated nextTime", "nextTime", nextTime, "sess.Date", sess.Date,
		"sess.Time", sess.Time)

	rem := convertSessionToReminderWithTZ(sess, nextTime)

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

	err = w.ReminderUsecase.AddReminder(context.Background(), rem)
	if err != nil {
		slog.Error("[createReminderFromSession] failed to add reminder", "error", err, "reminder", rem)
		return err
	}

	slog.Info("[createReminderFromSession] reminder created successfully", "reminderID", rem.ID)

	return nil
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
func convertSessionToReminderWithTZ(sess *session.AddReminderSession, nextTime time.Time) *domain.Reminder {
	now := time.Now().UTC()
	return &domain.Reminder{
		ChatID:      sess.ChatID,
		UserID:      sess.UserID,
		Text:        sess.Text,
		NextTime:    nextTime.UTC(), // Конвертируем в UTC для хранения в БД
		Repeat:      typeToRepeat(sess.Type),
		RepeatEvery: sess.Interval,
		Paused:      false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
