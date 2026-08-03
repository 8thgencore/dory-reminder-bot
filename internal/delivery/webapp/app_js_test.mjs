import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

const appSource = await readFile(
  new URL('./web/app.js', import.meta.url),
  'utf8',
);
const indexSource = await readFile(
  new URL('./web/index.html', import.meta.url),
  'utf8',
);

class FakeElement {
  constructor(tagName = 'div') {
    this.tagName = tagName.toUpperCase();
    this.hidden = false;
    this.value = '';
    this.children = [];
    this.listeners = new Map();
    this.dataset = {};
    this.className = '';
    this.style = {};
    this.attributes = new Map();
    this._textContent = '';
    this.classList = {
      toggle: () => {},
      add: () => {},
      remove: () => {},
    };
  }

  get options() {
    return this.children;
  }

  get textContent() {
    return this._textContent;
  }

  set textContent(value) {
    this._textContent = String(value);
    if (value === '') {
      this.children = [];
    }
  }

  appendChild(child) {
    this.children.push(child);
    return child;
  }

  addEventListener(name, handler) {
    this.listeners.set(name, handler);
  }

  setAttribute(name, value) {
    this.attributes.set(name, String(value));
  }

  querySelectorAll() {
    return [];
  }

  focus() {}

  remove() {}
}

function makeHarness(reminderResponses) {
  const elements = new Map();
  for (const match of indexSource.matchAll(/\bid="([^"]+)"/g)) {
    elements.set(match[1], new FakeElement());
  }
  elements.get('app').hidden = true;
  elements.get('splash').hidden = false;

  const frames = [];
  const buttonCalls = [];
  const fetchCalls = [];
  const telegramEvents = new Map();
  let readyCount = 0;

  const tg = {
    initData: 'signed-init-data',
    initDataUnsafe: {},
    expand() {},
    ready() {
      readyCount += 1;
    },
    onEvent(name, handler) {
      telegramEvents.set(name, handler);
    },
    BackButton: {
      show() {},
      hide() {},
      onClick() {},
    },
    MainButton: {
      hide() {
        buttonCalls.push({ operation: 'hide' });
      },
      setParams(params) {
        buttonCalls.push({ operation: 'setParams', params });
      },
      onClick() {},
      showProgress() {},
      hideProgress() {},
    },
    HapticFeedback: {
      notificationOccurred() {},
    },
    showAlert() {},
    showConfirm(_message, callback) {
      callback(false);
    },
  };

  const responses = {
    '/api/v1/me': {
      user: { id: 42 },
      chats: [
        { id: -1002, title: 'Команда', is_group: true },
        { id: 42, title: 'Дарья', is_group: false },
      ],
      launch_chat_id: -1002,
    },
    '/api/v1/timezones': {
      timezones: ['Europe/Moscow', 'UTC'],
    },
    ...reminderResponses,
  };

  const document = {
    documentElement: new FakeElement('html'),
    getElementById(id) {
      return elements.get(id) || null;
    },
    createElement(tagName) {
      return new FakeElement(tagName);
    },
    createTextNode(text) {
      const node = new FakeElement('#text');
      node.textContent = text;
      return node;
    },
  };

  const window = {
    Telegram: { WebApp: tg },
    document,
    Intl,
    location: { search: '', hash: '' },
    requestAnimationFrame(callback) {
      frames.push(callback);
    },
    setTimeout,
    confirm: () => false,
  };
  window.window = window;

  const context = vm.createContext({
    console,
    document,
    fetch: async (path) => {
      fetchCalls.push(path);
      const body = responses[path];
      if (!body) {
        throw new Error(`Unexpected fetch: ${path}`);
      }
      return {
        ok: true,
        status: 200,
        async json() {
          return body;
        },
      };
    },
    Intl,
    Promise,
    Set,
    URLSearchParams,
    clearTimeout,
    confirm: window.confirm,
    setTimeout,
    window,
  });

  vm.runInContext(appSource, context, { filename: 'app.js' });

  return {
    buttonCalls,
    context,
    elements,
    fetchCalls,
    flushFrame() {
      const callback = frames.shift();
      assert.ok(callback, 'expected an animation frame callback');
      callback();
    },
    get readyCount() {
      return readyCount;
    },
    telegramEvents,
  };
}

