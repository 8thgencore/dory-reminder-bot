package config

import (
	"errors"
	"fmt"
	"net/url"
	"time"

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
	Database DatabaseConfig
	WebApp   WebAppConfig
	// ProxyURL — необязательный исходящий прокси для Bot API (http, https или socks5).
	//
	// Тега env-required здесь быть не должно: cleanenv считает поле обязательным
	// по самому наличию тега, не глядя на значение, поэтому env-required:"false"
	// делал запуск без PROXY_URL невозможным.
	ProxyURL string `env:"PROXY_URL"`
}

// TelegramConfig is the configuration for the engine
type TelegramConfig struct {
	// Обязательность проверяется в Validate, а не тегом env-required: cleanenv
	// сообщил бы про поле "Token", тогда как настраивать нужно TELEGRAM_TOKEN.
	Token   string `env:"TELEGRAM_TOKEN"`
	BotName string `env:"BOT_NAME" env-default:"reminder_bot"`
}

// DatabaseConfig описывает расположение базы данных.
type DatabaseConfig struct {
	Path string `env:"DB_PATH" env-default:"data/reminders.db"`
}

// WebAppConfig описывает настройки Telegram Mini App.
type WebAppConfig struct {
	// Enabled включает HTTP-сервер Mini App.
	Enabled bool `env:"WEBAPP_ENABLED" env-default:"false"`
	// Addr — адрес, на котором слушает HTTP-сервер. TLS терминируется внешним reverse proxy.
	Addr string `env:"WEBAPP_ADDR" env-default:":8080"`
	// PublicURL — публичный HTTPS-адрес, который открывает Telegram.
	PublicURL string `env:"WEBAPP_PUBLIC_URL"`
	// ShortName — короткое имя direct-link Mini App из @BotFather (нужно для групп).
	ShortName string `env:"WEBAPP_SHORT_NAME"`
	// InitDataTTL — максимальный возраст initData, после которого он считается протухшим.
	InitDataTTL time.Duration `env:"WEBAPP_INITDATA_TTL" env-default:"24h"`
}

// NewConfig creates a new instance of Config from environment variables.
func NewConfig() (*Config, error) {
	cfg := &Config{}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate проверяет взаимозависимые параметры конфигурации.
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return errors.New("TELEGRAM_TOKEN is required")
	}

	if c.WebApp.Enabled && c.WebApp.PublicURL == "" {
		return errors.New("WEBAPP_PUBLIC_URL is required when WEBAPP_ENABLED=true: " +
			"Telegram opens Mini Apps only over a public HTTPS URL")
	}
	if c.WebApp.Enabled {
		publicURL, err := url.Parse(c.WebApp.PublicURL)
		if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" {
			return errors.New("WEBAPP_PUBLIC_URL must be a public HTTPS URL when WEBAPP_ENABLED=true")
		}
	}

	return nil
}
