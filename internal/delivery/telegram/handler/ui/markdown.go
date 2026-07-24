package ui

import "strings"

// escapedV2Chars — символы, которые Telegram требует экранировать в MarkdownV2.
//
// Набор задан спецификацией Bot API и должен совпадать с режимом разбора, указанным
// при отправке. Раньше здесь экранировался этот же набор, а сообщения уходили
// в legacy-режиме Markdown, где `\.` и `\!` не считаются экранированием, — и
// пользователь видел в списке буквальные обратные слэши.
const escapedV2Chars = `_*[]()~` + "`" + `>#+-=|{}.!\`

// EscapeMarkdownV2 экранирует специальные символы MarkdownV2.
//
// Применять обязательно ко всему, что пришло от пользователя: без этого текст
// напоминания может сломать разметку сообщения или подделать её.
func EscapeMarkdownV2(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if r < 128 && strings.ContainsRune(escapedV2Chars, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}

	return b.String()
}