async function eventually(predicate, message) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setImmediate(resolve));
  }
  assert.fail(message);
}

test('cold group launch opens timezone settings and forces Save on Android', async () => {
  const harness = makeHarness({
    '/api/v1/chats/-1002/reminders': { timezone: '', reminders: [] },
    '/api/v1/chats/42/reminders': { timezone: 'Europe/Moscow', reminders: [] },
  });

  await eventually(
    () => harness.buttonCalls.some((call) => call.operation === 'hide'),
    'bootstrap did not start forced button synchronization',
  );

  assert.equal(vm.runInContext('state.chatId', harness.context), -1002);
  assert.equal(vm.runInContext('state.view', harness.context), 'settings');
  assert.equal(harness.elements.get('chat-select').value, '-1002');
  assert.ok(harness.fetchCalls.includes('/api/v1/chats/-1002/reminders'));
  assert.equal(harness.readyCount, 1);

  harness.flushFrame();
  await eventually(
    () => harness.buttonCalls.some(
      (call) => call.operation === 'setParams' && call.params.text === 'Сохранить',
    ),
    'Save button was not sent after the forced frame',
  );

  const saveCall = harness.buttonCalls.find(
    (call) => call.operation === 'setParams' && call.params.text === 'Сохранить',
  );
  assert.deepEqual({ ...saveCall.params }, {
    text: 'Сохранить',
    is_visible: true,
    is_active: true,
  });
  assert.equal(harness.elements.get('view-settings').hidden, false);
});

test('chat changes and activation use the same current view state', async () => {
  const harness = makeHarness({
    '/api/v1/chats/-1002/reminders': { timezone: '', reminders: [] },
    '/api/v1/chats/42/reminders': { timezone: 'Europe/Moscow', reminders: [] },
  });

  await eventually(
    () => harness.buttonCalls.some((call) => call.operation === 'hide'),
    'bootstrap did not start forced synchronization',
  );
  harness.flushFrame();
  await eventually(
    () => harness.elements.get('field-timezone').options.length > 0,
    'timezone options did not load',
  );

  const change = harness.elements.get('chat-select').listeners.get('change');
  assert.ok(change);
  await change({ target: { value: '42' } });

  assert.equal(vm.runInContext('state.chatId', harness.context), 42);
  assert.equal(vm.runInContext('state.view', harness.context), 'list');
  assert.equal(harness.buttonCalls.at(-1).params.text, 'Добавить напоминание');

  await change({ target: { value: '-1002' } });
  assert.equal(vm.runInContext('state.view', harness.context), 'settings');
  assert.equal(harness.buttonCalls.at(-1).params.text, 'Сохранить');

  const activated = harness.telegramEvents.get('activated');
  assert.ok(activated);
  activated();
  assert.equal(harness.buttonCalls.at(-1).operation, 'hide');
  harness.flushFrame();
  assert.equal(harness.buttonCalls.at(-1).params.text, 'Сохранить');
});

test('date formatting falls back when the chat timezone is unknown', () => {
  const harness = makeHarness({
    '/api/v1/chats/-1002/reminders': { timezone: '', reminders: [] },
  });
  const iso = '2025-06-13T09:30:00Z';

  for (const expression of [
    `formatDateTime('${iso}', 'Invalid/Timezone') === formatDateTime('${iso}', '')`,
    `timeInZone('${iso}', 'Invalid/Timezone') === timeInZone('${iso}', '')`,
    `isoToDateInput('${iso}', 'Invalid/Timezone') === isoToDateInput('${iso}', '')`,
  ]) {
    assert.equal(vm.runInContext(expression, harness.context), true);
  }
});
