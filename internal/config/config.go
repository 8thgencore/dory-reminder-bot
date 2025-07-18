package config

import (
	"fmt"

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
func NewConfig() (*Config, error) {
	cfg := &Config{}

	// Load environment variables
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to read env variables: %w", err)
	}

	return cfg, nil
}
