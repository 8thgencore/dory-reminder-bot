package handler

import (
	"testing"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/wizards"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
)

type mockContext struct {
	tele.Context
	text      string
	sendCalls []string
}

func (m *mockContext) Text() string {
	return m.text
}

func (m *mockContext) Send(msg interface{}, opts ...interface{}) error {
	m.sendCalls = append(m.sendCalls, msg.(string))
	return nil
}

func (m *mockContext) Sender() *tele.User {
	return &tele.User{ID: 1}
}

func (m *mockContext) Chat() *tele.Chat {
	return &tele.Chat{ID: 1}
}

// TestAddWizard_NDaysFlow проверяет сценарий добавления напоминания с типом ndays.
func TestAddWizard_NDaysFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := wizards.NewAddReminderWizard(nil, sessionMgr) // nil usecase для тестов

	// Шаг 1: пользователь выбрал тип "ndays", сессия ожидает дату
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "ndays", Step: session.StepDate,
	}
	sessionMgr.Set(sess)

	// Вводим дату старта
	c := &mockContext{text: "13.06.2024"}
	err := wizard.HandleAddWizardText(c)
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "13.06.2024", sess.Date)
	assert.Equal(t, session.StepInterval, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "Введите интервал")

	// Вводим интервал
	c2 := &mockContext{text: "10"}
	err = wizard.HandleAddWizardText(c2)
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 10, sess.Interval)
	assert.Equal(t, session.StepTime, sess.Step)
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Во сколько")
}
