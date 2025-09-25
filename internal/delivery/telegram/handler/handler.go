package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/commands"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/ui"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/wizards"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// Handler представляет главный координатор для работы с напоминаниями через Telegram
type Handler struct {
	Bot        *tele.Bot
	SessionMgr *session.Manager
	BotName    string
	ChatUC     usecase.ChatUsecase

	// Компоненты
	BasicCommands     *commands.BasicCommands
	ReminderCRUD      *commands.ReminderCRUD
	AddReminderWizard *wizards.AddReminderWizard
	TimezoneWizard    *wizards.TimezoneWizard
}

// NewHandler создает новый Handler для работы с напоминаниями
func NewHandler(bot *tele.Bot, reminderUc usecase.ReminderUsecase,
	chatUc usecase.ChatUsecase,
	botName string,
) *Handler {
	sessionMgr := session.NewSessionManager()

	h := &Handler{
		Bot:               bot,
		SessionMgr:        sessionMgr,
		BotName:           botName,
		ChatUC:            chatUc,
		BasicCommands:     commands.NewBasicCommands(chatUc, ui.GetMainMenu),
		ReminderCRUD:      commands.NewReminderCRUD(reminderUc, chatUc),
		AddReminderWizard: wizards.NewAddReminderWizard(reminderUc, sessionMgr, chatUc, botName),
		TimezoneWizard:    wizards.NewTimezoneWizard(chatUc, sessionMgr, ui.GetMainMenu, botName),
	}

	return h
}

// Register регистрирует все обработчики команд и событий Telegram
func (h *Handler) Register() {
	// Базовые команды
	h.Bot.Handle("/start", h.BasicCommands.HandleStart)
	h.Bot.Handle("/help", h.BasicCommands.HandleHelp)

	// CRUD операции с напоминаниями
	h.Bot.Handle("/add", h.ReminderCRUD.OnAdd)
	h.Bot.Handle("/list", h.ReminderCRUD.OnList)
	h.Bot.Handle("/edit", h.ReminderCRUD.OnEdit)
	h.Bot.Handle("/delete", h.ReminderCRUD.OnDelete)
	h.Bot.Handle("/pause", h.ReminderCRUD.OnPause)
	h.Bot.Handle("/resume", h.ReminderCRUD.OnResume)

	// Настройка часового пояса
	h.Bot.Handle("/timezone", h.TimezoneWizard.OnTimezone)

	// Callback-обработчики для типов напоминаний
	h.Bot.Handle(ui.BtnToday, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "today")
	})
	h.Bot.Handle(ui.BtnTomorrow, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "tomorrow")
	})
	h.Bot.Handle(ui.BtnEveryDay, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "everyday")
	})
	h.Bot.Handle(ui.BtnWeek, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "week")
	})
	h.Bot.Handle(ui.BtnNDays, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "ndays")
	})
	h.Bot.Handle(ui.BtnMonth, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "month")
	})
	h.Bot.Handle(ui.BtnYear, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "year")
	})
	h.Bot.Handle(ui.BtnDate, func(c tele.Context) error {
		return h.AddReminderWizard.HandleAddTypeCallback(c, "date")
	})

	// Help menu handlers
	h.Bot.Handle(ui.BtnHelpAdd, h.cbHelpAdd)
	h.Bot.Handle(ui.BtnHelpList, h.cbHelpList)
	h.Bot.Handle(ui.BtnHelpManage, h.cbHelpManage)

	// Обработка текстовых сообщений
	h.Bot.Handle(tele.OnText, h.onText)

	// Обработка callback-запросов
	h.Bot.Handle(tele.OnCallback, h.onCallback)
}

// onText обрабатывает текстовые сообщения (мастер добавления/таймзона)
func (h *Handler) onText(c tele.Context) error {
	// Upsert chat on any text update
	if c.Chat() != nil {
		name := c.Chat().Title
		if name == "" && c.Chat().FirstName != "" {
			name = c.Chat().FirstName
		}
		_, _ = h.ChatUC.GetOrCreateChat(
			context.Background(),
			c.Chat().ID,
			string(c.Chat().Type),
			name,
			c.Chat().Username,
		) // best-effort
	}
	// Unified model: no user creation needed
	// Проверяем, является ли это ответом на сообщение бота или упоминанием бота
	isReply := c.Message().ReplyTo != nil
	isMention := strings.Contains(c.Text(), "@"+h.BotName)

	// В группах обрабатываем только ответы на сообщения бота или упоминания
	if c.Chat().Type != "private" && !isReply && !isMention {
		return nil
	}

	sess := h.SessionMgr.Get(c.Chat().ID, c.Sender().ID)
	if sess != nil && sess.Step == session.StepTimezone {
		return h.TimezoneWizard.HandleTimezoneText(c, h.BotName)
	}
	if sess != nil && (sess.Step == session.StepTime || sess.Step == session.StepText ||
		sess.Step == session.StepInterval || sess.Step == session.StepDate) {
		return h.AddReminderWizard.HandleAddWizardText(c, h.BotName)
	}

	return nil
}

// onCallback обрабатывает callback-запросы
func (h *Handler) onCallback(c tele.Context) error {
	callbackData := strings.TrimSpace(c.Callback().Data)

	if strings.HasPrefix(callbackData, "rem_page_") {
		return h.ReminderCRUD.OnList(c)
	}
	if strings.HasPrefix(callbackData, "weekday_") {
		return h.AddReminderWizard.HandleWeekdayCallback(c)
	}

	return nil
}

// Help menu callbacks (оставляем здесь для совместимости)
func (h *Handler) cbHelpAdd(c tele.Context) error {
	slog.Info("User requested help for adding reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)

	// Удаляем сообщение с кнопками
	if err := c.Delete(); err != nil {
		slog.Warn("Failed to delete help menu message", "error", err)
	}

	return h.ReminderCRUD.OnAdd(c)
}

func (h *Handler) cbHelpList(c tele.Context) error {
	slog.Info("User requested help for listing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)

	return h.ReminderCRUD.OnList(c)
}

func (h *Handler) cbHelpManage(c tele.Context) error {
	slog.Info("User requested help for managing reminders", "user_id", c.Sender().ID, "chat_id", c.Chat().ID)

	return h.BasicCommands.HandleHelp(c)
}
