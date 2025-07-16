package handler

import tele "gopkg.in/telebot.v4"

var (
	addMenu     = &tele.ReplyMarkup{}
	mainMenu    = &tele.ReplyMarkup{}
	btnToday    = addMenu.Data("Сегодня", "add_today")
	btnTomorrow = addMenu.Data("Завтра", "add_tomorrow")
	btnMultiDay = addMenu.Data("Несколько раз в день", "add_multiday")
	btnEveryDay = addMenu.Data("Ежедневно", "add_everyday")
	btnWeek     = addMenu.Data("Раз в неделю", "add_week")
	btnNDays    = addMenu.Data("Раз в несколько дней", "add_ndays")
	btnMonth    = addMenu.Data("Раз в месяц", "add_month")
	btnYear     = addMenu.Data("Раз в год", "add_year")
	btnDate     = addMenu.Data("Выбрать дату", "add_date")

	// Help menu buttons
	btnHelpAdd    = mainMenu.Data("➕ Добавить напоминание", "help_add")
	btnHelpList   = mainMenu.Data("📋 Список напоминаний", "help_list")
	btnHelpManage = mainMenu.Data("⚙️ Управление", "help_manage")
)

func init() {
	addMenu.Inline(
		addMenu.Row(btnToday, btnTomorrow),
		addMenu.Row(btnMultiDay, btnEveryDay),
		addMenu.Row(btnWeek, btnNDays),
		addMenu.Row(btnMonth, btnYear),
		addMenu.Row(btnDate),
	)

	mainMenu.Inline(
		mainMenu.Row(btnHelpAdd),
		mainMenu.Row(btnHelpList),
		mainMenu.Row(btnHelpManage),
	)
}

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
