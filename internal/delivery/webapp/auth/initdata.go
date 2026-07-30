// Package auth проверяет подлинность initData, который Telegram передаёт Mini App.
//
// Это единственная граница доверия HTTP-слоя: всё, что приходит от клиента, считается
// подделкой, пока подпись не сошлась.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ошибки проверки initData.
var (
	// ErrMissingInitData возвращается для пустой строки.
	ErrMissingInitData = errors.New("init data is missing")
	// ErrMalformedInitData возвращается, если строка не разбирается как query-string.
	ErrMalformedInitData = errors.New("init data is malformed")
	// ErrMissingHash возвращается, если в initData нет поля hash.
	ErrMissingHash = errors.New("init data has no hash")
	// ErrInvalidHash возвращается при несовпадении подписи.
	ErrInvalidHash = errors.New("init data signature does not match")
	// ErrExpired возвращается, если initData старше допустимого срока.
	ErrExpired = errors.New("init data has expired")
	// ErrMissingUser возвращается, если в initData нет пользователя.
	ErrMissingUser = errors.New("init data has no user")
)

// secretKeySalt — фиксированная соль из спецификации Telegram.
const secretKeySalt = "WebAppData"

// User описывает пользователя Telegram из initData.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	PhotoURL     string `json:"photo_url"`
	IsPremium    bool   `json:"is_premium"`
}

// InitData содержит проверенные данные запуска Mini App.
type InitData struct {
	User         User
	ChatType     string
	ChatInstance string
	StartParam   string
	AuthDate     time.Time
	QueryID      string
}

// Validator проверяет подпись initData для конкретного бота.
type Validator struct {
	// secret — HMAC-ключ, выведенный из токена бота. Сам токен не сохраняется.
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewValidator создает валидатор для указанного токена бота.
//
// ttl ограничивает возраст initData: без этого перехваченная строка работала бы вечно.
func NewValidator(botToken string, ttl time.Duration) *Validator {
	mac := hmac.New(sha256.New, []byte(secretKeySalt))
	mac.Write([]byte(botToken))

	return &Validator{secret: mac.Sum(nil), ttl: ttl, now: time.Now}
}

// Validate проверяет подпись и срок годности initData и возвращает его содержимое.
func (v *Validator) Validate(raw string) (*InitData, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrMissingInitData
	}

	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedInitData, err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return nil, ErrMissingHash
	}

	if !hmac.Equal([]byte(hash), []byte(v.sign(values))) {
		return nil, ErrInvalidHash
	}

	authDate, err := parseAuthDate(values.Get("auth_date"))
	if err != nil {
		return nil, err
	}
	if v.ttl > 0 && v.now().Sub(authDate) > v.ttl {
		return nil, fmt.Errorf("%w: issued at %s", ErrExpired, authDate.UTC().Format(time.RFC3339))
	}

	var user User
	rawUser := values.Get("user")
	if rawUser == "" {
		return nil, ErrMissingUser
	}
	if err := json.Unmarshal([]byte(rawUser), &user); err != nil {
		return nil, fmt.Errorf("%w: user is not valid JSON: %v", ErrMalformedInitData, err)
	}
	if user.ID == 0 {
		return nil, ErrMissingUser
	}

	return &InitData{
		User:         user,
		ChatType:     values.Get("chat_type"),
		ChatInstance: values.Get("chat_instance"),
		StartParam:   values.Get("start_param"),
		AuthDate:     authDate,
		QueryID:      values.Get("query_id"),
	}, nil
}

// sign вычисляет ожидаемую подпись для набора полей.
func (v *Validator) sign(values url.Values) string {
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(dataCheckString(values)))

	return hex.EncodeToString(mac.Sum(nil))
}

// dataCheckString собирает строку проверки: все поля, кроме hash,
// отсортированные по ключу и склеенные переводом строки.
//
// Telegram исключает signature только при сторонней Ed25519-проверке. При
// проверке HMAC с токеном бота signature входит в строку наравне с остальными
// полями.
func dataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+values.Get(k))
	}

	return strings.Join(pairs, "\n")
}

func parseAuthDate(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: auth_date is missing", ErrMalformedInitData)
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: auth_date is not a unix timestamp", ErrMalformedInitData)
	}

	return time.Unix(seconds, 0), nil
}
