package handler

import (
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	Bot         *tele.Bot
	Usecase     usecase.ReminderUsecase
	UserUsecase usecase.UserUsecase
	Session     *SessionManager
}

func NewHandler(bot *tele.Bot, uc usecase.ReminderUsecase, userUc usecase.UserUsecase) *Handler {
	return &Handler{Bot: bot, Usecase: uc, UserUsecase: userUc, Session: NewSessionManager()}
}

func (h *Handler) Register() {
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
		if strings.HasPrefix(c.Callback().Data, "rem_page_") {
			return h.onList(c)
		}
		return nil
	})

	// Help menu handlers
	h.Bot.Handle(&btnHelpAdd, h.cbHelpAdd)
	h.Bot.Handle(&btnHelpList, h.cbHelpList)
	h.Bot.Handle(&btnHelpManage, h.cbHelpManage)
}
