package webapp

import (
	"net/http"
)

// routes собирает дерево маршрутов.
//
// Роутер — http.ServeMux из стандартной библиотеки: с Go 1.22 он понимает метод
// и параметры пути, и отдельная зависимость для десятка маршрутов не нужна.
func (s *server) routes() http.Handler {
	api := http.NewServeMux()

	api.HandleFunc("GET /api/v1/me", s.handleMe)
	api.HandleFunc("GET /api/v1/timezones", s.handleTimezones)

	api.HandleFunc("GET /api/v1/chats/{chatID}", s.handleGetChat)
	api.HandleFunc("PUT /api/v1/chats/{chatID}/timezone", s.handleSetTimezone)
	api.HandleFunc("GET /api/v1/chats/{chatID}/reminders", s.handleListReminders)
	api.HandleFunc("POST /api/v1/chats/{chatID}/reminders", s.handleCreateReminder)

	api.HandleFunc("GET /api/v1/reminders/{id}", s.handleGetReminder)
	api.HandleFunc("PATCH /api/v1/reminders/{id}", s.handleUpdateReminder)
	api.HandleFunc("DELETE /api/v1/reminders/{id}", s.handleDeleteReminder)

	root := http.NewServeMux()

	// Аутентификация и лимит запросов действуют только на /api: статика отдаётся
	// до того, как Telegram передаст странице initData.
	root.Handle("/api/", s.withAuth(s.withRateLimit(api)))
	root.HandleFunc("GET /healthz", s.handleHealth)
	root.Handle("/", s.staticHandler())

	return s.withRecover(s.withRequestLog(s.withSecurityHeaders(root)))
}
