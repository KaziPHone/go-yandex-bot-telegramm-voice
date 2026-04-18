package bot

import (
	"fmt"
	"os"
	"time"

	tele "gopkg.in/telebot.v3"
)

type Bot struct {
}

func NewBot() (*tele.Bot, error) {
	return createBot()
}

func createBot() (*tele.Bot, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN environment variable is required")
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %v", err)
	}
	return b, nil
}

// notifier отправляет уведомления пользователю через экземпляр tele.Bot.
// Расположен в пакете internal/bot, чтобы код в cmd мог использовать его как
// конкретный тип, реализующий интерфейс queue.Notifier.
type notifier struct{ b *tele.Bot }

// NewNotifier возвращает новый экземпляр notifier.
func NewNotifier(b *tele.Bot) *notifier { return &notifier{b: b} }

// Notify отправляет текстовое сообщение пользователю с указанным userID.
func (n *notifier) Notify(userID int64, text string) error {
	recipient := &tele.User{ID: userID}
	_, err := n.b.Send(recipient, text)
	return err
}
