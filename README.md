# go-yandex-bot-telegramm-voice

Проект — скелет Telegram-бота для конспектирования встреч: бот принимает голосовые сообщения или аудиофайлы, отправляет аудио на сервис распознавания речи, сохраняет транскрипт и получает краткую сводку (summary) через клиент LLM (GigaChat). Реализована очередь задач и простое in-memory хранилище; поддержка Postgres предусмотрена через интерфейс `store`.

## Краткий обзор

- Язык: Go (модульный проект, Go 1.24)
- Входной файл: `cmd/bot/main.go`
- Telegram API: библиотека `gopkg.in/telebot.v3`
- Компоненты:
	- `internal/bot` — обёртка Telegram-бота и регистрация обработчиков сообщений
	- `internal/audio` — конвертация аудиоданных в OGG/Opus через `ffmpeg`
	- `internal/salutesspeech` — клиент для SaluteSpeech (распознавание речи)
	- `internal/gigachat` — минимальный клиент для GigaChat (получение summary / chat)
	- `internal/queue` — очередь задач и воркеры для обработки аудио
	- `internal/store` — интерфейс хранилища + in-memory реализация; также предусмотрена Postgres-реализация
	- `migrations/` — SQL-миграции для Postgres

Проект сделан как MVP-scaffold: многие места (обработка ошибок, надежность, безопасность) оставлены минимальными и рассчитаны на расширение.

## Быстрый старт (локальная разработка)

1. Установите Go 1.24.x (или совместимую).
2. Установите `ffmpeg` (требуется для конвертации аудио):

```bash
# macOS (Homebrew)
brew install ffmpeg
```

3. Подготовьте переменные окружения (минимум требуется `TELEGRAM_TOKEN`):

```bash
export TELEGRAM_TOKEN="<ваш-telegram-bot-token>"
# опционально (для распознавания / chat):
export SALUTE_BASE_URL="https://salutespeech.example/api"
export SALUTE_API_KEY="<key>"
export GIGA_BASE_URL="https://giga.example/api"
export GIGA_API_KEY="<key>"
```

4. Запуск с in-memory хранилищем (по умолчанию):

```bash
go run ./cmd/bot
```

5. Запуск с Postgres:

```bash
export STORE_TYPE=postgres
export DATABASE_URL="postgres://user:pass@localhost:5432/dbname?sslmode=disable"

# запустите миграции (установите golang-migrate или используйте предпочитаемый инструмент)
brew install golang-migrate
migrate -path ./migrations -database "$DATABASE_URL" up

go run ./cmd/bot
```

## Переменные окружения

- `TELEGRAM_TOKEN` (required) — токен Telegram-бота.
- `STORE_TYPE` — `postgres` или не задан (по умолчанию используется in-memory).
- `DATABASE_URL` — DSN для Postgres (требуется при `STORE_TYPE=postgres`).
- `SALUTE_BASE_URL` — базовый URL API SaluteSpeech.
- `SALUTE_API_KEY` — API ключ для SaluteSpeech.
- `GIGA_BASE_URL` — базовый URL API GigaChat.
- `GIGA_API_KEY` — API ключ для GigaChat.
- `INMEMORY_STORE_FILE` — путь к файлу для сохранения in-memory кэша (по умолчанию `./inmemory_store.json`).
- `INMEMORY_FLUSH_INTERVAL_SECONDS` — интервал автосохранения in-memory кэша (по умолчанию 5s).

## Запуск тестов

Запустить все тесты:

```bash
go test ./...
```

В проекте есть простые unit-тесты для некоторых пакетов (`internal/audio`, `internal/gigachat`, `internal/salutesspeech`, `internal/queue`, `internal/store`).

## Требования и примечания

- `ffmpeg` должен быть доступен в PATH — используется для конвертации входных аудиофайлов в OGG/Opus (`internal/audio.ToOpus`).
- Клиент `salutesspeech` в текущем виде ожидает синхронный endpoint `/speech:recognize` и возвращает JSON `{ "result": ["line1", "line2"] }`.
- Асинхронный режим распознавания (`RecognizeAsync`) в `salutesspeech` помечен как не реализованный (заглушка).
- `internal/gigachat` — минимальный клиент, отправляет body с `model: "GigaChat"` и извлекает `choices[0].message.content` в ответе.
- По умолчанию используется in-memory хранилище (`internal/store.NewInMemoryStore`) — удобно для разработки. Для продакшна рекомендована Postgres-реализация (см. `migrations/`).

## Архитектура обработки

1. Телеграм-обработчик (`internal/bot/handler.go`) принимает голос/аудиофайл, создаёт запись встречи в хранилище и помещает задачу в очередь.
2. Воркеры очереди (`internal/queue`) берут задачу, отправляют аудио в SaluteSpeech (или вызывают async endpoint) и сохраняют результат как транскрипт.
3. Полученный транскрипт отправляется в GigaChat для получения краткой сводки (summary).
4. Результаты сохраняются в хранилище; при наличии `notifier` пользователь уведомляется в Telegram.

## Отладка и полезные советы

- Для локальной проверки можно использовать только `TELEGRAM_TOKEN` и in-memory store — тогда бот будет принимать сообщения и хранить их в `inmemory_store.json`.
- Если загрузка файлов из Telegram не проходит, проверьте корректность `TELEGRAM_TOKEN` и сетевые настройки.
- Логи помогают отслеживать работу воркеров и возможные ошибки внешних сервисов.


