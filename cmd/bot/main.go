package main

import (
	"log/slog"
	"os"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/commands"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler"
	"github.com/8thgencore/dory-reminder-bot/internal/infrastructure/database"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/8thgencore/dory-reminder-bot/pkg/logger"
)

func main() {
	// Load config file name from environment variable, if set
	configFile := os.Getenv("CONFIG_FILE")

	// If CONFIG_FILE is empty, configFile will be an empty string,
	// which is acceptable for config.NewConfig("")
	cfg, err := config.NewConfig(configFile)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Env)
	log.Info("Starting dory-reminder-bot", "env", cfg.Env)

	// Validate required config
	if cfg.Telegram.Token == "" {
		log.Error("TELEGRAM_TOKEN is required")
		os.Exit(1)
	}

	// Initialize bot
	pref := tele.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}

	// Устанавливаем команды бота для меню Telegram
	commands.SetCommands(bot, log)

	// Init DB
	db, err := database.InitDatabase(log)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer database.CloseDatabase(db, log)

	reminderRepo := repository.NewReminderRepository(db)
	chatRepo := repository.NewChatRepository(db)
	reminderUc := usecase.NewReminderUsecase(reminderRepo)
	chatUc := usecase.NewChatUsecase(chatRepo)
	handler := handler.NewHandler(bot, reminderUc, chatUc, cfg.Telegram.BotName)
	handler.Register()

	telegram.StartScheduler(bot, reminderUc, chatUc)

	log.Info("Bot started successfully")
	bot.Start()
}
