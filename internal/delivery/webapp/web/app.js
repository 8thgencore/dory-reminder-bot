/*
 * Клиент Telegram Mini App для управления напоминаниями.
 *
 * Аутентификация: initData из Telegram уходит в заголовке Authorization при каждом
 * запросе; сервер проверяет подпись. Никаких токенов на клиенте не хранится.
 */
'use strict';

const tg = window.Telegram && window.Telegram.WebApp;

const WEEKDAYS = [
  { value: 1, short: 'Пн' },
  { value: 2, short: 'Вт' },
  { value: 3, short: 'Ср' },
  { value: 4, short: 'Чт' },
  { value: 5, short: 'Пт' },
  { value: 6, short: 'Сб' },
  { value: 0, short: 'Вс' },
];

const REPEAT_LABELS = {
  none: 'один раз',
  daily: 'каждый день',
  weekly: 'по дням недели',
  monthly: 'раз в месяц',
  every_n_days: 'каждые N дней',
  yearly: 'раз в год',
};

/** Текущее состояние приложения. */
const state = {
  view: 'list',
  chats: [],
  chatId: null,
  timezone: '',
  reminders: [],
  editing: null,
  selectedWeekdays: new Set(),
};

const $ = (id) => document.getElementById(id);

// --- Слой доступа к API ---------------------------------------------------

/**
 * Обёртка над fetch: подставляет initData и разворачивает ошибки API
 * в осмысленные сообщения.
 */
async function api(path, options = {}) {
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `tma ${tg ? tg.initData : ''}`,
      ...(options.headers || {}),
    },
  });

  if (response.status === 204) {
    return null;
  }

  let body = null;
  try {
    body = await response.json();
  } catch {
    body = null;
  }

  if (!response.ok) {
    const message = (body && body.message) || `Ошибка ${response.status}`;
    const error = new Error(message);
    error.code = body && body.code;
    throw error;
  }

  return body;
}

// --- Форматирование -------------------------------------------------------

/** Форматирует момент времени в часовом поясе чата. */
function formatDateTime(iso, timezone) {
  const date = new Date(iso);
  const options = {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  };
  if (timezone) {
    options.timeZone = timezone;
  }

  try {
    return new Intl.DateTimeFormat('ru-RU', options).format(date);
  } catch {
    // Неизвестный часовой пояс не должен ломать весь список.
    delete options.timeZone;
    return new Intl.DateTimeFormat('ru-RU', options).format(date);
  }
}

/** Собирает человекочитаемое описание повтора. */
function describeRepeat(reminder) {
  switch (reminder.repeat) {
    case 'weekly': {
      const names = (reminder.repeat_days || [])
        .map((d) => (WEEKDAYS.find((w) => w.value === d) || {}).short)
        .filter(Boolean);
      return names.length ? `еженедельно: ${names.join(', ')}` : 'еженедельно';
    }
    case 'monthly': {
      const day = (reminder.repeat_days || [])[0];
      return day ? `ежемесячно, ${day}-го числа` : 'ежемесячно';
    }
    case 'every_n_days':
      return `каждые ${reminder.repeat_every} дн.`;
    default:
      return REPEAT_LABELS[reminder.repeat] || reminder.repeat;
  }
}

/** Возвращает ЧЧ:ММ в часовом поясе чата — для предзаполнения формы. */
function timeInZone(iso, timezone) {
  const options = { hour: '2-digit', minute: '2-digit', hour12: false };
  if (timezone) {
    options.timeZone = timezone;
  }
  try {
    return new Intl.DateTimeFormat('ru-RU', options).format(new Date(iso));
  } catch {
    delete options.timeZone;
    return new Intl.DateTimeFormat('ru-RU', options).format(new Date(iso));
  }
}

// --- Навигация ------------------------------------------------------------

function showView(view) {
  state.view = view;
  $('view-list').hidden = view !== 'list';
  $('view-form').hidden = view !== 'form';
  $('view-settings').hidden = view !== 'settings';

  if (!tg) {
    return;
  }

  if (view === 'list') {
    tg.BackButton.hide();
    tg.MainButton.setText('Добавить напоминание');
  } else {
    tg.BackButton.show();
    tg.MainButton.setText('Сохранить');
  }
  tg.MainButton.show();
}

