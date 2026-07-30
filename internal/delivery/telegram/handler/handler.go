package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
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
	MemberUC   usecase.MemberUsecase

	// Компоненты
	BasicCommands     *commands.BasicCommands
	ReminderCRUD      *commands.ReminderCRUD
	WebAppCommands    *commands.WebAppCommands
	AddReminderWizard *wizards.AddReminderWizard
	TimezoneWizard    *wizards.TimezoneWizard
}

// NewHandler создает новый Handler для работы с напоминаниями
func NewHandler(bot *tele.Bot, reminderUc usecase.ReminderUsecase,
	chatUc usecase.ChatUsecase,
	memberUc usecase.MemberUsecase,
	webAppCfg config.WebAppConfig,
) *Handler {
	sessionMgr := session.NewSessionManager()
	botName := bot.Me.Username

	h := &Handler{
		Bot:               bot,
		SessionMgr:        sessionMgr,
		BotName:           botName,
		ChatUC:            chatUc,
		MemberUC:          memberUc,
		BasicCommands:     commands.NewBasicCommands(chatUc, ui.GetMainMenu),
		ReminderCRUD:      commands.NewReminderCRUD(reminderUc, chatUc),
		WebAppCommands:    commands.NewWebAppCommands(webAppCfg, botName),
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

	// Telegram Mini App
	h.Bot.Handle("/app", h.onApp)

	// Служебные события жизненного цикла группы и самого бота.
	h.Bot.Handle(tele.OnMigration, h.onMigration)
	h.Bot.Handle(tele.OnMyChatMember, h.onMyChatMember)

	// Callback-обработчики для типов напоминаний: все восемь кнопок ведут в один
	// обработчик и отличаются только типом.
	reminderTypeButtons := []struct {
		btn *tele.Btn
		typ string
	}{
		{ui.BtnToday, wizards.ReminderTypeToday},
		{ui.BtnTomorrow, wizards.ReminderTypeTomorrow},
		{ui.BtnEveryDay, wizards.ReminderTypeEveryDay},
		{ui.BtnWeek, wizards.ReminderTypeWeek},
		{ui.BtnNDays, wizards.ReminderTypeNDays},
		{ui.BtnMonth, wizards.ReminderTypeMonth},
		{ui.BtnYear, wizards.ReminderTypeYear},
		{ui.BtnDate, wizards.ReminderTypeDate},
	}
	for _, b := range reminderTypeButtons {
		h.Bot.Handle(b.btn, func(c tele.Context) error {
			return h.AddReminderWizard.HandleAddTypeCallback(c, b.typ)
		})
	}

	// Help menu handlers
	h.Bot.Handle(ui.BtnHelpAdd, h.cbHelpAdd)
	h.Bot.Handle(ui.BtnHelpList, h.cbHelpList)
	h.Bot.Handle(ui.BtnHelpManage, h.cbHelpManage)

	// Обработка текстовых сообщений
	h.Bot.Handle(tele.OnText, h.onText)

	// Обработка callback-запросов
	h.Bot.Handle(tele.OnCallback, h.onCallback)
}

// onApp запоминает чат и пользователя до выдачи ссылки. Иначе группа не попадёт
// в GET /api/v1/me, и Mini App откроет личные напоминания вместо групповых.
func (h *Handler) onApp(c tele.Context) error {
	if c.Chat() != nil && c.Sender() != nil {
		h.rememberChat(c)
		if c.Chat().Type != tele.ChatPrivate {
			if err := h.MemberUC.RememberWebAppLaunch(
				context.Background(),
				c.Chat().ID,
				c.Sender().ID,
			); err != nil {
				slog.Warn(
					"Failed to remember Mini App launch",
					"chat_id", c.Chat().ID,
					"user_id", c.Sender().ID,
					"error", err,
				)
			}
		}
	}

	return h.WebAppCommands.OnApp(c)
}

// onText обрабатывает текстовые сообщения (мастер добавления/таймзона)
func (h *Handler) onText(c tele.Context) error {
	chat, sender := c.Chat(), c.Sender()
	// У постов в канале и сообщений анонимных админов отправителя нет: без этой
	// проверки обращение к Sender().ID роняло бы обработчик.
	if chat == nil || sender == nil {
		return nil
	}

	h.rememberChat(c)

	// Проверяем, является ли это ответом на сообщение бота или упоминанием бота
	isReply := c.Message() != nil && c.Message().ReplyTo != nil
	isMention := strings.Contains(c.Text(), "@"+h.BotName)

	// В группах обрабатываем только ответы на сообщения бота или упоминания
	if chat.Type != tele.ChatPrivate && !isReply && !isMention {
		return nil
	}

	sess := h.SessionMgr.Get(chat.ID, sender.ID)
	if sess != nil && sess.Step == session.StepTimezone {
		return h.TimezoneWizard.HandleTimezoneText(c, h.BotName)
	}
	if sess != nil && (sess.Step == session.StepTime || sess.Step == session.StepText ||
		sess.Step == session.StepInterval || sess.Step == session.StepDate) {
		return h.AddReminderWizard.HandleAddWizardText(c, h.BotName)
	}

	return nil
}

// rememberChat фиксирует чат и присутствие в нём пользователя.
//
// Это единственный источник списка чатов для Mini App: бот узнаёт о группе только
// когда кто-то в ней ему пишет. Операция вспомогательная — её ошибки не должны
// мешать обработке сообщения.
func (h *Handler) rememberChat(c tele.Context) {
	chat, sender := c.Chat(), c.Sender()

	name := chat.Title
	if name == "" && chat.FirstName != "" {
		name = chat.FirstName
	}

	ctx := context.Background()
	if _, err := h.ChatUC.GetOrCreateChat(ctx, chat.ID, string(chat.Type), name, chat.Username); err != nil {
		slog.Warn("Failed to upsert chat", "chat_id", chat.ID, "error", err)
		return
	}
	if err := h.ChatUC.SetAvailable(ctx, chat.ID, true); err != nil {
		slog.Warn("Failed to activate chat", "chat_id", chat.ID, "error", err)
		return
	}

	if err := h.MemberUC.Remember(ctx, chat.ID, sender.ID); err != nil {
		slog.Warn("Failed to remember chat member", "chat_id", chat.ID, "user_id", sender.ID, "error", err)
	}
}

func (h *Handler) onMigration(c tele.Context) error {
	oldChatID, newChatID := c.Migration()
	if oldChatID == 0 || newChatID == 0 {
		return nil
	}

	if err := h.ChatUC.MigrateChat(context.Background(), oldChatID, newChatID); err != nil {
		slog.Error(
			"Failed to migrate Telegram chat",
			"old_chat_id", oldChatID,
			"new_chat_id", newChatID,
			"error", err,
		)
		return nil
	}

	slog.Info("Migrated Telegram chat", "old_chat_id", oldChatID, "new_chat_id", newChatID)

	return nil
}

func (h *Handler) onMyChatMember(c tele.Context) error {
	update, chat := c.ChatMember(), c.Chat()
	if update == nil || update.NewChatMember == nil || chat == nil {
		return nil
	}

	ctx := context.Background()
	role := update.NewChatMember.Role

	switch role {
	case tele.Creator, tele.Administrator, tele.Member, tele.Restricted:
		name := chat.Title
		if name == "" && chat.FirstName != "" {
			name = chat.FirstName
		}
		if _, err := h.ChatUC.GetOrCreateChat(
			ctx,
			chat.ID,
			string(chat.Type),
			name,
			chat.Username,
		); err != nil {
			slog.Error("Failed to reactivate Telegram chat", "chat_id", chat.ID, "error", err)
			return nil
		}
		if err := h.ChatUC.SetAvailable(ctx, chat.ID, true); err != nil {
			slog.Error("Failed to reactivate Telegram chat", "chat_id", chat.ID, "error", err)
			return nil
		}
		slog.Info("Telegram bot became available in chat", "chat_id", chat.ID, "status", role)

	case tele.Left, tele.Kicked:
		if err := h.ChatUC.SetAvailable(ctx, chat.ID, false); err != nil {
			slog.Error("Failed to freeze unavailable Telegram chat", "chat_id", chat.ID, "error", err)
			return nil
		}
		slog.Warn("Telegram bot became unavailable in chat", "chat_id", chat.ID, "status", role)
	}

	return nil
}

// onCallback обрабатывает callback-запросы
func (h *Handler) onCallback(c tele.Context) error {
	if c.Chat() != nil && c.Sender() != nil {
		h.rememberChat(c)
	}

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
