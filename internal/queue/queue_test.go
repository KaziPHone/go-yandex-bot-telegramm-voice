package queue

import (
	"context"
	"testing"
	"time"

	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/salutesspeech"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/store"
)

// Моки (тестовые реализации)
type mockSpeech struct{}

func (m *mockSpeech) Recognize(ctx context.Context, data []byte, mime string) (*salutesspeech.Result, error) {
	return &salutesspeech.Result{Text: "recognized text"}, nil
}
func (m *mockSpeech) RecognizeAsync(ctx context.Context, ref string) (*salutesspeech.Result, error) {
	return nil, nil
}

type mockChat struct{}

func (m *mockChat) Summarize(ctx context.Context, text string) (string, error) { return "summary", nil }
func (m *mockChat) Chat(ctx context.Context, q string) (string, error)         { return "chat", nil }

type mockStore struct {
	savedTranscript chan string
	savedSummary    chan string
}

func NewMockStore() *mockStore {
	return &mockStore{savedTranscript: make(chan string, 1), savedSummary: make(chan string, 1)}
}
func (m *mockStore) CreateOrUpdateUser(userID int64, username string)       {}
func (m *mockStore) CreateMeetingForUser(userID int64, title string) string { return "mid" }
func (m *mockStore) ListMeetingsForUser(userID int64) []store.Meeting       { return nil }
func (m *mockStore) MarkMeetingFailed(meetingID string, reason string)      {}
func (m *mockStore) SaveTranscript(meetingID string, text string)           { m.savedTranscript <- text }
func (m *mockStore) SaveSummary(meetingID string, summary string)           { m.savedSummary <- summary }
func (m *mockStore) GetTranscript(meetingID string) *store.Meeting          { return nil }
func (m *mockStore) SearchTranscripts(query string) []store.SearchResult    { return nil }

type mockNotifier struct{ ch chan string }

func (n *mockNotifier) Notify(userID int64, text string) error { n.ch <- text; return nil }

func TestWorkerProcessesTask(t *testing.T) {
	q := NewQueue(4)
	ms := NewMockStore()
	notifier := &mockNotifier{ch: make(chan string, 1)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.StartWorkers(ctx, 1, &mockSpeech{}, &mockChat{}, ms, notifier)

	// поставить задачу с Data, чтобы вызвать Recognize
	q.Enqueue(&Task{UserID: 111, MeetingID: "m1", Data: []byte{1, 2, 3}, MimeType: "audio/ogg"})

	select {
	case tr := <-ms.savedTranscript:
		if tr != "recognized text" {
			t.Fatalf("unexpected transcript: %s", tr)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for SaveTranscript")
	}

	select {
	case s := <-ms.savedSummary:
		if s != "summary" {
			t.Fatalf("unexpected summary: %s", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for SaveSummary")
	}

	select {
	case msg := <-notifier.ch:
		if msg == "" {
			t.Fatalf("empty notify message")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for notification")
	}

	cancel()
}