function haptic(type) {
  if (tg && tg.HapticFeedback) {
    tg.HapticFeedback.notificationOccurred(type);
  }
}

// --- Экран списка ---------------------------------------------------------

function renderList() {
  const list = $('reminders');
  list.textContent = '';

  $('empty-state').hidden = state.reminders.length > 0;

  const tzHint = $('tz-hint');
  if (state.timezone) {
    tzHint.textContent = `Часовой пояс: ${state.timezone}`;
    tzHint.hidden = false;
  } else {
    tzHint.textContent = 'Часовой пояс не задан — откройте настройки.';
    tzHint.hidden = false;
  }

  for (const reminder of state.reminders) {
    list.appendChild(renderReminder(reminder));
  }
}

function renderReminder(reminder) {
  const item = document.createElement('li');
  item.className = reminder.paused ? 'reminder reminder--paused' : 'reminder';

  const text = document.createElement('p');
  text.className = 'reminder__text';
  // textContent, а не innerHTML: текст напоминания приходит от пользователя.
  text.textContent = reminder.text;
  if (reminder.paused) {
    const badge = document.createElement('span');
    badge.className = 'badge';
    badge.textContent = 'на паузе';
    text.appendChild(badge);
  }
  item.appendChild(text);

  const meta = document.createElement('p');
  meta.className = 'reminder__meta';
  meta.textContent = `${formatDateTime(reminder.next_time, state.timezone)} · ${describeRepeat(reminder)}`;
  item.appendChild(meta);

  const actions = document.createElement('div');
  actions.className = 'reminder__actions';
  actions.appendChild(makeButton('Изменить', () => openForm(reminder)));
  actions.appendChild(
    makeButton(reminder.paused ? 'Возобновить' : 'Пауза', () => togglePause(reminder)),
  );
  actions.appendChild(makeButton('Удалить', () => confirmDelete(reminder), 'danger'));
  item.appendChild(actions);

  return item;
}

function makeButton(label, onClick, className) {
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = label;
  if (className) {
    button.className = className;
  }
  button.addEventListener('click', onClick);

  return button;
}

