package webapp

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/auth"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/authz"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/pkg/timezone"
)

const recentWebAppLaunchTTL = 10 * time.Minute

type launchCandidate struct {
	id     int64
	source string
	chat   *auth.Chat
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleTimezones(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"timezones": timezone.ValidTimezones})
}

// handleMe отдаёт профиль пользователя и список доступных ему чатов.
func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())

	// Личный чат существует всегда: его идентификатор совпадает с идентификатором
	// пользователя, даже если бот ещё не сохранял этот чат.
	chats := []chatDTO{{
		ID:       user.User.ID,
		Type:     chatTypePrivate,
		Title:    displayName(user),
		Username: user.User.Username,
	}}
	included := map[int64]bool{user.User.ID: true}

	known, err := s.memberUC.ListChats(r.Context(), user.User.ID)
	if err != nil {
		s.logHandlerError(r, err)
		s.writeDomainError(w, err)

		return
	}

	for _, chat := range known {
		if chat.ID == user.User.ID {
			// Личный чат уже добавлен, но с данными из initData — берём таймзону из базы.
			chats[0].Timezone = chat.Timezone
			continue
		}
		if included[chat.ID] {
			continue
		}

		// Запись в chat_members означает лишь, что бот когда-то видел пользователя
		// в группе. Перед раскрытием названия и добавлением чата в ответ проверяем
		// актуальное членство через Telegram.
		resolvedID, err := s.access.Resolve(r.Context(), user.User.ID, chat.ID)
		if err != nil {
			if !errors.Is(err, authz.ErrForbidden) {
				s.logHandlerError(r, err)
			}
			continue
		}
		if included[resolvedID] {
			continue
		}
		if resolvedID != chat.ID {
			chat, err = s.chatUC.Get(r.Context(), resolvedID)
			if err != nil {
				s.logHandlerError(r, err)
				continue
			}
		}

		chats = append(chats, toChatDTO(chat))
		included[resolvedID] = true
	}

	launchChatID, launchSource := s.resolveLaunchChat(r, user, &chats, included)

	if chats[0].Timezone == "" {
		if chat, err := s.chatUC.Get(r.Context(), user.User.ID); err == nil {
			chats[0].Timezone = chat.Timezone
		}
	}
	if launchChatID != 0 {
		// Чат запуска ставим первым не только для текущего клиента, который читает
		// LaunchChatID, но и для уже открытых Telegram WebView со старым app.js:
		// такие клиенты выбирают первый элемент списка.
		chats = prioritizeChat(chats, launchChatID)
	}

	s.log.Debug(
		"Resolved Mini App launch context",
		"user_id", user.User.ID,
		"launch_chat_id", launchChatID,
		"launch_source", launchSource,
	)

	writeJSON(w, http.StatusOK, meResponse{
		User: userDTO{
			ID:        user.User.ID,
			FirstName: user.User.FirstName,
			LastName:  user.User.LastName,
			Username:  user.User.Username,
		},
		Chats:        chats,
		LaunchChatID: launchChatID,
	})
}

// resolveLaunchChat выбирает первый доступный чат из подписанных или серверных
// подсказок. Подсказка влияет только на выбор по умолчанию: право доступа всё равно
// подтверждается Bot API.
func (s *server) resolveLaunchChat(
	r *http.Request,
	user *auth.InitData,
	chats *[]chatDTO,
	included map[int64]bool,
) (int64, string) {
	candidates := s.launchCandidates(r, user)
	seen := make(map[int64]bool, len(candidates))

	for _, candidate := range candidates {
		if candidate.id == 0 || seen[candidate.id] {
			continue
		}
		seen[candidate.id] = true

		resolvedID, err := s.access.Resolve(r.Context(), user.User.ID, candidate.id)
		if err != nil {
			if !errors.Is(err, authz.ErrForbidden) {
				s.logHandlerError(r, err)
			}
			continue
		}
		if included[resolvedID] {
			return resolvedID, candidate.source
		}

		chat, err := s.chatUC.Get(r.Context(), resolvedID)
		switch {
		case errors.Is(err, repository.ErrChatNotFound):
			dto := chatDTO{
				ID:       resolvedID,
				Type:     chatTypeGroup,
				IsPublic: true,
			}
			if candidate.chat != nil {
				dto.Type = candidate.chat.Type
				dto.Title = candidate.chat.Title
				dto.Username = candidate.chat.Username
			}
			*chats = append(*chats, dto)
		case err != nil:
			s.logHandlerError(r, err)
			continue
		default:
			*chats = append(*chats, toChatDTO(chat))
		}

		included[resolvedID] = true
		return resolvedID, candidate.source
	}

	return 0, "none"
}

