package wizards

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/ui"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/scheduling"
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

type reminderCreator interface {
	AddReminder(ctx context.Context, reminder *domain.Reminder) error
}

type chatLocationProvider interface {
	Location(ctx context.Context, chatID int64) *time.Location
}

// AddReminderWizard обрабатывает мастер добавления напоминаний
type AddReminderWizard struct {
	ReminderUsecase reminderCreator
	SessionManager  *session.Manager
	ChatUsecase     chatLocationProvider
	BotName         string
}

// NewAddReminderWizard создает новый экземпляр мастера
func NewAddReminderWizard(
	reminderUc reminderCreator,
	sessionMgr *session.Manager,
	chatUc chatLocationProvider,
	botName string,
) *AddReminderWizard {
	return &AddReminderWizard{
		ReminderUsecase: reminderUc,
		SessionManager:  sessionMgr,
		ChatUsecase:     chatUc,
		BotName:         botName,
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

	// Удаляем сообщение с кнопками
	if err := c.Delete(); err != nil {
		slog.Warn("Failed to delete message with buttons", "error", err)
	}

	if typ == ReminderTypeWeek {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		return c.Send(texts.PromptWeek, ui.WeekdaysMenu())
	}
	if typ == ReminderTypeMonth {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterMonth))
	}
	if typ == ReminderTypeYear {
		sess.Step = session.StepInterval
		w.updateSession(sess)
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDateDDMM))
	}
	if typ == ReminderTypeNDays {
		sess.Step = session.StepDate
		w.updateSession(sess)
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDate))
	}
	if typ == ReminderTypeDate {
		sess.Step = session.StepDate
		w.updateSession(sess)
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDateDDMMYYYY))
	}

	sess.Step = session.StepTime
	w.updateSession(sess)
	msg := getAddReminderMessage(typ)

	return c.Send(withGroupHint(c, w.BotName, msg))
}

// HandleAddWizardText обрабатывает текстовые шаги мастера добавления напоминания
func (w *AddReminderWizard) HandleAddWizardText(c tele.Context, botName string) error {
	userID := c.Sender().ID
	chatID := c.Chat().ID
	sess := w.getSession(chatID, userID)

	// Убираем упоминание бота из текста, если оно есть
	text := c.Text()
	text = strings.ReplaceAll(text, "@"+botName, "")
	text = strings.TrimSpace(text)

	// Текст сообщения — это содержимое напоминания пользователя, поэтому в журнал
	// он не попадает; для диагностики хватает шага мастера.
	slog.Debug(
		"[HandleAddWizardText] called",
		"chatID", chatID,
		"userID", userID,
		"chatType", c.Chat().Type,
		"step", sess.Step,
		"type", sess.Type,
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
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterTime))
	}
	sess.Time = text
	sess.Step = session.StepText
	w.updateSession(sess)

	return c.Send(texts.ValidateEnterText)
}

func (w *AddReminderWizard) handleStepIntervalWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Debug("[handleStepInterval]", "type", sess.Type, "date", sess.Date, "interval", sess.Interval)

	switch sess.Type {
	case ReminderTypeWeek:
		weekday, ok := parseWeekday(text)
		if !ok {
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterWeekday))
		}
		sess.Interval = weekday
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Debug("[handleStepInterval]", "set_weekday", weekday, "next_step", "StepTime")

		return c.Send(withGroupHint(c, w.BotName, texts.PromptEveryDay))
	case ReminderTypeMonth:
		n, ok := validator.ParseDayOfMonth(text)
		if !ok {
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterMonth))
		}
		sess.Interval = n
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Debug("[handleStepInterval]", "set_month", n, "next_step", "StepTime")

		return c.Send(withGroupHint(c, w.BotName, texts.PromptEveryDay))
	case ReminderTypeYear:
		if !validator.IsDateDDMM(text) {
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDateDDMM))
		}
		sess.Date = text
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Debug("[handleStepInterval]", "set_year_date", text, "next_step", "StepTime")

		return c.Send(withGroupHint(c, w.BotName, texts.PromptEveryDay))
	case ReminderTypeNDays:
		n, ok := validator.ParseInterval(text)
		if !ok {
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterInterval))
		}
		sess.Interval = n
		sess.Step = session.StepTime
		w.updateSession(sess)
		slog.Debug("[handleStepInterval]", "set_ndays_interval", n, "next_step", "StepTime")

		return c.Send(withGroupHint(c, w.BotName, texts.PromptEveryDay))
	}

	return nil
}

func (w *AddReminderWizard) handleStepTextWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Debug("[handleStepTextWithText] called", "chatID", sess.ChatID, "userID", sess.UserID)

	if text == "" {
		slog.Debug("[handleStepTextWithText] empty text", "chatID", sess.ChatID)
		return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterText))
	}
	sess.Text = text
	sess.Step = session.StepConfirm
	w.updateSession(sess)

	slog.Debug("[handleStepTextWithText] text set, moving to confirm", "step", sess.Step)

	return w.handleStepConfirm(c, sess)
}