async function togglePause(reminder) {
  try {
    await api(`/reminders/${reminder.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ paused: !reminder.paused }),
    });
    haptic('success');
    await loadReminders();
  } catch (error) {
    haptic('error');
    showAlert(error.message);
  }
}

function confirmDelete(reminder) {
  const remove = async () => {
    try {
      await api(`/reminders/${reminder.id}`, { method: 'DELETE' });
      haptic('success');
      await loadReminders();
    } catch (error) {
      haptic('error');
      showAlert(error.message);
    }
  };

  if (tg && tg.showConfirm) {
    tg.showConfirm('Удалить напоминание?', (confirmed) => {
      if (confirmed) {
        remove();
      }
    });
    return;
  }

  if (window.confirm('Удалить напоминание?')) {
    remove();
  }
}

function showAlert(message) {
  if (tg && tg.showAlert) {
    tg.showAlert(message);
    return;
  }
  window.alert(message);
}

// --- Форма ----------------------------------------------------------------

function buildWeekdayButtons() {
  const container = $('field-weekdays');
  container.textContent = '';

  for (const day of WEEKDAYS) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'weekday';
    button.textContent = day.short;
    button.setAttribute('aria-pressed', 'false');
    button.addEventListener('click', () => {
      if (state.selectedWeekdays.has(day.value)) {
        state.selectedWeekdays.delete(day.value);
        button.setAttribute('aria-pressed', 'false');
      } else {
        state.selectedWeekdays.add(day.value);
        button.setAttribute('aria-pressed', 'true');
      }
    });
    container.appendChild(button);
  }
}

function syncWeekdayButtons() {
  const buttons = $('field-weekdays').querySelectorAll('.weekday');
  buttons.forEach((button, index) => {
    const selected = state.selectedWeekdays.has(WEEKDAYS[index].value);
    button.setAttribute('aria-pressed', selected ? 'true' : 'false');
  });
}

/** Показывает поля, относящиеся к выбранному типу повтора. */
function syncFormFields() {
  const repeat = $('field-repeat').value;

  $('field-weekdays-wrap').hidden = repeat !== 'weekly';
  $('field-monthday-wrap').hidden = repeat !== 'monthly';
  $('field-every-wrap').hidden = repeat !== 'every_n_days';

  const needsDate = repeat === 'none' || repeat === 'yearly' || repeat === 'every_n_days';
  $('field-date-wrap').hidden = !needsDate;

  if (repeat === 'yearly') {
    $('field-date-label').textContent = 'Дата (год не важен)';
  } else if (repeat === 'every_n_days') {
    $('field-date-label').textContent = 'Дата начала (необязательно)';
  } else {
    $('field-date-label').textContent = 'Дата';
  }
}

function openForm(reminder) {
  state.editing = reminder || null;
  $('form-error').hidden = true;

  const text = $('field-text');
  const repeat = $('field-repeat');
  const time = $('field-time');

  state.selectedWeekdays = new Set();

  if (reminder) {
    text.value = reminder.text;
    repeat.value = reminder.repeat;
    time.value = timeInZone(reminder.next_time, state.timezone);

    if (reminder.repeat === 'weekly') {
      state.selectedWeekdays = new Set(reminder.repeat_days || []);
    }
    if (reminder.repeat === 'monthly') {
      $('field-monthday').value = (reminder.repeat_days || [])[0] || '';
    }
    if (reminder.repeat === 'every_n_days') {
      $('field-every').value = reminder.repeat_every || '';
    }
    $('field-date').value = isoToDateInput(reminder.next_time, state.timezone);
  } else {
    text.value = '';
    repeat.value = 'none';
    time.value = '09:00';
    $('field-monthday').value = '';
    $('field-every').value = '';
    $('field-date').value = isoToDateInput(new Date().toISOString(), state.timezone);
  }

  updateTextCounter();
  syncWeekdayButtons();
  syncFormFields();
  showView('form');
}

/** Переводит момент времени в значение для <input type="date"> (ГГГГ-ММ-ДД). */
function isoToDateInput(iso, timezone) {
  const options = { year: 'numeric', month: '2-digit', day: '2-digit' };
  if (timezone) {
    options.timeZone = timezone;
  }
  let parts;
  try {
    parts = new Intl.DateTimeFormat('en-CA', options).formatToParts(new Date(iso));
  } catch {
    delete options.timeZone;
    parts = new Intl.DateTimeFormat('en-CA', options).formatToParts(new Date(iso));
  }

  const get = (type) => (parts.find((p) => p.type === type) || {}).value;

  return `${get('year')}-${get('month')}-${get('day')}`;
}

/** Переводит ГГГГ-ММ-ДД из поля ввода в формат API ДД.ММ.ГГГГ. */
function dateInputToAPI(value) {
  if (!value) {
    return null;
  }
  const [year, month, day] = value.split('-');

  return `${day}.${month}.${year}`;
}

function updateTextCounter() {
  $('text-counter').textContent = String($('field-text').value.length);
}

/** Собирает тело запроса из полей формы, проверяя обязательные значения. */
function collectFormPayload() {
  const repeat = $('field-repeat').value;
  const text = $('field-text').value.trim();
  const time = $('field-time').value;

  if (!text) {
    throw new Error('Введите текст напоминания');
  }
  if (!time) {
    throw new Error('Укажите время');
  }

  const payload = { text, time, repeat };

  if (repeat === 'weekly') {
    if (state.selectedWeekdays.size === 0) {
      throw new Error('Выберите хотя бы один день недели');
    }
    payload.repeat_days = [...state.selectedWeekdays].sort((a, b) => a - b);
  }

  if (repeat === 'monthly') {
    const day = Number($('field-monthday').value);
    if (!Number.isInteger(day) || day < 1 || day > 31) {
      throw new Error('Укажите число месяца от 1 до 31');
    }
    payload.repeat_days = [day];
  }

  if (repeat === 'every_n_days') {
    const every = Number($('field-every').value);
    if (!Number.isInteger(every) || every < 1 || every > 365) {
      throw new Error('Укажите интервал от 1 до 365 дней');
    }
    payload.repeat_every = every;

    const date = dateInputToAPI($('field-date').value);
    if (date) {
      payload.date = date;
    }
  }

  if (repeat === 'none' || repeat === 'yearly') {
    const date = dateInputToAPI($('field-date').value);
    if (!date) {
      throw new Error('Укажите дату');
    }
    payload.date = date;
  }

  return payload;
}

async function submitForm() {
  const errorBox = $('form-error');
  errorBox.hidden = true;

  let payload;
  try {
    payload = collectFormPayload();
  } catch (error) {
    errorBox.textContent = error.message;
    errorBox.hidden = false;
    haptic('error');

    return;
  }

  if (tg) {
    tg.MainButton.showProgress();
  }

  try {
    if (state.editing) {
      await api(`/reminders/${state.editing.id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      });
    } else {
      await api(`/chats/${state.chatId}/reminders`, {
        method: 'POST',
        body: JSON.stringify(payload),
      });
    }
    haptic('success');
    await loadReminders();
    showView('list');
  } catch (error) {
    errorBox.textContent = error.message;
    errorBox.hidden = false;
    haptic('error');
  } finally {
    if (tg) {
      tg.MainButton.hideProgress();
    }
  }
}

