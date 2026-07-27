package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// SignInitData собирает строку initData и подписывает её тем же алгоритмом, что
// использует Telegram.
//
// Предназначена для тестов и для ручной проверки API через curl — в рабочем коде
// подписывает только Telegram. Дополнительные поля (chat_type, start_param, signature)
// передаются через extra.
func SignInitData(botToken string, user User, authDate time.Time, extra map[string]string) string {
	encodedUser, err := json.Marshal(user)
	if err != nil {
		// User состоит из строк, чисел и bool — сериализация не может провалиться.
		panic("auth: failed to marshal user: " + err.Error())
	}

	values := url.Values{}
	values.Set("user", string(encodedUser))
	values.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	for k, v := range extra {
		values.Set(k, v)
	}

	secretMac := hmac.New(sha256.New, []byte(secretKeySalt))
	secretMac.Write([]byte(botToken))

	mac := hmac.New(sha256.New, secretMac.Sum(nil))
	mac.Write([]byte(dataCheckString(values)))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))

	return values.Encode()
}
