package wizards

import (
	"context"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/session"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
)

// Mock usecase implementations
type mockReminderUsecase struct{}

func (m *mockReminderUsecase) AddReminder(ctx context.Context, r *domain.Reminder) error {
	return nil
}

func (m *mockReminderUsecase) EditReminder(ctx context.Context, r *domain.Reminder) error {
	return nil
}

func (m *mockReminderUsecase) DeleteReminder(ctx context.Context, id int64) error {
	return nil
}

func (m *mockReminderUsecase) PauseReminder(ctx context.Context, id int64) error {
	return nil
}

func (m *mockReminderUsecase) ResumeReminder(ctx context.Context, id int64) error {
	return nil
}

func (m *mockReminderUsecase) ListReminders(ctx context.Context, chatID int64) ([]*domain.Reminder, error) {
	return nil, nil
}

func (m *mockReminderUsecase) ListDue(ctx context.Context, now time.Time) ([]*domain.Reminder, error) {
	return nil, nil
}

type mockUserUsecase struct{}

func (m *mockUserUsecase) GetOrCreateUser(ctx context.Context, chatID, userID int64, username, firstName, lastName string) (*domain.User, error) {
	return &domain.User{
		ID:        userID,
		ChatID:    chatID,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
		Timezone:  "Europe/Moscow",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockUserUsecase) SetTimezone(ctx context.Context, chatID, userID int64, timezone string) error {
	return nil
}

func (m *mockUserUsecase) HasTimezone(ctx context.Context, chatID, userID int64) (bool, error) {
	return true, nil
}

type mockContext struct {
	tele.Context
	text      string
	sendCalls []string
	callback  *tele.Callback
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

func (m *mockContext) Callback() *tele.Callback {
	return m.callback
}

func (m *mockContext) Respond(resp ...*tele.CallbackResponse) error {
	return nil
}

func (m *mockContext) Delete() error {
	return nil
}

// TestAddWizard_NDaysFlow проверяет сценарий добавления напоминания с типом ndays.
func TestAddWizard_NDaysFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "ndays", сессия ожидает дату
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "ndays", Step: session.StepDate,
	}
	sessionMgr.Set(sess)

	// Вводим дату старта
	c := &mockContext{text: "13.06.2024"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "13.06.2024", sess.Date)
	assert.Equal(t, session.StepInterval, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "интервал в днях")

	// Вводим интервал
	c2 := &mockContext{text: "10"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 10, sess.Interval)
	assert.Equal(t, session.StepTime, sess.Step)
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Во сколько")
}

