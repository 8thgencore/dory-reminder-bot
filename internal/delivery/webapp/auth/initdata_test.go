package auth

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:AAHtestTokenValueForUnitTests"

func testUser() User {
	return User{ID: 42, FirstName: "Дарья", Username: "dory", LanguageCode: "ru"}
}

func TestValidate_AcceptsGenuineInitData(t *testing.T) {
	now := time.Now()
	raw := SignInitData(testToken, testUser(), now, map[string]string{
		"chat":          `{"id":-100777,"type":"supergroup","title":"Команда","username":"team"}`,
		"chat_type":     "private",
		"chat_instance": "-1234567890",
		"start_param":   "chat_-100500",
		"query_id":      "AAF123",
	})

	got, err := NewValidator(testToken, time.Hour).Validate(raw)
	require.NoError(t, err)

	assert.Equal(t, int64(42), got.User.ID)
	assert.Equal(t, "dory", got.User.Username)
	require.NotNil(t, got.Chat)
	assert.Equal(t, int64(-100777), got.Chat.ID)
	assert.Equal(t, "Команда", got.Chat.Title)
	assert.Equal(t, "private", got.ChatType)
	assert.Equal(t, "chat_-100500", got.StartParam)
	assert.Equal(t, "AAF123", got.QueryID)
	assert.Equal(t, now.Unix(), got.AuthDate.Unix())
}

func TestValidate_RejectsMalformedSignedChat(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now(), map[string]string{
		"chat": `{"id":"broken"}`,
	})

	_, err := NewValidator(testToken, time.Hour).Validate(raw)
	assert.ErrorIs(t, err, ErrMalformedInitData)
}

// Telegram добавляет поле signature для сторонней проверки; при проверке HMAC
// оно входит в подписываемые данные наравне с остальными полями.
func TestValidate_AcceptsSignedSignatureField(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now(), map[string]string{
		"signature": "3A2ImDLFakeEd25519Signature",
	})

	got, err := NewValidator(testToken, time.Hour).Validate(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(42), got.User.ID)
}

func TestValidate_RejectsTamperedSignatureField(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now(), map[string]string{
		"signature": "3A2ImDLOriginalEd25519Signature",
	})
	values, err := url.ParseQuery(raw)
	require.NoError(t, err)
	values.Set("signature", "3A2ImDLTamperedEd25519Signature")

	_, err = NewValidator(testToken, time.Hour).Validate(values.Encode())
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestValidate_RejectsTamperedPayload(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now(), nil)

	values, err := url.ParseQuery(raw)
	require.NoError(t, err)

	// Подмена пользователя на чужой ID — ровно то, чем был бы захват чужих напоминаний.
	values.Set("user", `{"id":999999,"first_name":"Mallory"}`)

	_, err = NewValidator(testToken, time.Hour).Validate(values.Encode())
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestValidate_RejectsForgedHash(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now(), nil)
	values, err := url.ParseQuery(raw)
	require.NoError(t, err)
	values.Set("hash", strings.Repeat("a", 64))

	_, err = NewValidator(testToken, time.Hour).Validate(values.Encode())
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestValidate_RejectsOtherBotsToken(t *testing.T) {
	raw := SignInitData("999:otherBotToken", testUser(), time.Now(), nil)

	_, err := NewValidator(testToken, time.Hour).Validate(raw)
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestValidate_RejectsExpiredInitData(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now().Add(-25*time.Hour), nil)

	_, err := NewValidator(testToken, 24*time.Hour).Validate(raw)
	assert.ErrorIs(t, err, ErrExpired)
}

func TestValidate_ZeroTTLDisablesExpiry(t *testing.T) {
	raw := SignInitData(testToken, testUser(), time.Now().Add(-365*24*time.Hour), nil)

	_, err := NewValidator(testToken, 0).Validate(raw)
	assert.NoError(t, err)
}

func TestValidate_RejectsMalformedInput(t *testing.T) {
	v := NewValidator(testToken, time.Hour)

	tests := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"пустая строка", "", ErrMissingInitData},
		{"только пробелы", "   ", ErrMissingInitData},
		{"нет hash", "user=%7B%22id%22%3A1%7D&auth_date=1700000000", ErrMissingHash},
		{"мусор", "%%%", ErrMalformedInitData},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Validate(tt.raw)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidate_RejectsMissingUser(t *testing.T) {
	// Подписываем валидный набор полей, но без пользователя: initData из канала
	// не содержит user, а работать от чьего-то имени мы обязаны.
	values := url.Values{}
	values.Set("auth_date", "1700000000")
	values.Set("chat_type", "channel")

	v := NewValidator(testToken, 0)
	values.Set("hash", v.sign(values))

	_, err := v.Validate(values.Encode())
	assert.ErrorIs(t, err, ErrMissingUser)
}

func TestValidate_RejectsBadAuthDate(t *testing.T) {
	values := url.Values{}
	values.Set("user", `{"id":42}`)
	values.Set("auth_date", "not-a-number")

	v := NewValidator(testToken, time.Hour)
	values.Set("hash", v.sign(values))

	_, err := v.Validate(values.Encode())
	assert.ErrorIs(t, err, ErrMalformedInitData)
}
