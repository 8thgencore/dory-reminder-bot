# 🤖 Dory Reminder Bot

Telegram-бот для создания и управления напоминаниями с поддержкой различных типов повторов и часовых поясов.

## ✨ Возможности

- **Создание напоминаний** с различными типами повторов:
  - Сегодня/завтра
  - Ежедневно
  - Раз в неделю (с выбором дня)
  - Раз в месяц (с выбором числа)
  - Раз в несколько дней
  - Раз в год
  - Разовое напоминание в конкретную дату

- **Управление напоминаниями**:
  - Просмотр списка активных напоминаний
  - Редактирование существующих напоминаний
  - Удаление напоминаний
  - Постановка на паузу/возобновление

- **Поддержка часовых поясов**:
  - Установка персонального часового пояса
  - Автоматический расчет времени с учетом ТЗ

- **Удобный интерфейс**:
  - Интерактивные кнопки
  - Пошаговые мастера создания
  - Поддержка групповых чатов

## 🏗️ Архитектура

Проект построен по принципам **Clean Architecture**:

```
├── cmd/bot/           # Точка входа приложения
├── internal/          # Внутренняя логика
│   ├── config/        # Конфигурация
│   ├── domain/        # Бизнес-модели
│   ├── repository/    # Слой данных
│   ├── usecase/       # Бизнес-логика
│   └── delivery/      # Слой доставки (Telegram)
├── pkg/               # Переиспользуемые пакеты
└── data/              # База данных SQLite
```

## 🚀 Быстрый старт

### Предварительные требования

- Go 1.24.5+
- Task (для управления командами)
- Docker (опционально)

### Установка зависимостей

```bash
task deps:install
```

### Настройка

1. Создайте файл `.env` в корне проекта:

```env
TELEGRAM_TOKEN=your_bot_token_here
BOT_NAME=your_bot_name
ENV=dev
```

2. Получите токен бота у [@BotFather](https://t.me/BotFather) в Telegram

### Запуск в режиме разработки

```bash
task dev:dev
```

Приложение запустится с hot-reload через Air.

### Сборка и запуск

```bash
# Сборка
task build:bot

# Запуск
./bin/bot
```

## 🛠️ Доступные команды

### Разработка

```bash
task dev:dev      # Запуск с hot-reload
task dev:lint     # Проверка кода линтером
task dev:format   # Форматирование кода
task dev:gosec    # Проверка безопасности
```

### Тестирование

```bash
task test         # Запуск всех тестов
task test:coverage # Тесты с отчетом покрытия
```

### Сборка

```bash
task build:bot    # Сборка бота
```

### Docker

```bash
task docker:build # Сборка Docker образа
```

## 🐳 Docker

### Сборка образа

```bash
task docker:build
```

### Запуск с Docker Compose

```bash
docker-compose up -d
```

## 📊 База данных

Проект использует SQLite для хранения данных. База автоматически создается в папке `data/` при первом запуске.

### Схема базы данных

- **users** - информация о пользователях
- **reminders** - напоминания пользователей

## 🔧 Конфигурация

Основные параметры конфигурации:

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `TELEGRAM_TOKEN` | Токен Telegram бота | - |
| `BOT_NAME` | Имя бота | `reminder_bot` |
| `ENV` | Окружение | `dev` |

## 📝 Команды бота

- `/start` - Запустить бота
- `/help` - Справка по командам
- `/add` - Добавить напоминание
- `/list` - Список напоминаний
- `/edit` - Редактировать напоминание
- `/delete` - Удалить напоминание
- `/pause` - Поставить на паузу
- `/resume` - Возобновить
- `/timezone` - Установить часовой пояс

## 🤝 Вклад в проект

1. Форкните репозиторий
2. Создайте ветку для новой функции
3. Внесите изменения
4. Запустите тесты: `task test`
5. Проверьте код: `task dev:lint`
6. Создайте Pull Request

## 📄 Лицензия

MIT License
