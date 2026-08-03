package webapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/config"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/auth"
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/webapp/authz"
	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository"
	"github.com/8thgencore/dory-reminder-bot/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	_ "github.com/mattn/go-sqlite3"
)

const (
	botToken   = "123456:AAHtestTokenValueForUnitTests"
	testUserID = int64(42)
	// Личный чат другого пользователя.
	foreignUserID = int64(84)
	// Пользователь состоит в этой группе.
	memberGroupID = int64(-1001111111111)
	// И в этой — нужна для проверки однозначного выбора среди нескольких групп.
	secondMemberGroupID = int64(-1003333333333)
	// В этой — нет.
	foreignGroupID = int64(-1002222222222)
)

// membershipStub изображает Bot API: пользователь состоит только в memberGroupID.
type membershipStub struct{}

func (membershipStub) ChatMemberOf(chat, _ tele.Recipient) (*tele.ChatMember, error) {
	if chat.Recipient() == "-1001111111111" || chat.Recipient() == "-1003333333333" {
		return &tele.ChatMember{Role: tele.Member}, nil
	}

	return &tele.ChatMember{Role: tele.Left}, nil
}

type desktopMigrationStub struct {
	oldChatID int64
	newChatID int64
	calls     atomic.Int64
}

func (s *desktopMigrationStub) ChatMemberOf(chat, _ tele.Recipient) (*tele.ChatMember, error) {
	s.calls.Add(1)
	switch chat.Recipient() {
	case strconv.FormatInt(s.oldChatID, 10):
		return nil, tele.GroupError{MigratedTo: s.newChatID}
	case strconv.FormatInt(s.newChatID, 10):
		return nil, tele.ErrKickedFromSuperGroup
	default:
		return &tele.ChatMember{Role: tele.Left}, nil
	}
}

// testEnv — поднятый на памяти стек: реальная БД, реальные usecase, фиктивный Bot API.
type testEnv struct {
	t        *testing.T
	server   *httptest.Server
	chatUC   usecase.ChatUsecase
	memberUC usecase.MemberUsecase
	remUC    usecase.ReminderUsecase
	db       *sql.DB
}

func newTestEnv(t *testing.T) *testEnv {
	return newTestEnvWithMembership(t, membershipStub{})
}

func newTestEnvWithMembership(t *testing.T, checker interface {
	ChatMemberOf(chat, user tele.Recipient) (*tele.ChatMember, error)
}) *testEnv {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_loc=UTC")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, repository.Migrate(db))

	remUC := usecase.NewReminderUsecase(repository.NewReminderRepository(db))
	chatUC := usecase.NewChatUsecase(repository.NewChatRepository(db))
	memberUC := usecase.NewMemberUsecase(repository.NewMemberRepository(db))

	s := &server{
		cfg:        config.WebAppConfig{InitDataTTL: time.Hour},
		env:        config.Prod,
		validator:  auth.NewValidator(botToken, time.Hour),
		access:     authz.New(checker, chatUC),
		reminderUC: remUC,
		chatUC:     chatUC,
		memberUC:   memberUC,
		log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)

	env := &testEnv{
		t: t, server: srv, chatUC: chatUC, memberUC: memberUC, remUC: remUC, db: db,
	}

	// Личный чат с известной таймзоной: без неё расчёт времени опирался бы на UTC.
	env.seedChat(testUserID, "private", "Europe/Berlin")
	env.seedChat(memberGroupID, "group", "Europe/Berlin")
	env.seedChat(secondMemberGroupID, "supergroup", "")
	env.seedChat(foreignGroupID, "group", "Europe/Berlin")

	return env
}

func (e *testEnv) seedChat(chatID int64, chatType, tz string) {
	e.t.Helper()
	ctx := context.Background()
	_, err := e.chatUC.GetOrCreateChat(ctx, chatID, chatType, "Test", "")
	require.NoError(e.t, err)
	if tz != "" {
		require.NoError(e.t, e.chatUC.SetTimezone(ctx, chatID, tz))
	}
}

// initData подписывает данные запуска для тестового пользователя.
func initData() string {
	return initDataWith(nil)
}

func initDataWith(extra map[string]string) string {
	return auth.SignInitData(
		botToken,
		auth.User{ID: testUserID, FirstName: "Дарья"},
		time.Now(),
		extra,
	)
}

// do выполняет запрос с валидным initData.
func (e *testEnv) do(method, path string, body any) *http.Response {
	e.t.Helper()

	return e.doWithAuth(method, path, body, "tma "+initData())
}

