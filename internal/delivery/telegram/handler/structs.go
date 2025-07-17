package handler

import (
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// Handler представляет обработчик для работы с напоминаниями через Telegram.
type Handler struct {
	Bot         *tele.Bot
	Usecase     usecase.ReminderUsecase
	UserUsecase usecase.UserUsecase
	Session     *session.SessionManager
}

// NewHandler создает новый Handler для работы с напоминаниями.
func NewHandler(bot *tele.Bot, uc usecase.ReminderUsecase, userUc usecase.UserUsecase) *Handler {
	return &Handler{Bot: bot, Usecase: uc, UserUsecase: userUc, Session: session.NewSessionManager()}
}

// Register регистрирует все обработчики команд и событий Telegram.
func (h *Handler) Register() {
	h.Bot.Handle("/start", func(c tele.Context) error {
		return h.HandleStart(c, h.UserUsecase)
	})
	h.Bot.Handle("/help", h.HandleHelp)
	h.Bot.Handle("/add", h.onAdd)
	h.Bot.Handle(&btnToday, h.cbAddToday)
	h.Bot.Handle(&btnTomorrow, h.cbAddTomorrow)
	h.Bot.Handle(&btnMultiDay, h.cbAddMultiDay)
	h.Bot.Handle(&btnEveryDay, h.cbAddEveryDay)
	h.Bot.Handle(&btnWeek, h.cbAddWeek)
	h.Bot.Handle(&btnNDays, h.cbAddNDays)
	h.Bot.Handle(&btnMonth, h.cbAddMonth)
	h.Bot.Handle(&btnYear, h.cbAddYear)
	h.Bot.Handle(&btnDate, h.cbAddDate)
	h.Bot.Handle("/list", h.onList)
	h.Bot.Handle("/edit", h.onEdit)
	h.Bot.Handle("/delete", h.onDelete)
	h.Bot.Handle("/pause", h.onPause)
	h.Bot.Handle("/resume", h.onResume)
	h.Bot.Handle("/timezone", h.onTimezone)
	h.Bot.Handle(tele.OnText, h.onText)
	h.Bot.Handle(tele.OnCallback, func(c tele.Context) error {
		slog.Info("OnCallback", "data", c.Callback().Data)
		callbackData := strings.TrimSpace(c.Callback().Data)
		if strings.HasPrefix(callbackData, "rem_page_") {
			return h.onList(c)
		}
		if strings.HasPrefix(callbackData, "weekday_") {
			return h.HandleWeekdayCallback(c)
		}
		if strings.HasPrefix(callbackData, "month_") {
			return h.HandleMonthCallback(c)
		}
		return nil
	})

	// Help menu handlers
	h.Bot.Handle(&btnHelpAdd, h.cbHelpAdd)
	h.Bot.Handle(&btnHelpList, h.cbHelpList)
	h.Bot.Handle(&btnHelpManage, h.cbHelpManage)
}
