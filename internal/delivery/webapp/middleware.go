package webapp

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/auth"
)

// initDataPrefix — схема заголовка Authorization, принятая в Telegram Mini Apps.
const initDataPrefix = "tma "

// initDataHeader — запасной заголовок для клиентов, которым неудобен Authorization.
const initDataHeader = "X-Telegram-Init-Data"

type contextKey struct{ name string }

var userContextKey = contextKey{name: "webapp.user"}

// userFrom возвращает проверенные данные пользователя из контекста запроса.
// Наличие значения гарантирует withAuth.
func userFrom(ctx context.Context) *auth.InitData {
	data, _ := ctx.Value(userContextKey).(*auth.InitData)

	return data
}

// withAuth проверяет подпись initData и кладёт пользователя в контекст запроса.
func (s *server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := extractInitData(r)

		data, err := s.validator.Validate(raw)
		if err != nil {
			// Причину наружу не раскрываем: она подсказала бы, чем именно
			// не подошла подделка.
			s.log.Debug("Rejected init data", "path", r.URL.Path, "error", err)
			writeError(w, http.StatusUnauthorized, "unauthorized", "Не удалось подтвердить вход через Telegram")

			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, data)))
	})
}

func extractInitData(r *http.Request) string {
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, initDataPrefix) {
		return strings.TrimPrefix(header, initDataPrefix)
	}

	return r.Header.Get(initDataHeader)
}

// withRecover не даёт панике в обработчике уронить процесс: в нём же работает бот.
func (s *server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("Panic in HTTP handler", "path", r.URL.Path, "panic", rec)
				writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// statusRecorder запоминает код ответа для журнала.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// withRequestLog журналирует запросы. Тело не пишется: в нём тексты напоминаний.
func (s *server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		s.log.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(started),
		)
	})
}

// withSecurityHeaders выставляет заголовки безопасности.
func (s *server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// telegram-web-app.js обязателен для Mini App и грузится только с telegram.org.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://telegram.org; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self'; "+
				"base-uri 'none'; "+
				"form-action 'none'; "+
				"frame-ancestors https://telegram.org https://web.telegram.org")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")

		next.ServeHTTP(w, r)
	})
}

// Параметры ограничения частоты запросов.
const (
	rateLimitPerMinute = 60
	rateLimitWindow    = time.Minute
)

// rateLimiter — счётчик запросов в скользящем окне, по пользователю.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[int64]*rateWindow
}

type rateWindow struct {
	count     int
	resetsAt  time.Time
	lastUsage time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{windows: make(map[int64]*rateWindow)}
}

// allow уменьшает остаток запросов пользователя и сообщает, можно ли обслужить запрос.
func (l *rateLimiter) allow(userID int64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[userID]
	if !ok || now.After(w.resetsAt) {
		w = &rateWindow{resetsAt: now.Add(rateLimitWindow)}
		l.windows[userID] = w
	}
	w.lastUsage = now
	w.count++

	if len(l.windows)%256 == 0 {
		l.evictStaleLocked(now)
	}

	return w.count <= rateLimitPerMinute
}

// evictStaleLocked убирает пользователей, давно не делавших запросов, чтобы карта
// не росла бесконечно.
func (l *rateLimiter) evictStaleLocked(now time.Time) {
	for id, w := range l.windows {
		if now.Sub(w.lastUsage) > 10*rateLimitWindow {
			delete(l.windows, id)
		}
	}
}

// withRateLimit ограничивает частоту запросов одного пользователя.
// Ставится после withAuth: лимит считается по проверенному user_id, а не по IP,
// потому что все запросы приходят через один reverse proxy.
func (s *server) withRateLimit(next http.Handler) http.Handler {
	limiter := newRateLimiter()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFrom(r.Context())
		if user == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Не удалось подтвердить вход через Telegram")
			return
		}

		if !limiter.allow(user.User.ID, time.Now()) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов, попробуйте позже")

			return
		}

		next.ServeHTTP(w, r)
	})
}

// logHandlerError пишет в журнал непредвиденные ошибки обработчиков.
func (s *server) logHandlerError(r *http.Request, err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	s.log.Error("Handler failed", "method", r.Method, "path", r.URL.Path, "error", err)
}
