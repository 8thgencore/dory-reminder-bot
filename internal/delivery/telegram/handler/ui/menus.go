package ui

import tele "gopkg.in/telebot.v4"

var (
	AddMenu     = &tele.ReplyMarkup{}
	MainMenu    = &tele.ReplyMarkup{}
	btnToday    = AddMenu.Data("Сегодня", "add_today")
	btnTomorrow = AddMenu.Data("Завтра", "add_tomorrow")
	btnMultiDay = AddMenu.Data("Несколько раз в день", "add_multiday")
	btnEveryDay = AddMenu.Data("Ежедневно", "add_everyday")
	btnWeek     = AddMenu.Data("Раз в неделю", "add_week")
	btnNDays    = AddMenu.Data("Раз в несколько дней", "add_ndays")
	btnMonth    = AddMenu.Data("Раз в месяц", "add_month")
	btnYear     = AddMenu.Data("Раз в год", "add_year")
	btnDate     = AddMenu.Data("Выбрать дату", "add_date")

	// Help menu buttons
	btnHelpAdd    = MainMenu.Data("➕ Добавить напоминание", "help_add")
	btnHelpList   = MainMenu.Data("📋 Список напоминаний", "help_list")
	btnHelpManage = MainMenu.Data("⚙️ Управление", "help_manage")
)

func init() {
	AddMenu.Inline(
		AddMenu.Row(btnToday, btnTomorrow),
		AddMenu.Row(btnMultiDay, btnEveryDay),
		AddMenu.Row(btnWeek, btnNDays),
		AddMenu.Row(btnMonth, btnYear),
		AddMenu.Row(btnDate),
	)

	MainMenu.Inline(
		MainMenu.Row(btnHelpAdd),
		MainMenu.Row(btnHelpList),
		MainMenu.Row(btnHelpManage),
	)
}

// GetMainMenu возвращает главное меню бота
func GetMainMenu() *tele.ReplyMarkup {
	return MainMenu
}

// GetAddMenu возвращает меню добавления напоминаний
func GetAddMenu() *tele.ReplyMarkup {
	return AddMenu
}

// WeekdaysMenu возвращает inline-меню для выбора дня недели
func WeekdaysMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	btnMonday := m.Data("Пн", "weekday_1")
	btnTuesday := m.Data("Вт", "weekday_2")
	btnWednesday := m.Data("Ср", "weekday_3")
	btnThursday := m.Data("Чт", "weekday_4")
	btnFriday := m.Data("Пт", "weekday_5")
	btnSaturday := m.Data("Сб", "weekday_6")
	btnSunday := m.Data("Вс", "weekday_0")
	m.Inline(
		m.Row(btnMonday, btnTuesday, btnWednesday, btnThursday, btnFriday, btnSaturday, btnSunday),
	)

	return m
}

// Кнопки для обработчиков
var (
	BtnToday    = &btnToday
	BtnTomorrow = &btnTomorrow
	BtnMultiDay = &btnMultiDay
	BtnEveryDay = &btnEveryDay
	BtnWeek     = &btnWeek
	BtnNDays    = &btnNDays
	BtnMonth    = &btnMonth
	BtnYear     = &btnYear
	BtnDate     = &btnDate

	BtnHelpAdd    = &btnHelpAdd
	BtnHelpList   = &btnHelpList
	BtnHelpManage = &btnHelpManage
)
