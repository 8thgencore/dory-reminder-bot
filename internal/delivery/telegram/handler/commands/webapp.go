package commands

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	tele "gopkg.in/telebot.v4"
)

// WebAppCommands открывает пользователю Mini App.
type WebAppCommands struct {
	cfg     config.WebAppConfig
	botName string
}

// NewWebAppCommands создает обработчик команды открытия Mini App.
func NewWebAppCommands(cfg config.WebAppConfig, botName string) *WebAppCommands {
	return &WebAppCommands{cfg: cfg, botName: botName}
}

// Enabled сообщает, есть ли смысл предлагать пользователю Mini App.
func (c *WebAppCommands) Enabled() bool {
	return c.cfg.Enabled && c.cfg.PublicURL != ""
}

// OnApp обрабатывает команду /app.
func (c *WebAppCommands) OnApp(ctx tele.Context) error {
	if !c.Enabled() {
		return ctx.Send(texts.WebAppUnavailable)
	}

	if ctx.Chat().Type == tele.ChatPrivate {
		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(markup.WebApp(texts.WebAppButton, &tele.WebApp{URL: c.cfg.PublicURL})))

		return ctx.Send(texts.WebAppOpenPrivate, markup)
	}

	return c.sendGroupEntry(ctx)
}

// sendGroupEntry отправляет ссылку на Mini App для группового чата.
//
// Кнопки типа web_app Telegram разрешает только в личных чатах, поэтому из группы
// приложение открывается ссылкой на direct-link Mini App. Идентификатор чата уходит
// в startapp — сервер использует его как подсказку и всё равно проверяет права.
func (c *WebAppCommands) sendGroupEntry(ctx tele.Context) error {
	if c.cfg.ShortName == "" {
		return ctx.Send(texts.WebAppUsePrivate)
	}

	link := fmt.Sprintf("https://t.me/%s/%s?startapp=chat_%s",
		url.PathEscape(c.botName),
		url.PathEscape(c.cfg.ShortName),
		strconv.FormatInt(ctx.Chat().ID, 10),
	)

	markup := &tele.ReplyMarkup{}
	markup.Inline(markup.Row(markup.URL(texts.WebAppButton, link)))

	return ctx.Send(texts.WebAppOpenGroup, markup)
}

// SetupMenuButton делает кнопку меню бота ярлыком Mini App во всех личных чатах.
func (c *WebAppCommands) SetupMenuButton(bot *tele.Bot, log *slog.Logger) {
	if !c.Enabled() {
		return
	}

	// nil в качестве чата означает настройку по умолчанию для всех личных чатов.
	err := bot.SetMenuButton(nil, &tele.MenuButton{
		Type:   tele.MenuButtonWebApp,
		Text:   "Напоминания",
		WebApp: &tele.WebApp{URL: c.cfg.PublicURL},
	})
	if err != nil {
		log.Error("Failed to set Mini App menu button", "error", err)
		return
	}

	log.Info("Mini App menu button configured", "url", c.cfg.PublicURL)
}
