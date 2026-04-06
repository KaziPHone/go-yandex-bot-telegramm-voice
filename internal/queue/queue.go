package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/gigachat"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/salutesspeech"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/store"
)

type Queue struct {
	ch chan *Task
}

type Task struct {
	UserID    int64
	MeetingID string
	FileID    string
	FileName  string
	MimeType  string
	Data      []byte
}

type Notifier interface {
	Notify(userID int64, text string) error
}

func NewQueue(buffer int) *Queue {
	return &Queue{ch: make(chan *Task, buffer)}
}

func (q *Queue) Enqueue(t *Task) {
	q.ch <- t
}

// StartWorkers запускает n воркеров для обработки задач из очереди.
// notifier может быть nil (в этом случае уведомления пользователю не отправляются).
func (q *Queue) StartWorkers(ctx context.Context, n int, sc salutesspeech.Client, gc gigachat.Client, st store.Store, notifier Notifier) {
	for i := 0; i < n; i++ {
		go func(id int) {
			log.Printf("worker-%d started", id)
			for {
				select {
				case <-ctx.Done():
					log.Printf("worker-%d stopping", id)
					return
				case t := <-q.ch:
					log.Printf("worker-%d processing meeting=%s file=%s", id, t.MeetingID, t.FileID)
					var res *salutesspeech.Result
					var err error
					// Если присутствуют сырые данные, используем синхронный endpoint Recognize
					if len(t.Data) > 0 {
						res, err = sc.Recognize(ctx, t.Data, t.MimeType)
					} else {
						// иначе — фолбэк на асинхронный поток (по ссылке на файл)
						res, err = sc.RecognizeAsync(ctx, t.FileID)
					}
					if err != nil {
						log.Printf("recognize error: %v", err)
						st.MarkMeetingFailed(t.MeetingID, err.Error())
						continue
					}
					// Сохранить транскрипт
					st.SaveTranscript(t.MeetingID, res.Text)
					// Получить сводку через GigaChat
					summary, _ := gc.Summarize(ctx, res.Text)
					st.SaveSummary(t.MeetingID, summary)

					// Уведомить пользователя о завершении обработки
					if notifier != nil {
						msg := fmt.Sprintf("Готово: встреча %s\n%s", t.MeetingID, summary)
						if err := notifier.Notify(t.UserID, msg); err != nil {
							log.Printf("failed to notify user %d: %v", t.UserID, err)
						}
					}

					log.Printf("worker-%d done meeting=%s", id, t.MeetingID)
					// небольшая пауза для симуляции времени обработки
					time.Sleep(500 * time.Millisecond)
				}
			}
		}(i + 1)
	}
}
