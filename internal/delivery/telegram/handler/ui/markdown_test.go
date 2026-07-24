package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeMarkdownV2(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"обычный текст", "позвонить маме", "позвонить маме"},
		{"точка", "Встреча в 15.00", `Встреча в 15\.00`},
		{"дефис и восклицание", "купить хлеб-батон!", `купить хлеб\-батон\!`},
		{"попытка выделения", "*важно*", `\*важно\*`},
		{"скобки и ссылка", "[тык](http://a.b)", `\[тык\]\(http://a\.b\)`},
		{"обратный слэш", `путь\к\файлу`, `путь\\к\\файлу`},
		{"вертикальная черта", "а|б", `а\|б`},
		{"пустая строка", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EscapeMarkdownV2(tt.input))
		})
	}
}

// Кириллица и эмодзи не должны трогаться: экранируется только ASCII из набора
// спецсимволов MarkdownV2.
func TestEscapeMarkdownV2_LeavesUnicodeIntact(t *testing.T) {
	input := "Напоминание 🕐 про встречу"
	assert.Equal(t, input, EscapeMarkdownV2(input))
}
