package session

import (
	"sync"
	"time"
)

// AddReminderStep описывает шаг мастера добавления напоминания.
type AddReminderStep int

// Возможные шаги мастера добавления напоминания.
const (
	StepNone     AddReminderStep = iota // начальное состояние
	StepType                            // выбор типа
	StepTime                            // ввод времени
	StepText                            // ввод текста
	StepInterval                        // ввод интервала
	StepDate                            // ввод даты
	StepConfirm                         // подтверждение
	StepTimezone                        // ввод таймзоны
)

// sessionTTL — срок жизни брошенного мастера.
//
// Пользователь может закрыть чат посреди диалога, и без ограничения такая сессия
// осталась бы в памяти навсегда.
const sessionTTL = 30 * time.Minute

// AddReminderSession хранит состояние сессии добавления напоминания.
type AddReminderSession struct {
	UserID   int64
	ChatID   int64
	Step     AddReminderStep
	Type     string // today, tomorrow, everyday, etc
	Time     string // 15:00
	Date     string // 13.06.2025
	Interval int    // N дней
	Text     string // текст напоминания
}

type sessionKey struct {
	chatID int64
	userID int64
}

type sessionEntry struct {
	session   AddReminderSession
	expiresAt time.Time
}

// Manager управляет сессиями добавления напоминаний.
type Manager struct {
	mu       sync.Mutex
	sessions map[sessionKey]sessionEntry
	now      func() time.Time
}

// NewSessionManager создает новый Manager.
func NewSessionManager() *Manager {
	return &Manager{
		sessions: make(map[sessionKey]sessionEntry),
		now:      time.Now,
	}
}

// Get возвращает копию сессии по chatID и userID или nil, если её нет либо она истекла.
//
// Возвращается именно копия: telebot обрабатывает каждое обновление в своей горутине,
// и отдача указателя на хранимую структуру приводила к гонке при правке полей
// вызывающим кодом.
func (sm *Manager) Get(chatID, userID int64) *AddReminderSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, ok := sm.sessions[sessionKey{chatID: chatID, userID: userID}]
	if !ok {
		return nil
	}
	if sm.now().After(entry.expiresAt) {
		delete(sm.sessions, sessionKey{chatID: chatID, userID: userID})
		return nil
	}

	copied := entry.session

	return &copied
}

// Set сохраняет сессию и продлевает её срок жизни.
func (sm *Manager) Set(s *AddReminderSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.evictExpiredLocked()
	sm.sessions[sessionKey{chatID: s.ChatID, userID: s.UserID}] = sessionEntry{
		session:   *s,
		expiresAt: sm.now().Add(sessionTTL),
	}
}

// Delete удаляет сессию по chatID и userID.
func (sm *Manager) Delete(chatID, userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionKey{chatID: chatID, userID: userID})
}

// Len возвращает число живых сессий. Используется в тестах.
func (sm *Manager) Len() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.evictExpiredLocked()

	return len(sm.sessions)
}

func (sm *Manager) evictExpiredLocked() {
	now := sm.now()
	for key, entry := range sm.sessions {
		if now.After(entry.expiresAt) {
			delete(sm.sessions, key)
		}
	}
}