// --- Настройки ------------------------------------------------------------

async function openSettings() {
  const select = $('field-timezone');
  $('settings-error').hidden = true;

  if (select.options.length === 0) {
    try {
      const { timezones } = await api('/timezones');
      for (const zone of timezones) {
        const option = document.createElement('option');
        option.value = zone;
        option.textContent = zone;
        select.appendChild(option);
      }
    } catch (error) {
      showAlert(error.message);
      return;
    }
  }

  const detected = detectTimezone();
  select.value = state.timezone || detected || 'UTC';

  const hint = $('tz-detected');
  if (!state.timezone && detected) {
    hint.textContent = `Определён по устройству: ${detected}`;
    hint.hidden = false;
  } else {
    hint.hidden = true;
  }

  showView('settings');
}

/** Определяет часовой пояс устройства — избавляет от ручного ввода. */
function detectTimezone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
  } catch {
    return '';
  }
}

async function saveSettings() {
  const errorBox = $('settings-error');
  errorBox.hidden = true;

  if (tg) {
    tg.MainButton.showProgress();
  }

  try {
    await api(`/chats/${state.chatId}/timezone`, {
      method: 'PUT',
      body: JSON.stringify({ timezone: $('field-timezone').value }),
    });
    haptic('success');
    await loadReminders();
    showView('list');
  } catch (error) {
    errorBox.textContent = error.message;
    errorBox.hidden = false;
    haptic('error');
  } finally {
    if (tg) {
      tg.MainButton.hideProgress();
    }
  }
}

// --- Загрузка данных ------------------------------------------------------

async function loadReminders() {
  const data = await api(`/chats/${state.chatId}/reminders`);
  state.timezone = data.timezone || '';
  state.reminders = data.reminders || [];
  renderList();
}

function renderChatPicker() {
  const picker = $('chat-picker');
  const select = $('chat-select');
  const current = state.chats.find((chat) => chat.id === state.chatId);

  if (current) {
    if (current.is_group) {
      const name = current.title || (current.username ? `@${current.username}` : 'Без названия');
      $('chat-context-title').textContent = `Напоминания группы «${name}»`;
    } else {
      $('chat-context-title').textContent = 'Мои напоминания';
    }
  }

  // Переключатель нужен, только если чатов больше одного.
  if (state.chats.length < 2) {
    picker.hidden = true;
    return;
  }

  select.textContent = '';
  for (const chat of state.chats) {
    const option = document.createElement('option');
    option.value = String(chat.id);
    option.textContent = chat.title || (chat.is_group ? 'Группа' : 'Личные напоминания');
    select.appendChild(option);
  }
  select.value = String(state.chatId);
  picker.hidden = false;
}

