package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	au "github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/audio"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/gigachat"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/queue"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/salutesspeech"
	"github.com/KaziPHone/go-yandex-bot-telegramm-voice/internal/store"
)

type Handler struct {
	bot      *tele.Bot
	queue    *queue.Queue
	store    store.Store
	speech   salutesspeech.Client
	gigachat gigachat.Client
}

func NewHandler(b *tele.Bot, q *queue.Queue, s store.Store, sc salutesspeech.Client, gc gigachat.Client) *Handler {
	return &Handler{bot: b, queue: q, store: s, speech: sc, gigachat: gc}
}

func (h *Handler) Register() {
	h.bot.Handle(tele.OnText, h.onText)
	h.bot.Handle(tele.OnVoice, h.onVoice)
	h.bot.Handle(tele.OnAudio, h.onAudio)
}

func (h *Handler) onText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	user := c.Sender()
	log.Printf("OnText from %s: %s", user.Username, text)

	if text == "/start" {
		h.store.CreateOrUpdateUser(user.ID, user.Username)
		return c.Send("Привет! Я бот для конспектирования встреч. Отправь голосовое сообщение или аудиофайл.")
	}

	if strings.HasPrefix(text, "/list") {
		meetings := h.store.ListMeetingsForUser(user.ID)
		if len(meetings) == 0 {
			return c.Send("У вас ещё нет сохранённых встреч.")
		}
		var b strings.Builder
		for _, m := range meetings {
			fmt.Fprintf(&b, "%s — %s — id:%s\n", m.CreatedAt.Format(time.RFC3339), m.Title, m.ID)
		}
		return c.Send(b.String())
	}

	if strings.HasPrefix(text, "/get") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return c.Send("Использование: /get <id>")
		}
		id := parts[1]
		tr := h.store.GetTranscript(id)
		if tr == nil {
			return c.Send("Транскрипт не найден или ещё не готов.")
		}
		return c.Send(fmt.Sprintf("Summary:\n%s\n\nTranscript:\n%s", tr.Summary, tr.Transcript))
	}

	if strings.HasPrefix(text, "/find") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return c.Send("Использование: /find <ключевое слово>")
		}
		query := strings.Join(parts[1:], " ")
		res := h.store.SearchTranscripts(query)
		if len(res) == 0 {
			return c.Send("Ничего не найдено.")
		}
		var b strings.Builder
		for _, r := range res {
			fmt.Fprintf(&b, "id:%s date:%s snippet:%s\n", r.ID, r.CreatedAt.Format(time.RFC3339), r.Snippet)
		}
		return c.Send(b.String())
	}

	if strings.HasPrefix(text, "/chat") {
		parts := strings.Fields(text)
		if len(parts) < 2 {
			return c.Send("Использование: /chat <вопрос>")
		}
		q := strings.Join(parts[1:], " ")
		// Для MVP вызываем GigaChat напрямую (мок)
		answer, err := h.gigachat.Chat(context.Background(), q)
		if err != nil {
			return c.Send("Ошибка при обращении к GigaChat")
		}
		return c.Send(answer)
	}

	return c.Send("Неизвестная команда. Доступные: /start, /list, /get, /find, /chat")
}

func (h *Handler) onVoice(c tele.Context) error {
	user := c.Sender()
	log.Printf("OnVoice from %d", user.ID)
	voice := c.Message().Voice
	if voice == nil {
		return c.Send("Не удалось получить голосовое сообщение")
	}

	// Создать запись встречи
	meetingID := h.store.CreateMeetingForUser(user.ID, "Voice message")

	// Попытка скачать файл из Telegram и поставить сырые байты в очередь для SaluteSpeech
	data, _, name, err := downloadTelegramFile(h.bot, os.Getenv("TELEGRAM_TOKEN"), voice.FileID)
	if err != nil {
		log.Printf("failed to download voice file: %v", err)
		// фолбэк: поставить в очередь по FileID
		h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: voice.FileID})
	} else {
		// попытка конвертировать в Opus
		ctxConv, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		convData, convMime, convErr := au.ToOpus(ctxConv, data, name)
		if convErr != nil {
			log.Printf("conversion to opus failed: %v, enqueueing original", convErr)
			h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: voice.FileID, FileName: name, MimeType: "audio/ogg;codecs=opus", Data: data})
		} else {
			h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: voice.FileID, FileName: name, MimeType: convMime, Data: convData})
		}
	}

	return c.Send(fmt.Sprintf("Файл принят, ид встречи: %s. Начинаю распознавание.", meetingID))
}

func (h *Handler) onAudio(c tele.Context) error {
	user := c.Sender()
	audio := c.Message().Audio
	if audio == nil {
		return c.Send("Не удалось получить файл аудио")
	}
	log.Printf("OnAudio from %d file:%s", user.ID, audio.FileID)
	meetingID := h.store.CreateMeetingForUser(user.ID, audio.FileID)
	data, _, name, err := downloadTelegramFile(h.bot, os.Getenv("TELEGRAM_TOKEN"), audio.FileID)
	if err != nil {
		log.Printf("failed to download audio file: %v", err)
		h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: audio.FileID})
	} else {
		ctxConv, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		convData, convMime, convErr := au.ToOpus(ctxConv, data, name)
		if convErr != nil {
			log.Printf("conversion to opus failed: %v, enqueueing original", convErr)
			h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: audio.FileID, FileName: name, MimeType: "audio/ogg;codecs=opus", Data: data})
		} else {
			h.queue.Enqueue(&queue.Task{UserID: user.ID, MeetingID: meetingID, FileID: audio.FileID, FileName: name, MimeType: convMime, Data: convData})
		}
	}
	return c.Send(fmt.Sprintf("Файл принят, ид встречи: %s. Начинаю распознавание.", meetingID))
}

// downloadTelegramFile скачивает байты файла из Telegram, используя API getFile для получения file_path
func downloadTelegramFile(b *tele.Bot, token, fileID string) ([]byte, string, string, error) {
	// Вызов Telegram API getFile для получения file_path
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", token, fileID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, "", "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close http response body: %v", err)
		}
	}()
	var getFileResp struct {
		Ok     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}
	if err := json.Unmarshal(body, &getFileResp); err != nil {
		return nil, "", "", err
	}
	if !getFileResp.Ok || getFileResp.Result.FilePath == "" {
		return nil, "", "", fmt.Errorf("failed to get file path from telegram")
	}
	filePath := getFileResp.Result.FilePath
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, filePath)
	dl, err := http.Get(url)
	if err != nil {
		return nil, "", "", err
	}
	defer func() {
		if err := dl.Body.Close(); err != nil {
			log.Printf("failed to close http download body: %v", err)
		}
	}()
	data, err := io.ReadAll(dl.Body)
	if err != nil {
		return nil, "", "", err
	}
	mime := dl.Header.Get("Content-Type")
	if mime == "" {
		mime = "audio/ogg"
	}
	return data, mime, filePath, nil
}
