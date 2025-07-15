package handler

import (
	"fmt"
	"sync"
)

type AddReminderStep int

const (
	StepNone AddReminderStep = iota
	StepType
	StepTime
	StepText
	StepInterval
	StepDate
	StepConfirm
	StepTimezone
)

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

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*AddReminderSession // key: chatID:userID
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*AddReminderSession)}
}

func sessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

func (sm *SessionManager) Get(chatID, userID int64) *AddReminderSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.sessions[sessionKey(chatID, userID)]
}

func (sm *SessionManager) Set(s *AddReminderSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[sessionKey(s.ChatID, s.UserID)] = s
}

func (sm *SessionManager) Delete(chatID, userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionKey(chatID, userID))
}