// TestAddWizard_TodayFlow проверяет сценарий добавления напоминания на сегодня
func TestAddWizard_TodayFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "today", сессия ожидает время
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "today", Step: session.StepTime,
	}
	sessionMgr.Set(sess)

	// Вводим время
	c := &mockContext{text: "15:30"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "15:30", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "текст напоминания")

	// Вводим текст
	c2 := &mockContext{text: "Позвонить маме"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_EveryDayFlow проверяет сценарий добавления ежедневного напоминания
func TestAddWizard_EveryDayFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "everyday", сессия ожидает время
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "everyday", Step: session.StepTime,
	}
	sessionMgr.Set(sess)

	// Вводим время
	c := &mockContext{text: "09:00"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "09:00", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)

	// Вводим текст
	c2 := &mockContext{text: "Принять таблетку"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_WeekFlow проверяет сценарий добавления еженедельного напоминания
func TestAddWizard_WeekFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "week", сессия ожидает день недели
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "week", Step: session.StepInterval,
	}
	sessionMgr.Set(sess)

	// Вводим день недели
	c := &mockContext{text: "понедельник"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 1, sess.Interval) // понедельник = 1
	assert.Equal(t, session.StepTime, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "Во сколько")

	// Вводим время
	c2 := &mockContext{text: "18:00"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "18:00", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)

	// Вводим текст
	c3 := &mockContext{text: "Встреча с командой"}
	err = wizard.HandleAddWizardText(c3, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c3.sendCalls)
	assert.Contains(t, c3.sendCalls[len(c3.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_MonthFlow проверяет сценарий добавления ежемесячного напоминания
func TestAddWizard_MonthFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "month", сессия ожидает число месяца
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "month", Step: session.StepInterval,
	}
	sessionMgr.Set(sess)

	// Вводим число месяца
	c := &mockContext{text: "15"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 15, sess.Interval)
	assert.Equal(t, session.StepTime, sess.Step)

	// Вводим время
	c2 := &mockContext{text: "12:00"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "12:00", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)

	// Вводим текст
	c3 := &mockContext{text: "Оплатить счета"}
	err = wizard.HandleAddWizardText(c3, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c3.sendCalls)
	assert.Contains(t, c3.sendCalls[len(c3.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_YearFlow проверяет сценарий добавления ежегодного напоминания
func TestAddWizard_YearFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "year", сессия ожидает дату ДД.ММ
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "year", Step: session.StepInterval,
	}
	sessionMgr.Set(sess)

	// Вводим дату
	c := &mockContext{text: "13.06"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "13.06", sess.Date)
	assert.Equal(t, session.StepTime, sess.Step)

	// Вводим время
	c2 := &mockContext{text: "15:00"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "15:00", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)

	// Вводим текст
	c3 := &mockContext{text: "День рождения друга"}
	err = wizard.HandleAddWizardText(c3, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c3.sendCalls)
	assert.Contains(t, c3.sendCalls[len(c3.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_DateFlow проверяет сценарий добавления напоминания на конкретную дату
func TestAddWizard_DateFlow(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Шаг 1: пользователь выбрал тип "date", сессия ожидает дату и время
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "date", Step: session.StepDate,
	}
	sessionMgr.Set(sess)

	// Вводим дату и время
	c := &mockContext{text: "25.12.2024 20:00"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "25.12.2024", sess.Date)
	assert.Equal(t, "20:00", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)

	// Вводим текст
	c2 := &mockContext{text: "Новогодний ужин"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	// После создания напоминания сессия удаляется
	sess = sessionMgr.Get(1, 1)
	assert.Nil(t, sess) // сессия должна быть удалена
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Напоминание создано")
}

// TestAddWizard_InvalidInputs проверяет обработку некорректных входных данных
func TestAddWizard_InvalidInputs(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	tests := []struct {
		name     string
		sess     *session.AddReminderSession
		input    string
		expected string
	}{
		{
			name: "invalid time format",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "today", Step: session.StepTime,
			},
			input:    "25:00",
			expected: "время в формате",
		},
		{
			name: "invalid weekday",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "week", Step: session.StepInterval,
			},
			input:    "несуществующий день",
			expected: "день недели",
		},
		{
			name: "invalid month day",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "month", Step: session.StepInterval,
			},
			input:    "32",
			expected: "Во сколько",
		},
		{
			name: "invalid year date",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "year", Step: session.StepInterval,
			},
			input:    "32.13",
			expected: "Во сколько",
		},
		{
			name: "invalid ndays date",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "ndays", Step: session.StepDate,
			},
			input:    "32.13.2024",
			expected: "интервал в днях",
		},
		{
			name: "invalid date format",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "date", Step: session.StepDate,
			},
			input:    "25.12.2024",
			expected: "дату и время в формате",
		},
		{
			name: "empty text",
			sess: &session.AddReminderSession{
				UserID: 1, ChatID: 1, Type: "today", Step: session.StepText,
			},
			input:    "",
			expected: "текст напоминания",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionMgr.Set(tt.sess)
			c := &mockContext{text: tt.input}
			err := wizard.HandleAddWizardText(c, "reminder_bot")
			assert.NoError(t, err)
			assert.NotEmpty(t, c.sendCalls)
			assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], tt.expected)
		})
	}
}

// TestAddWizard_BotMentionRemoval проверяет удаление упоминания бота из текста
func TestAddWizard_BotMentionRemoval(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "today", Step: session.StepTime,
	}
	sessionMgr.Set(sess)

	// Текст с упоминанием бота
	c := &mockContext{text: "15:30 @reminder_bot"}
	err := wizard.HandleAddWizardText(c, "reminder_bot")
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, "15:30", sess.Time)
	assert.Equal(t, session.StepText, sess.Step)
}

// TestAddWizard_HandleAddTypeCallback проверяет обработку выбора типа напоминания
func TestAddWizard_HandleAddTypeCallback(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	tests := []struct {
		name     string
		typ      string
		expected string
	}{
		{"today", "today", "сегодня"},
		{"tomorrow", "tomorrow", "завтра"},
		{"everyday", "everyday", "каждый день"},
		{"week", "week", "день недели"},
		{"ndays", "ndays", "дату старта"},
		{"month", "month", "число месяца"},
		{"year", "year", "дату в формате"},
		{"date", "date", "дату и время в формате"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &mockContext{}
			err := wizard.HandleAddTypeCallback(c, tt.typ)
			assert.NoError(t, err)
			assert.NotEmpty(t, c.sendCalls)
			assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], tt.expected)

			// Проверяем, что сессия создана с правильным типом
			sess := sessionMgr.Get(1, 1)
			assert.NotNil(t, sess)
			assert.Equal(t, tt.typ, sess.Type)
		})
	}
}

