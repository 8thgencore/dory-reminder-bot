package wizards

import (
	"github.com/8thgencore/dory-reminder-bot/internal/delivery/telegram/handler/texts"
	tele "gopkg.in/telebot.v4"
)

// withGroupHint дописывает к вопросу подсказку про упоминание бота.
//
// В группах бот видит только ответы на свои сообщения и сообщения с упоминанием
// (см. Handler.onText), поэтому без подсказки пошаговый мастер выглядит зависшим.
func withGroupHint(c tele.Context, botName, msg string) string {
	if c.Chat().Type == tele.ChatPrivate || msg == texts.PromptUnknown {
		return msg
	}

	return msg + texts.GroupMentionHint + botName
}