func (s *server) launchCandidates(r *http.Request, user *auth.InitData) []launchCandidate {
	candidates := make([]launchCandidate, 0, 3)

	if id, ok := parseLaunchChatID(user.StartParam); ok {
		candidates = append(candidates, launchCandidate{id: id, source: "start_param"})
	}

	if user.Chat != nil && isGroupChatType(user.Chat.Type) {
		candidates = append(candidates, launchCandidate{
			id:     user.Chat.ID,
			source: "signed_chat",
			chat:   user.Chat,
		})
	}

	if isGroupChatType(user.ChatType) {
		chat, err := s.memberUC.RecentWebAppLaunch(
			r.Context(),
			user.User.ID,
			time.Now().Add(-recentWebAppLaunchTTL),
		)
		switch {
		case err == nil:
			candidates = append(candidates, launchCandidate{
				id:     chat.ID,
				source: "recent_app_command",
			})
		case errors.Is(err, repository.ErrChatNotFound):
			// Нет свежего /app — это обычный запуск, а не ошибка.
		default:
			s.logHandlerError(r, err)
		}
	}

	return candidates
}

func isGroupChatType(chatType string) bool {
	return chatType == chatTypeGroup || chatType == "supergroup"
}

func prioritizeChat(chats []chatDTO, chatID int64) []chatDTO {
	for i := range chats {
		if chats[i].ID != chatID {
			continue
		}
		if i == 0 {
			return chats
		}

		preferred := chats[i]
		copy(chats[1:i+1], chats[:i])
		chats[0] = preferred

		return chats
	}

	return chats
}

func parseLaunchChatID(startParam string) (int64, bool) {
	const prefix = "chat_"
	if !strings.HasPrefix(startParam, prefix) {
		return 0, false
	}

	id, err := strconv.ParseInt(strings.TrimPrefix(startParam, prefix), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}

	return id, true
}

// displayName собирает отображаемое имя пользователя для карточки личного чата.
func displayName(user *auth.InitData) string {
	name := strings.TrimSpace(user.User.FirstName + " " + user.User.LastName)
	if name != "" {
		return name
	}
	if user.User.Username != "" {
		return "@" + user.User.Username
	}

	return "Личные напоминания"
}

// handleGetChat отдаёт настройки чата.
func (s *server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.authorizeChat(w, r)
	if !ok {
		return
	}

	chat, err := s.chatUC.Get(r.Context(), chatID)
	if errors.Is(err, repository.ErrChatNotFound) {
		// Бот ещё не видел этот чат — отдаём пустые настройки вместо 404,
		// иначе Mini App не смог бы задать часовой пояс первым действием.
		chatType := chatTypeFor(chatID, r)
		writeJSON(w, http.StatusOK, chatDTO{
			ID:       chatID,
			Type:     chatType,
			IsPublic: chatType != chatTypePrivate,
		})

		return
	}
	if err != nil {
		s.logHandlerError(r, err)
		s.writeDomainError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, toChatDTO(chat))
}

// chatTypeFor определяет тип чата, когда записи о нём ещё нет.
func chatTypeFor(chatID int64, r *http.Request) string {
	if user := userFrom(r.Context()); user != nil && user.User.ID == chatID {
		return chatTypePrivate
	}

	return chatTypeGroup
}

// handleSetTimezone задаёт часовой пояс чата.
func (s *server) handleSetTimezone(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.authorizeChat(w, r)
	if !ok {
		return
	}

	var req timezoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// В базе может не быть строки чата: Mini App открывают и не написав боту ни разу.
	if _, err := s.chatUC.GetOrCreateChat(r.Context(), chatID, chatTypeFor(chatID, r), "", ""); err != nil {
		s.logHandlerError(r, err)
		s.writeDomainError(w, err)

		return
	}

	if err := s.chatUC.SetTimezone(r.Context(), chatID, req.Timezone); err != nil {
		s.writeDomainError(w, err)
		return
	}

	chatType := chatTypeFor(chatID, r)
	writeJSON(w, http.StatusOK, chatDTO{
		ID:       chatID,
		Type:     chatType,
		Timezone: req.Timezone,
		IsPublic: chatType != chatTypePrivate,
	})
}

// handleListReminders отдаёт напоминания чата.
func (s *server) handleListReminders(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.authorizeChat(w, r)
	if !ok {
		return
	}

	reminders, err := s.reminderUC.ListReminders(r.Context(), chatID)
	if err != nil {
		s.logHandlerError(r, err)
		s.writeDomainError(w, err)

		return
	}

	items := make([]reminderDTO, 0, len(reminders))
	for _, rem := range reminders {
		items = append(items, toReminderDTO(rem))
	}

	writeJSON(w, http.StatusOK, reminderListResponse{
		Timezone:  s.timezoneOf(r, chatID),
		Reminders: items,
	})
}

