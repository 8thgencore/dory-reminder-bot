//revive:disable

package texts

// Все тексты, отправляемые пользователю, вынесены сюда.

const (
	WelcomeTextNoTZ = "Привет! Я бот-напоминалка. 🌍\n\nСначала установите ваш часовой пояс командой /timezone"
	WelcomeText     = `🤖 *Dory Reminder Bot*

Привет! Я бот для создания и управления напоминаниями.`
	HelpText = `*Справка по командам:*

/help - справка по командам
/add - добавить напоминание
/list - список напоминаний
/edit - редактировать напоминание
/delete - удалить напоминание
/pause - поставить на паузу
/resume - возобновить напоминание
/timezone - установить часовой пояс
/app - открыть приложение`
	SetTimezonePrompt = "🌍 Введите ваш часовой пояс в формате IANA (например, Europe/Moscow, " +
		"America/New_York, Asia/Tokyo):"
	UnknownTimezone = "❌ Неизвестный или невалидный часовой пояс. Введите в формате IANA, например: " +
		"Europe/Moscow, America/New_York, Asia/Tokyo. Список поддерживаемых: " +
		"https://en.wikipedia.org/wiki/List_of_tz_database_time_zones"
	HelpMainMenu = "🤖 *Dory Reminder Bot*\n\nПривет! Я бот для создания и управления напоминаниями.\n\n" +
		"Выберите раздел справки:"

	// GroupMentionHint дописывается к вопросам мастера в групповых чатах.
	GroupMentionHint = "\n\nЧтобы бот увидел ваш ответ, добавьте в конце @"

	// Сообщения о результате операций.
	TimezoneSet       = "✅ Часовой пояс успешно установлен: "
	ReminderCreated   = "Напоминание создано!"
	ReminderUpdated   = "Напоминание обновлено!"
	ReminderDeleted   = "🗑️ Напоминание удалено!"
	ReminderPaused    = "⏸️ Напоминание поставлено на паузу!"
	ReminderResumed   = "▶️ Напоминание возобновлено!"
	RemindersHeader   = "📋 *Ваши напоминания*"
	ReminderPrefix    = "⏰ Напоминание: "
	TimezoneRequired  = "⚠️ Сначала установите часовой пояс командой /timezone"
	AddViaWizardOnly  = "Для создания напоминания используйте мастер через /add без параметров."
	EditUsage         = "Формат: /edit <номер> <новый текст> или /edit <номер> <время> <новый текст>"
	EditTimeFormat    = "Время должно быть в формате 15:00"
	WebAppUnavailable = "Веб-приложение сейчас недоступно. Используйте команды бота: /add, /list."
	WebAppOpenPrivate = "Управляйте напоминаниями в удобном интерфейсе:"
	WebAppOpenGroup   = "Управляйте напоминаниями этого чата:"
	WebAppUsePrivate  = "Откройте приложение в личном чате со мной — там можно выбрать этот чат в списке."
	WebAppButton      = "📱 Открыть приложение"
)

// Функции для генерации динамических текстов можно добавить ниже.
