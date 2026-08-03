// Package webapp отдаёт HTTP API и статику Telegram Mini App.
//
// Сервер слушает обычный HTTP: TLS и публичный домен обеспечивает внешний reverse proxy,
// адрес которого задаётся в WEBAPP_PUBLIC_URL.
package webapp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/auth"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/authz"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	tele "gopkg.in/telebot.v4"
)

// Таймауты HTTP-сервера: сервер смотрит в интернет через reverse proxy, и без них
// зависшее соединение держало бы горутину бесконечно.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// Deps — зависимости HTTP-слоя.
type Deps struct {
	Config     config.WebAppConfig
	Env        config.Env
	BotToken   string
	Bot        *tele.Bot
	ReminderUC usecase.ReminderUsecase
	ChatUC     usecase.ChatUsecase
	MemberUC   usecase.MemberUsecase
	Log        *slog.Logger
}

// server держит состояние HTTP-слоя между обработчиками.
type server struct {
	cfg        config.WebAppConfig
	env        config.Env
	validator  *auth.Validator
	access     *authz.Access
	reminderUC usecase.ReminderUsecase
	chatUC     usecase.ChatUsecase
	memberUC   usecase.MemberUsecase
	log        *slog.Logger
}

// NewServer собирает HTTP-сервер Mini App.
func NewServer(d Deps) *http.Server {
	s := &server{
		cfg:        d.Config,
		env:        d.Env,
		validator:  auth.NewValidator(d.BotToken, d.Config.InitDataTTL),
		access:     authz.New(d.Bot, d.ChatUC),
		reminderUC: d.ReminderUC,
		chatUC:     d.ChatUC,
		memberUC:   d.MemberUC,
		log:        d.Log,
	}

	return &http.Server{
		Addr:              d.Config.Addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
