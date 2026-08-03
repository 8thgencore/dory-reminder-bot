package repository

import "github.com/8thgencore/dory-reminder-bot/internal/domain"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReminder(scanner rowScanner) (*domain.Reminder, error) {
	var reminder domain.Reminder
	var repeatDays string

	if err := scanner.Scan(
		&reminder.ID,
		&reminder.ChatID,
		&reminder.Text,
		&reminder.NextTime,
		&reminder.Repeat,
		&repeatDays,
		&reminder.RepeatEvery,
		&reminder.Paused,
		&reminder.CreatedAt,
		&reminder.UpdatedAt,
	); err != nil {
		return nil, err
	}

	reminder.RepeatDays = deserializeRepeatDays(repeatDays)

	return &reminder, nil
}

func scanChat(scanner rowScanner) (*domain.Chat, error) {
	var chat domain.Chat
	if err := scanner.Scan(
		&chat.ID,
		&chat.Type,
		&chat.Name,
		&chat.Username,
		&chat.Timezone,
		&chat.Available,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &chat, nil
}
