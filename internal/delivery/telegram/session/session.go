package session

import (
	"fmt"
	"sync"
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

// Manager управляет сессиями добавления напоминаний.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*AddReminderSession // key: chatID:userID
}

// NewSessionManager создает новый Manager.
func NewSessionManager() *Manager {
	return &Manager{sessions: make(map[string]*AddReminderSession)}
}

func sessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// Get возвращает сессию по chatID и userID.
func (sm *Manager) Get(chatID, userID int64) *AddReminderSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[sessionKey(chatID, userID)]
}

// Set сохраняет сессию.
func (sm *Manager) Set(s *AddReminderSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[sessionKey(s.ChatID, s.UserID)] = s
}

// Delete удаляет сессию по chatID и userID.
func (sm *Manager) Delete(chatID, userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionKey(chatID, userID))
}