// handleCreateReminder создаёт напоминание в чате.
func (s *server) handleCreateReminder(w http.ResponseWriter, r *http.Request) {
	chatID, ok := s.authorizeChat(w, r)
	if !ok {
		return
	}

	var req reminderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Text == nil || req.Repeat == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Нужно указать текст и тип повтора")
		return
	}

	rem := &domain.Reminder{ChatID: chatID}
	if err := s.applyRequest(rem, req, s.chatUC.Location(r.Context(), chatID)); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if err := s.reminderUC.AddReminder(r.Context(), rem); err != nil {
		s.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toReminderDTO(rem))
}

// handleGetReminder отдаёт одно напоминание.
func (s *server) handleGetReminder(w http.ResponseWriter, r *http.Request) {
	rem, ok := s.loadOwnedReminder(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, toReminderDTO(rem))
}

// handleUpdateReminder меняет напоминание.
func (s *server) handleUpdateReminder(w http.ResponseWriter, r *http.Request) {
	rem, ok := s.loadOwnedReminder(w, r)
	if !ok {
		return
	}

	var req reminderRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := s.applyRequest(rem, req, s.chatUC.Location(r.Context(), rem.ChatID)); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if err := s.reminderUC.UpdateOwned(r.Context(), rem, rem.ChatID); err != nil {
		s.writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toReminderDTO(rem))
}

// handleDeleteReminder удаляет напоминание.
func (s *server) handleDeleteReminder(w http.ResponseWriter, r *http.Request) {
	rem, ok := s.loadOwnedReminder(w, r)
	if !ok {
		return
	}

	if err := s.reminderUC.DeleteOwned(r.Context(), rem.ID, rem.ChatID); err != nil {
		s.writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// loadOwnedReminder загружает напоминание по идентификатору из пути и проверяет,
// что запросивший пользователь имеет доступ к его чату.
//
// Идентификатор чата в этих маршрутах не участвует, поэтому порядок обратный обычному:
// сначала читаем напоминание, затем авторизуем его чат. Любая неудача — 404, чтобы
// перебор идентификаторов не выдавал существующие записи.
func (s *server) loadOwnedReminder(w http.ResponseWriter, r *http.Request) (*domain.Reminder, bool) {
	notFound := func() (*domain.Reminder, bool) {
		writeError(w, http.StatusNotFound, "not_found", "Напоминание не найдено")

		return nil, false
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return notFound()
	}

	rem, err := s.reminderUC.GetReminder(r.Context(), id)
	if err != nil {
		if !errors.Is(err, repository.ErrReminderNotFound) {
			s.logHandlerError(r, err)
		}

		return notFound()
	}

	user := userFrom(r.Context())
	resolvedID, err := s.access.Resolve(r.Context(), user.User.ID, rem.ChatID)
	if err != nil {
		if !errors.Is(err, authz.ErrForbidden) {
			s.logHandlerError(r, err)
		}

		return notFound()
	}
	// Миграция в Resolve уже перенесла строку напоминания в БД. Обновляем и
	// загруженный объект, чтобы последующие update/delete работали с новым chat_id.
	rem.ChatID = resolvedID

	return rem, true
}

// authorizeChat читает chatID из пути и проверяет права пользователя на этот чат.
func (s *server) authorizeChat(w http.ResponseWriter, r *http.Request) (int64, bool) {
	user := userFrom(r.Context())

	chatID, err := strconv.ParseInt(r.PathValue("chatID"), 10, 64)
	if err != nil || chatID == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Некорректный идентификатор чата")
		return 0, false
	}

	resolvedID, err := s.access.Resolve(r.Context(), user.User.ID, chatID)
	if err != nil {
		if !errors.Is(err, authz.ErrForbidden) {
			s.logHandlerError(r, err)
		}
		writeError(w, http.StatusForbidden, "forbidden", "Нет доступа к этому чату")

		return 0, false
	}

	return resolvedID, true
}

// timezoneOf возвращает часовой пояс чата для отображения на клиенте.
func (s *server) timezoneOf(r *http.Request, chatID int64) string {
	chat, err := s.chatUC.Get(r.Context(), chatID)
	if err != nil {
		return time.UTC.String()
	}
	if chat.Timezone == "" {
		return ""
	}

	return chat.Timezone
}
