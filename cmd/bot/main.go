package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	// tzdata встраивает базу часовых поясов в бинарник: без неё time.LoadLocation
	// падает на минимальных образах, и напоминания молча уезжают в UTC.
	_ "time/tzdata"

	tele "gopkg.in/telebot.v4"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/commands"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp"
	"github.com/8thgencore/dory-reminder-bot/internal/infrastructure/database"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/8thgencore/dory-reminder-bot/pkg/logger"
)

// shutdownTimeout ограничивает время на корректное завершение HTTP-сервера.
const shutdownTimeout = 10 * time.Second

// Явный список не зависит от ранее сохранённого allowed_updates на стороне Telegram.
// Событие message также несёт служебное сообщение о миграции группы.
var botAllowedUpdates = []string{"message", "callback_query", "my_chat_member"}

func main() {
	if err := run(); err != nil {
		slog.Error("Fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	log := logger.New(cfg.Env)
	log.Info("Starting dory-reminder-bot", "env", cfg.Env, "webapp_enabled", cfg.WebApp.Enabled)

	bot, err := newBot(cfg)
	if err != nil {
		return err
	}
	commands.SetCommands(bot, log, cfg.WebApp.Enabled && cfg.WebApp.PublicURL != "")

	db, err := database.InitDatabase(cfg.Database, log)
	if err != nil {
		return err
	}
	defer database.CloseDatabase(db, log)

	reminderUc := usecase.NewReminderUsecase(repository.NewReminderRepository(db))
	chatUc := usecase.NewChatUsecase(repository.NewChatRepository(db))
	memberUc := usecase.NewMemberUsecase(repository.NewMemberRepository(db))

	h := handler.NewHandler(bot, reminderUc, chatUc, memberUc, cfg.WebApp)
	h.Register()
	h.WebAppCommands.SetupMenuButton(bot, log)

	// Контекст закрывается по SIGINT/SIGTERM и служит сигналом остановки для всех подсистем.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler := telegram.NewScheduler(bot, reminderUc, chatUc)
	go scheduler.Run(ctx)

	srv := startWebApp(ctx, cfg, bot, reminderUc, chatUc, memberUc, log)

	go func() {
		log.Info("Bot started successfully")
		bot.Start()
	}()

	<-ctx.Done()
	log.Info("Shutdown signal received")

	// bot.Start() блокируется на long polling, поэтому останавливаем его явно.
	bot.Stop()

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("Failed to shut down web app server", "error", err)
		} else {
			log.Info("Web app server stopped")
		}
	}

	return nil
}

// newBot собирает клиента Telegram, при необходимости заворачивая его в прокси.
func newBot(cfg *config.Config) (*tele.Bot, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	return tele.NewBot(tele.Settings{
		Token: cfg.Telegram.Token,
		Poller: &tele.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: botAllowedUpdates,
		},
		Client: client,
	})
}

// startWebApp поднимает HTTP-сервер Mini App. Возвращает nil, если WebApp выключен.
func startWebApp(
	ctx context.Context,
	cfg *config.Config,
	bot *tele.Bot,
	reminderUc usecase.ReminderUsecase,
	chatUc usecase.ChatUsecase,
	memberUc usecase.MemberUsecase,
	log *slog.Logger,
) *http.Server {
	if !cfg.WebApp.Enabled {
		log.Info("Web app is disabled, set WEBAPP_ENABLED=true to enable it")
		return nil
	}

	srv := webapp.NewServer(webapp.Deps{
		Config:     cfg.WebApp,
		Env:        cfg.Env,
		BotToken:   cfg.Telegram.Token,
		Bot:        bot,
		ReminderUC: reminderUc,
		ChatUC:     chatUc,
		MemberUC:   memberUc,
		Log:        log,
	})

	go func() {
		log.Info("Web app server listening", "addr", cfg.WebApp.Addr, "public_url", cfg.WebApp.PublicURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Web app server failed", "error", err)
			// Сервер упал — валим весь процесс, чтобы supervisor перезапустил его целиком.
			stopProcess(ctx)
		}
	}()

	return srv
}

// stopProcess шлёт процессу SIGTERM, чтобы сработал общий путь завершения из run().
func stopProcess(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(syscall.SIGTERM)
	}
}