/**
 * Определяет чат, который нужно открыть.
 *
 * Бот кладёт идентификатор чата в start_param при открытии из группы. Значение
 * не даёт прав: сервер всё равно проверяет членство через Bot API.
 */
function launchChatId() {
  // initData подписан Telegram и является основным источником. Остальные варианты
  // нужны для клиентов разных версий; подмена безопасна, потому что API отдельно
  // проверяет членство пользователя в запрошенном чате.
  const signedParam = tg && tg.initData
    ? new URLSearchParams(tg.initData).get('start_param')
    : '';
  const unsafeParam = tg && tg.initDataUnsafe ? tg.initDataUnsafe.start_param : '';
  const queryParam = new URLSearchParams(window.location.search).get('tgWebAppStartParam');
  const hashParam = new URLSearchParams(window.location.hash.slice(1)).get('tgWebAppStartParam');
  const param = signedParam || unsafeParam || queryParam || hashParam || '';

  if (param && param.startsWith('chat_')) {
    const requested = Number(param.slice('chat_'.length));
    if (Number.isSafeInteger(requested) && requested !== 0) {
      return requested;
    }
  }

  return null;
}

function initialChatId(chats, requested) {
  if (requested !== null) {
    return requested;
  }

  const chatType = tg && tg.initDataUnsafe ? tg.initDataUnsafe.chat_type : '';
  if (chatType === 'group' || chatType === 'supergroup') {
    const groups = chats.filter((chat) => chat.is_group);
    if (groups.length === 1) {
      return groups[0].id;
    }
  }

  return chats.length ? chats[0].id : null;
}

async function bootstrap() {
  const splash = $('splash');
  const splashText = $('splash-text');

  if (!tg || !tg.initData) {
    splashText.textContent = 'Откройте это приложение через Telegram.';
    return;
  }

  tg.ready();
  tg.expand();

  try {
    const me = await api('/me');
    state.chats = me.chats || [];
    const requested = launchChatId();

    // Обычно /me уже содержит группу: /app записывает её до отправки ссылки.
    // Если Telegram открыл старую ссылку или запись ещё не появилась, догружаем
    // чат через защищённый endpoint вместо отката к личным напоминаниям.
    if (requested !== null && !state.chats.some((chat) => chat.id === requested)) {
      state.chats.push(await api(`/chats/${requested}`));
    }

    state.chatId = initialChatId(state.chats, requested);

    if (state.chatId === null) {
      splashText.textContent = 'Не удалось определить чат.';
      return;
    }

    await loadReminders();
  } catch (error) {
    splashText.textContent = error.message;
    return;
  }

  renderChatPicker();

  splash.hidden = true;
  $('app').hidden = false;
  showView('list');
  if (!state.timezone) {
    await openSettings();
  }
}

// --- Обработчики ----------------------------------------------------------

function wireEvents() {
  $('field-repeat').addEventListener('change', syncFormFields);
  $('field-text').addEventListener('input', updateTextCounter);
  $('settings-button').addEventListener('click', openSettings);

  $('chat-select').addEventListener('change', async (event) => {
    state.chatId = Number(event.target.value);
    renderChatPicker();
    try {
      await loadReminders();
      if (state.timezone) {
        showView('list');
      } else {
        await openSettings();
      }
    } catch (error) {
      showAlert(error.message);
    }
  });

  // Форма не отправляется браузером: сохранение идёт через MainButton Telegram.
  $('reminder-form').addEventListener('submit', (event) => {
    event.preventDefault();
    submitForm();
  });

  if (!tg) {
    return;
  }

  tg.MainButton.onClick(() => {
    if (state.view === 'list') {
      openForm(null);
    } else if (state.view === 'form') {
      submitForm();
    } else {
      saveSettings();
    }
  });

  tg.BackButton.onClick(() => showView('list'));
}

buildWeekdayButtons();
wireEvents();
bootstrap();
