package webapp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/authz"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/internal/scheduling"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
)

// errorResponse — единый формат ошибки API.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Заголовки уже отправлены, менять код ответа поздно — остаётся журнал.
		slog.Error("Failed to encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

// writeDomainError переводит ошибку слоёв ниже в HTTP-ответ.
//
// Чужое напоминание отдаётся как 404, а не 403: 403 подтвердил бы, что запись
// с таким идентификатором существует.
func (s *server) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrReminderNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Напоминание не найдено")

	case errors.Is(err, repository.ErrChatNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Чат не найден")

	case errors.Is(err, authz.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Нет доступа к этому чату")

	case errors.Is(err, domain.ErrTooManyReminders):
		writeError(w, http.StatusConflict, "too_many_reminders", err.Error())

	case errors.Is(err, usecase.ErrInvalidTimezone):
		writeError(w, http.StatusBadRequest, "invalid_timezone", "Неизвестный часовой пояс")

	case errors.Is(err, domain.ErrEmptyText),
		errors.Is(err, domain.ErrTextTooLong),
		errors.Is(err, domain.ErrInvalidChatID),
		errors.Is(err, domain.ErrInvalidRepeat),
		errors.Is(err, repository.ErrInvalidReminder),
		errors.Is(err, scheduling.ErrInvalidDate),
		errors.Is(err, scheduling.ErrInvalidInterval):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())

	default:
		slog.Error("Unhandled error in web app handler", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
	}
}

// decodeJSON читает тело запроса, отвергая неизвестные поля и слишком большие тела.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	const maxBodyBytes = 64 << 10

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Некорректное тело запроса")

		return false
	}

	return true
}