func (e *testEnv) doWithAuth(method, path string, body any, authHeader string) *http.Response {
	e.t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(e.t, err)
		reader = strings.NewReader(string(encoded))
	}

	req, err := http.NewRequestWithContext(context.Background(), method, e.server.URL+path, reader)
	require.NoError(e.t, err)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.server.Client().Do(req)
	require.NoError(e.t, err)
	e.t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var out T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))

	return out
}

// createReminder добавляет напоминание напрямую в базу, минуя HTTP.
func (e *testEnv) createReminder(chatID int64, text string) *domain.Reminder {
	e.t.Helper()

	rem := &domain.Reminder{
		ChatID:   chatID,
		Text:     text,
		NextTime: time.Now().Add(time.Hour).UTC(),
		Repeat:   domain.RepeatEveryDay,
	}
	require.NoError(e.t, e.remUC.AddReminder(context.Background(), rem))

	return rem
}

// --- Аутентификация -------------------------------------------------------

func TestAPI_RequiresValidInitData(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		name string
		auth string
	}{
		{"без заголовка", ""},
		{"мусор вместо подписи", "tma user=%7B%22id%22%3A42%7D&hash=deadbeef"},
		{"подпись чужого бота", "tma " + auth.SignInitData("999:other", auth.User{ID: 42}, time.Now(), nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, tt.auth)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestAPI_HealthAndStaticNeedNoAuth(t *testing.T) {
	env := newTestEnv(t)

	resp := env.doWithAuth(http.MethodGet, "/healthz", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- /me ------------------------------------------------------------------

func TestMe_ListsPrivateChatAlways(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodGet, "/api/v1/me", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Equal(t, testUserID, body.User.ID)
	require.NotEmpty(t, body.Chats)
	assert.Equal(t, testUserID, body.Chats[0].ID)
	assert.Equal(t, "private", body.Chats[0].Type)
	assert.Equal(t, "Europe/Berlin", body.Chats[0].Timezone)
}

func TestMe_ListsOnlyGroupsWithCurrentMembership(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	require.NoError(t, env.memberUC.Remember(ctx, memberGroupID, testUserID))
	require.NoError(t, env.memberUC.Remember(ctx, foreignGroupID, testUserID))

	resp := env.do(http.MethodGet, "/api/v1/me", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	ids := make([]int64, 0, len(body.Chats))
	for _, chat := range body.Chats {
		ids = append(ids, chat.ID)
	}
	assert.ElementsMatch(t, []int64{testUserID, memberGroupID}, ids)
}

func TestMe_DesktopMigrationThenKickedFreezesNewChat(t *testing.T) {
	const (
		oldChatID = int64(-5342885594)
		newChatID = int64(-1004314310091)
	)
	checker := &desktopMigrationStub{oldChatID: oldChatID, newChatID: newChatID}
	env := newTestEnvWithMembership(t, checker)
	env.seedChat(oldChatID, "group", "Europe/Moscow")

	ctx := context.Background()
	require.NoError(t, env.memberUC.Remember(ctx, oldChatID, testUserID))
	reminder := env.createReminder(oldChatID, "перенести")

	raw := initDataWith(map[string]string{"start_param": "chat_" + itoa(oldChatID)})
	resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Zero(t, body.LaunchChatID)
	require.Len(t, body.Chats, 1)
	assert.Equal(t, testUserID, body.Chats[0].ID)
	assert.EqualValues(t, 2, checker.calls.Load(), "old ID and migrated ID must each be checked once")

	_, err := env.chatUC.Get(ctx, oldChatID)
	assert.ErrorIs(t, err, repository.ErrChatNotFound)
	newChat, err := env.chatUC.Get(ctx, newChatID)
	require.NoError(t, err)
	assert.False(t, newChat.Available)

	moved, err := env.remUC.GetReminder(ctx, reminder.ID)
	require.NoError(t, err)
	assert.Equal(t, newChatID, moved.ChatID)

	// Alias и available=false должны исключить повторение обоих WARN на следующем
	// запросе того же старого Desktop-контекста.
	resp = env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = decode[meResponse](t, resp)
	assert.EqualValues(t, 2, checker.calls.Load())
}

func TestMe_PrefersAuthorizedLaunchGroupFromSignedInitData(t *testing.T) {
	env := newTestEnv(t)
	raw := initDataWith(map[string]string{"start_param": "chat_" + itoa(memberGroupID)})

	resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Equal(t, memberGroupID, body.LaunchChatID)
	require.NotEmpty(t, body.Chats)
	assert.Equal(t, memberGroupID, body.Chats[0].ID,
		"the launch chat must be first for clients that do not read launch_chat_id")
	assert.Contains(t, body.Chats, chatDTO{
		ID:       memberGroupID,
		Type:     chatTypeGroup,
		Title:    "Test",
		Timezone: "Europe/Berlin",
		IsPublic: true,
	})
}

func TestMe_RejectsForeignLaunchGroupFromSignedInitData(t *testing.T) {
	env := newTestEnv(t)
	raw := initDataWith(map[string]string{"start_param": "chat_" + itoa(foreignGroupID)})

	resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Zero(t, body.LaunchChatID)
	for _, chat := range body.Chats {
		assert.NotEqual(t, foreignGroupID, chat.ID)
	}
}

func TestMe_UsesSignedChatContextWithoutStartParam(t *testing.T) {
	env := newTestEnv(t)
	raw := initDataWith(map[string]string{
		"chat_type": "supergroup",
		"chat":      `{"id":-1003333333333,"type":"supergroup","title":"Команда"}`,
	})

	resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Equal(t, secondMemberGroupID, body.LaunchChatID)
	require.NotEmpty(t, body.Chats)
	assert.Equal(t, secondMemberGroupID, body.Chats[0].ID)
}

func TestMe_UsesRecentAppCommandWhenAndroidOmitsStartParam(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	require.NoError(t, env.memberUC.Remember(ctx, memberGroupID, testUserID))
	require.NoError(t, env.memberUC.Remember(ctx, secondMemberGroupID, testUserID))
	require.NoError(t, env.memberUC.RememberWebAppLaunch(ctx, secondMemberGroupID, testUserID))

	raw := initDataWith(map[string]string{"chat_type": "group"})
	resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[meResponse](t, resp)
	assert.Equal(t, secondMemberGroupID, body.LaunchChatID)
	require.NotEmpty(t, body.Chats)
	assert.Equal(t, secondMemberGroupID, body.Chats[0].ID)
}

func TestMe_IgnoresStaleAppCommandAndPrivateLaunch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	require.NoError(t, env.memberUC.Remember(ctx, secondMemberGroupID, testUserID))
	require.NoError(t, env.memberUC.RememberWebAppLaunch(ctx, secondMemberGroupID, testUserID))
	_, err := env.db.Exec(
		`UPDATE webapp_launch_contexts SET launched_at = ? WHERE user_id = ?`,
		time.Now().UTC().Add(-recentWebAppLaunchTTL-time.Minute),
		testUserID,
	)
	require.NoError(t, err)

	for _, chatType := range []string{"group", "private"} {
		raw := initDataWith(map[string]string{"chat_type": chatType})
		resp := env.doWithAuth(http.MethodGet, "/api/v1/me", nil, "tma "+raw)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body := decode[meResponse](t, resp)
		assert.Zero(t, body.LaunchChatID)
		require.NotEmpty(t, body.Chats)
		assert.Equal(t, testUserID, body.Chats[0].ID)
	}
}

// --- Авторизация доступа к чату -------------------------------------------

func TestChatAccess_DeniesForeignChats(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		method string
		suffix string
		body   any
	}{
		{http.MethodGet, "", nil},
		{http.MethodPut, "/timezone", map[string]any{"timezone": "Europe/Moscow"}},
		{http.MethodGet, "/reminders", nil},
		{http.MethodPost, "/reminders", map[string]any{
			"text": "чужое", "repeat": "daily", "time": "10:00",
		}},
	}

	for _, chatID := range []int64{foreignUserID, foreignGroupID} {
		for _, tt := range tests {
			path := "/api/v1/chats/" + itoa(chatID) + tt.suffix
			t.Run(tt.method+" "+path, func(t *testing.T) {
				resp := env.do(tt.method, path, tt.body)
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			})
		}
	}
}

func TestChatAccess_AllowsGroupsWithMembership(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodGet, "/api/v1/chats/"+itoa(memberGroupID)+"/reminders", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Ключевая проверка: чужое напоминание должно быть неотличимо от несуществующего,
// иначе перебором идентификаторов можно выяснить, какие записи есть в базе.
func TestReminderAccess_ForeignReminderLooksMissing(t *testing.T) {
	env := newTestEnv(t)

	tests := []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodPatch, map[string]any{"paused": true}},
		{http.MethodDelete, nil},
	}

	for _, chatID := range []int64{foreignUserID, foreignGroupID} {
		foreign := env.createReminder(chatID, "чужое напоминание")

		for _, tt := range tests {
			t.Run(itoa(chatID)+" "+tt.method, func(t *testing.T) {
				resp := env.do(tt.method, "/api/v1/reminders/"+itoa(foreign.ID), tt.body)
				require.Equal(t, http.StatusNotFound, resp.StatusCode)

				body := decode[errorResponse](t, resp)
				assert.Equal(t, "not_found", body.Code)
			})
		}

		// Напоминание должно уцелеть.
		stored, err := env.remUC.GetReminder(context.Background(), foreign.ID)
		require.NoError(t, err)
		assert.Equal(t, "чужое напоминание", stored.Text)
	}
}

func TestReminderAccess_UnknownIDIsNotFound(t *testing.T) {
	env := newTestEnv(t)

	for _, id := range []string{"999999", "0", "-1", "abc"} {
		t.Run(id, func(t *testing.T) {
			resp := env.do(http.MethodGet, "/api/v1/reminders/"+id, nil)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

// --- CRUD напоминаний -----------------------------------------------------

func TestCreateReminder_Daily(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":   "выпить воды",
		"repeat": "daily",
		"time":   "09:30",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	created := decode[reminderDTO](t, resp)
	assert.Equal(t, "выпить воды", created.Text)
	assert.Equal(t, "daily", created.Repeat)
	assert.False(t, created.Paused)
	assert.True(t, created.NextTime.After(time.Now()), "next time must be in the future")

	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	assert.Equal(t, 9, created.NextTime.In(loc).Hour(), "time must be interpreted in the chat timezone")
	assert.Equal(t, 30, created.NextTime.In(loc).Minute())
}

func TestCreateReminder_WeeklyPicksEarliestDay(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":        "созвон",
		"repeat":      "weekly",
		"time":        "10:00",
		"repeat_days": []int{1, 3, 5},
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	created := decode[reminderDTO](t, resp)
	assert.Equal(t, []int{1, 3, 5}, created.RepeatDays)
	assert.Zero(t, created.RepeatEvery, "repeat_every must stay empty for weekly reminders")

	loc, _ := time.LoadLocation("Europe/Berlin")
	weekday := int(created.NextTime.In(loc).Weekday())
	assert.Contains(t, []int{1, 3, 5}, weekday)
}

// Дата старта в прошлом — законный способ попасть в нужный цикл; напоминание
// при этом не должно срабатывать сразу после сохранения.
func TestCreateReminder_EveryNDaysWithPastStartDate(t *testing.T) {
	env := newTestEnv(t)

	past := time.Now().AddDate(0, 0, -30).Format("02.01.2006")

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":         "полить цветы",
		"repeat":       "every_n_days",
		"time":         "08:00",
		"repeat_every": 7,
		"date":         past,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	created := decode[reminderDTO](t, resp)
	assert.True(t, created.NextTime.After(time.Now()), "next time must be in the future, got %s", created.NextTime)

	loc, _ := time.LoadLocation("Europe/Berlin")
	assert.Equal(t, 8, created.NextTime.In(loc).Hour())
}

func TestCreateReminder_RejectsInvalidInput(t *testing.T) {
	env := newTestEnv(t)
	path := "/api/v1/chats/" + itoa(testUserID) + "/reminders"

	tests := []struct {
		name string
		body map[string]any
	}{
		{"нет текста", map[string]any{"repeat": "daily", "time": "09:00"}},
		{"нет типа повтора", map[string]any{"text": "привет", "time": "09:00"}},
		{"неизвестный повтор", map[string]any{"text": "привет", "repeat": "hourly", "time": "09:00"}},
		{"кривое время", map[string]any{"text": "привет", "repeat": "daily", "time": "25:99"}},
		{"еженедельно без дней", map[string]any{"text": "привет", "repeat": "weekly", "time": "09:00"}},
		{"ежемесячно без числа", map[string]any{"text": "привет", "repeat": "monthly", "time": "09:00"}},
		{"разовое без даты", map[string]any{"text": "привет", "repeat": "none", "time": "09:00"}},
		{
			"каждые N дней с нулевым интервалом",
			map[string]any{"text": "привет", "repeat": "every_n_days", "time": "09:00", "repeat_every": 0},
		},
		{"слишком длинный текст", map[string]any{
			"text": strings.Repeat("я", domain.MaxTextLen+1), "repeat": "daily", "time": "09:00",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := env.do(http.MethodPost, path, tt.body)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestUpdateReminder_PauseKeepsSchedule(t *testing.T) {
	env := newTestEnv(t)
	rem := env.createReminder(testUserID, "старый текст")
	before := rem.NextTime

	resp := env.do(http.MethodPatch, "/api/v1/reminders/"+itoa(rem.ID), map[string]any{"paused": true})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	updated := decode[reminderDTO](t, resp)
	assert.True(t, updated.Paused)
	// Пауза не должна сдвигать ближайшее срабатывание.
	assert.True(t, before.Equal(updated.NextTime), "want %s, got %s", before, updated.NextTime)
}

func TestUpdateReminder_ChangesTextAndTime(t *testing.T) {
	env := newTestEnv(t)
	rem := env.createReminder(testUserID, "старый текст")

	resp := env.do(http.MethodPatch, "/api/v1/reminders/"+itoa(rem.ID), map[string]any{
		"text": "новый текст",
		"time": "07:15",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	updated := decode[reminderDTO](t, resp)
	assert.Equal(t, "новый текст", updated.Text)

	loc, _ := time.LoadLocation("Europe/Berlin")
	assert.Equal(t, 7, updated.NextTime.In(loc).Hour())
	assert.Equal(t, 15, updated.NextTime.In(loc).Minute())
}

func TestDeleteReminder(t *testing.T) {
	env := newTestEnv(t)
	rem := env.createReminder(testUserID, "удалить меня")

	resp := env.do(http.MethodDelete, "/api/v1/reminders/"+itoa(rem.ID), nil)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, err := env.remUC.GetReminder(context.Background(), rem.ID)
	assert.ErrorIs(t, err, repository.ErrReminderNotFound)
}

func TestListReminders_ReturnsChatTimezone(t *testing.T) {
	env := newTestEnv(t)
	env.createReminder(testUserID, "первое")
	env.createReminder(testUserID, "второе")

	resp := env.do(http.MethodGet, "/api/v1/chats/"+itoa(testUserID)+"/reminders", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := decode[reminderListResponse](t, resp)
	assert.Equal(t, "Europe/Berlin", body.Timezone)
	assert.Len(t, body.Reminders, 2)
}

// --- Часовой пояс ---------------------------------------------------------

func TestSetTimezone(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPut, "/api/v1/chats/"+itoa(testUserID)+"/timezone",
		map[string]any{"timezone": "Asia/Tokyo"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	chat, err := env.chatUC.Get(context.Background(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, "Asia/Tokyo", chat.Timezone)
}

func TestSetTimezone_RejectsUnknownZone(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPut, "/api/v1/chats/"+itoa(testUserID)+"/timezone",
		map[string]any{"timezone": "Mars/Olympus_Mons"})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body := decode[errorResponse](t, resp)
	assert.Equal(t, "invalid_timezone", body.Code)
}

// --- Лимиты ---------------------------------------------------------------

func TestCreateReminder_EnforcesPerChatLimit(t *testing.T) {
	env := newTestEnv(t)

	for range domain.MaxRemindersPerChat {
		env.createReminder(testUserID, "заполнитель")
	}

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":   "лишнее",
		"repeat": "daily",
		"time":   "09:00",
	})
	require.Equal(t, http.StatusConflict, resp.StatusCode)

	body := decode[errorResponse](t, resp)
	assert.Equal(t, "too_many_reminders", body.Code)
}

func TestRequest_RejectsOversizedBody(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":   strings.Repeat("a", 100_000),
		"repeat": "daily",
		"time":   "09:00",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_RejectsUnknownFields(t *testing.T) {
	env := newTestEnv(t)

	resp := env.do(http.MethodPost, "/api/v1/chats/"+itoa(testUserID)+"/reminders", map[string]any{
		"text":     "привет",
		"repeat":   "daily",
		"time":     "09:00",
		"chat_id":  -1,
		"is_admin": true,
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Заголовки ------------------------------------------------------------

func TestSecurityHeaders(t *testing.T) {
	env := newTestEnv(t)

	resp := env.doWithAuth(http.MethodGet, "/healthz", nil, "")

	csp := resp.Header.Get("Content-Security-Policy")
	assert.Contains(t, csp, "script-src 'self' https://telegram.org")
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