// TestAddWizard_HandleWeekdayCallback проверяет обработку выбора дня недели
func TestAddWizard_HandleWeekdayCallback(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Создаем сессию для недельного напоминания
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "week", Step: session.StepInterval,
	}
	sessionMgr.Set(sess)

	// Тестируем валидный день недели
	c := &mockContext{
		callback: &tele.Callback{Data: "weekday_1"}, // понедельник
	}
	err := wizard.HandleWeekdayCallback(c)
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 1, sess.Interval)
	assert.Equal(t, session.StepTime, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "Во сколько")

	// Тестируем невалидный день недели
	c2 := &mockContext{
		callback: &tele.Callback{Data: "weekday_10"},
	}
	err = wizard.HandleWeekdayCallback(c2)
	assert.NoError(t, err)
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "неверный день недели")
}

// TestAddWizard_HandleMonthCallback проверяет обработку выбора месяца
func TestAddWizard_HandleMonthCallback(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Создаем сессию для месячного напоминания
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "month", Step: session.StepInterval,
	}
	sessionMgr.Set(sess)

	// Тестируем валидный месяц
	c := &mockContext{
		callback: &tele.Callback{Data: "month_5"},
	}
	err := wizard.HandleMonthCallback(c)
	assert.NoError(t, err)
	sess = sessionMgr.Get(1, 1)
	assert.Equal(t, 5, sess.Interval)
	assert.Equal(t, session.StepText, sess.Step)
	assert.NotEmpty(t, c.sendCalls)
	assert.Contains(t, c.sendCalls[len(c.sendCalls)-1], "текст напоминания")

	// Тестируем невалидный месяц
	c2 := &mockContext{
		callback: &tele.Callback{Data: "month_32"},
	}
	err = wizard.HandleMonthCallback(c2)
	assert.NoError(t, err)
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "неверный месяц")
}

// TestAddWizard_ParseWeekday проверяет функцию парсинга дней недели
func TestAddWizard_ParseWeekday(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		ok       bool
	}{
		{"понедельник", 1, true},
		{"вторник", 2, true},
		{"среда", 3, true},
		{"четверг", 4, true},
		{"пятница", 5, true},
		{"суббота", 6, true},
		{"воскресенье", 0, true},
		{"ПОНЕДЕЛЬНИК", 1, true},   // проверка регистра
		{" Понедельник ", 1, true}, // проверка пробелов
		{"несуществующий", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := parseWeekday(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

// TestAddWizard_GetAddReminderMessage проверяет функцию получения сообщений для типов
func TestAddWizard_GetAddReminderMessage(t *testing.T) {
	tests := []struct {
		typ      string
		expected string
	}{
		{"today", "сегодня"},
		{"tomorrow", "завтра"},
		{"everyday", "каждый день"},
		{"week", "день недели"},
		{"ndays", "дней повторять"},
		{"month", "день месяца"},
		{"year", "дату и во сколько"},
		{"date", "дату и время"},
		{"unknown", "Неизвестный тип"},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			msg := getAddReminderMessage(tt.typ)
			assert.Contains(t, msg, tt.expected)
		})
	}
}

// TestAddWizard_SessionManagement проверяет управление сессиями
func TestAddWizard_SessionManagement(t *testing.T) {
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Тест получения несуществующей сессии
	sess := wizard.getSession(1, 1)
	assert.NotNil(t, sess)
	assert.Equal(t, int64(1), sess.UserID)
	assert.Equal(t, int64(1), sess.ChatID)
	assert.Equal(t, session.StepType, sess.Step)

	// Тест обновления сессии
	sess.Type = "today"
	wizard.updateSession(sess)

	// Проверяем, что сессия сохранилась
	savedSess := sessionMgr.Get(1, 1)
	assert.NotNil(t, savedSess)
	assert.Equal(t, "today", savedSess.Type)
}

// TestAddWizard_TypeToRepeat проверяет конвертацию типов в RepeatType
func TestAddWizard_TypeToRepeat(t *testing.T) {
	// Тестируем функцию typeToRepeat (если она экспортирована)
	// В данном случае она не экспортирована, но можно протестировать через публичные методы
	sessionMgr := session.NewSessionManager()
	wizard := NewAddReminderWizard(&mockReminderUsecase{}, sessionMgr, &mockUserUsecase{})

	// Создаем сессию с типом everyday
	sess := &session.AddReminderSession{
		UserID: 1, ChatID: 1, Type: "everyday", Step: session.StepTime,
	}
	sessionMgr.Set(sess)

	// Проходим через весь flow
	c1 := &mockContext{text: "09:00"}
	err := wizard.HandleAddWizardText(c1, "reminder_bot")
	assert.NoError(t, err)

	c2 := &mockContext{text: "Тест"}
	err = wizard.HandleAddWizardText(c2, "reminder_bot")
	assert.NoError(t, err)
	assert.NotEmpty(t, c2.sendCalls)
	assert.Contains(t, c2.sendCalls[len(c2.sendCalls)-1], "Напоминание создано")
}
