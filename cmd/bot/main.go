package main

import (
	"context"
	"log"
	"os"

	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/bot"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/gigachat"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/queue"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/salutesspeech"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/store"
)

// botNotifier определён в telegramm.go

func main() {
	log.Println("Starting meeting-notes bot (MVP scaffold)")

	b, err := bot.NewBot()
	if err != nil {
		log.Fatalf("failed to create telegram bot: %v", err)
	}

	// Выбор хранилища: memory или postgres (по переменной STORE_TYPE)
	var st store.Store
	storeType := os.Getenv("STORE_TYPE")
	if storeType == "postgres" {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL is required when STORE_TYPE=postgres")
		}
		pg, err := store.NewPostgresStore(dsn)
		if err != nil {
			log.Fatalf("failed to connect to postgres: %v", err)
		}
		st = pg
	} else {
		st = store.NewInMemoryStore()
	}

	// Инициализация мок-клиентов (заменить реальными позже)
	speechClient := salutesspeech.NewRealClientFromEnv()
	chatClient := gigachat.NewClient()

	// Очередь задач и воркеры
	q := queue.NewQueue(4)

	// Нотификатор, отправляющий сообщения через Telegram-бота
	notifier := bot.NewNotifier(b)
	q.StartWorkers(context.Background(), 2, speechClient, chatClient, st, notifier)

	// Регистрация обработчиков бота
	h := bot.NewHandler(b, q, st, speechClient, chatClient)
	h.Register()

	log.Println("Bot registered handlers, starting...")
	b.Start()
}
