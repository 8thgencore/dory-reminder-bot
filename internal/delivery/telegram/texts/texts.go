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
/timezone - установить часовой пояс`
	AddTypePrompt     = "Выберите тип напоминания:"
	SetTimezonePrompt = "🌍 Введите ваш часовой пояс в формате IANA (например, Europe/Moscow, America/New_York, Asia/Tokyo):"
	UnknownTimezone   = "❌ Неизвестный или невалидный часовой пояс. Введите в формате IANA, например: Europe/Moscow, America/New_York, Asia/Tokyo. Список поддерживаемых: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones"
	HelpMainMenu      = "🤖 *Dory Reminder Bot*\n\nПривет! Я бот для создания и управления напоминаниями.\n\nВыберите раздел справки:"
)

// Функции для генерации динамических текстов можно добавить ниже.
