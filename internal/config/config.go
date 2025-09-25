package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Env type for environment
type Env string

const (
	// Dev is the development environment
	Dev Env = "dev"
	// Prod is the production environment
	Prod Env = "prod"
)

// Config is the configuration for the application
type Config struct {
	Env      Env `env:"ENV" env-default:"dev"`
	Telegram TelegramConfig
}

// TelegramConfig is the configuration for the engine
type TelegramConfig struct {
	Token   string `env:"TELEGRAM_TOKEN" env-required:"true"`
	BotName string `env:"BOT_NAME" env-default:"reminder_bot"`
}

// NewConfig creates a new instance of Config.
func NewConfig(envFile string) (*Config, error) {
	cfg := &Config{}

	var err error
	if envFile != "" {
		// Проверяем, существует ли файл
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("env file %s does not exist", envFile)
		}
		// Загружаем из файла
		err = cleanenv.ReadConfig(envFile, cfg)
	} else {
		// Загружаем из системных переменных окружения
		err = cleanenv.ReadEnv(cfg)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return cfg, nil
}