func (w *AddReminderWizard) handleStepDateWithText(c tele.Context, sess *session.AddReminderSession,
	text string,
) error {
	slog.Debug("[handleStepDate] called", "type", sess.Type, "date", sess.Date, "interval", sess.Interval)

	if sess.Type == ReminderTypeNDays {
		if !validator.IsDateDDMMYYYY(text) {
			slog.Warn("[handleStepDate] NDays: invalid date", "val", text)
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDate))
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
			return c.Send(withGroupHint(c, w.BotName, texts.ValidateEnterDateDDMMYYYY))
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
	slog.Debug("[handleStepConfirm] called", "chatID", sess.ChatID, "type", sess.Type)

	err := w.createReminderFromSession(sess)
	w.SessionManager.Delete(sess.ChatID, sess.UserID)
	if err != nil {
		slog.Error("[handleStepConfirm] failed to create reminder", "error", err, "chatID", sess.ChatID)
		return c.Send(texts.ErrCreateReminder)
	}

	slog.Debug("[handleStepConfirm] reminder created successfully")

	return c.Send(texts.ReminderCreated)
}

func (w *AddReminderWizard) createReminderFromSession(sess *session.AddReminderSession) error {
	ctx := context.Background()
	now := time.Now()

	slog.Debug("[createReminderFromSession] before calculation", "type", sess.Type, "date", sess.Date,
		"time", sess.Time, "interval", sess.Interval, "chatID", sess.ChatID)

	loc := w.ChatUsecase.Location(ctx, sess.ChatID)

	t, err := time.ParseInLocation("15:04", sess.Time, loc)
	if err != nil {
		slog.Warn("[createReminderFromSession] failed to parse time", "sess.Time", sess.Time, "err", err)
		return err
	}

	nextTime, err := w.calcNextTime(sess, now.In(loc), t, loc)
	if err != nil {
		slog.Warn("[createReminderFromSession] failed to calculate next time", "type", sess.Type, "err", err)
		return err
	}

	rem := convertSessionToReminder(sess, nextTime)

	slog.Debug("[createReminderFromSession] final reminder",
		"chatID", rem.ChatID, "nextTime", rem.NextTime, "repeat", rem.Repeat)

	if err := w.ReminderUsecase.AddReminder(ctx, rem); err != nil {
		slog.Error("[createReminderFromSession] failed to add reminder", "error", err, "chatID", rem.ChatID)
		return err
	}

	slog.Info("[createReminderFromSession] reminder created", "reminderID", rem.ID, "chatID", rem.ChatID)

	return nil
}

// calcNextTime вычисляет первое срабатывание для выбранного пользователем типа напоминания.
func (w *AddReminderWizard) calcNextTime(
	sess *session.AddReminderSession,
	now, t time.Time,
	loc *time.Location,
) (time.Time, error) {
	switch sess.Type {
	case ReminderTypeToday:
		return scheduling.NextToday(now, t), nil
	case ReminderTypeTomorrow:
		return scheduling.NextTomorrow(now, t), nil
	case ReminderTypeEveryDay:
		return scheduling.NextToday(now, t), nil
	case ReminderTypeWeek:
		return scheduling.NextWeekday(now, t, sess.Interval)
	case ReminderTypeMonth:
		return scheduling.NextMonthDay(now, t, sess.Interval)
	case ReminderTypeYear:
		return scheduling.NextYearDay(now, t, sess.Date)
	case ReminderTypeDate:
		return scheduling.AtDate(t, sess.Date, loc)
	case ReminderTypeNDays:
		startTime, err := validator.ParseDateDDMMYYYY(sess.Date, loc)
		if err != nil {
			return time.Time{}, err
		}

		return scheduling.NextNDays(startTime, t, sess.Interval)
	}

	return time.Time{}, fmt.Errorf("unknown reminder type %q", sess.Type)
}

func parseWeekday(s string) (int, bool) {
	const (
		sunday    = "воскресенье"
		monday    = "понедельник"
		tuesday   = "вторник"
		wednesday = "среда"
		thursday  = "четверг"
		friday    = "пятница"
		saturday  = "суббота"
	)

	weekdays := map[string]int{
		sunday: 0, monday: 1, tuesday: 2, wednesday: 3,
		thursday: 4, friday: 5, saturday: 6,
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

	// Удаляем сообщение с кнопками дней недели
	if err := c.Delete(); err != nil {
		slog.Warn("Failed to delete weekday buttons message", "error", err)
	}

	return c.Send(texts.PromptEveryDay)
}

// convertSessionToReminder собирает доменное напоминание из состояния мастера.
//
// sess.Interval переиспользуется под разные смыслы в зависимости от типа: день недели,
// число месяца или интервал в днях. Раскладывать его нужно по соответствующим полям —
// раньше он безусловно попадал в RepeatEvery, засоряя интервал повтора днём недели.
func convertSessionToReminder(sess *session.AddReminderSession, nextTime time.Time) *domain.Reminder {
	now := time.Now().UTC()
	rem := &domain.Reminder{
		ChatID:    sess.ChatID,
		Text:      sess.Text,
		NextTime:  nextTime.UTC(), // Конвертируем в UTC для хранения в БД
		Paused:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	switch sess.Type {
	case ReminderTypeEveryDay:
		rem.Repeat = domain.RepeatEveryDay
	case ReminderTypeWeek:
		rem.Repeat = domain.RepeatEveryWeek
		rem.RepeatDays = []int{sess.Interval}
	case ReminderTypeMonth:
		rem.Repeat = domain.RepeatEveryMonth
		rem.RepeatDays = []int{sess.Interval}
	case ReminderTypeYear:
		rem.Repeat = domain.RepeatEveryYear
	case ReminderTypeNDays:
		rem.Repeat = domain.RepeatEveryNDays
		rem.RepeatEvery = sess.Interval
	case ReminderTypeToday, ReminderTypeTomorrow, ReminderTypeDate:
		rem.Repeat = domain.RepeatNone
	}

	return rem
}
