package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// NewInMemoryStore возвращает потокобезопасное in-memory хранилище.
func NewInMemoryStore() Store {
	s := &inMemoryStore{meetings: map[string]Meeting{}, mu: sync.RWMutex{}}
	// Попытаться загрузить ранее сохранённый кэш
	if err := s.loadFromFile(); err != nil {
		// не фатальная ошибка — просто логируем, продолжаем с пустым кэшем
		log.Printf("in-memory store: failed to load cache: %v", err)
	}

	// Интервал флашера в секундах (по умолчанию 5s). Можно задать через
	// INMEMORY_FLUSH_INTERVAL_SECONDS.
	iv := 5
	if v := os.Getenv("INMEMORY_FLUSH_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			iv = n
		}
	}
	s.flushInterval = time.Duration(iv) * time.Second
	s.flusherQuit = make(chan struct{})
	s.flusherDone = make(chan struct{})
	s.startFlusher()
	return s
}

type inMemoryStore struct {
	mu            sync.RWMutex
	meetings      map[string]Meeting
	flushInterval time.Duration
	flusherQuit   chan struct{}
	flusherDone   chan struct{}
}

// filePath возвращает путь к файлу, используемому для сохранения кэша in-memory.
// Можно переопределить через переменную окружения INMEMORY_STORE_FILE.
func filePath() string {
	if p := os.Getenv("INMEMORY_STORE_FILE"); p != "" {
		return p
	}
	return filepath.Join(".", "inmemory_store.json")
}

// loadFromFile загружает сохранённый кэш из JSON-файла, если он существует.
func (s *inMemoryStore) loadFromFile() error {
	p := filePath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read file %s: %w", p, err)
	}
	var m map[string]Meeting
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	s.mu.Lock()
	s.meetings = m
	s.mu.Unlock()
	return nil
}

// saveToFile сохраняет текущий кэш в JSON-файл (атомично).
func (s *inMemoryStore) saveToFile() error {
	p := filePath()
	s.mu.RLock()
	data, err := json.MarshalIndent(s.meetings, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	// Записать во временный файл, затем переименовать для атомарности
	tmp, err := os.CreateTemp(filepath.Dir(p), "inmemory-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		if cerr := tmp.Close(); cerr != nil {
			log.Printf("failed to close temp file after write error: %v", cerr)
		}
		if rerr := os.Remove(tmpName); rerr != nil {
			log.Printf("failed to remove temp file after write error: %v", rerr)
		}
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		if rerr := os.Remove(tmpName); rerr != nil {
			log.Printf("failed to remove temp file after close error: %v", rerr)
		}
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		if rerr := os.Remove(tmpName); rerr != nil {
			log.Printf("failed to remove temp file after rename error: %v", rerr)
		}
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// startFlusher запускает фоновую горутину, которая сохраняет кэш каждые s.flushInterval.
func (s *inMemoryStore) startFlusher() {
	go func() {
		ticker := time.NewTicker(s.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.saveToFile(); err != nil {
					log.Printf("in-memory store: flusher save error: %v", err)
				}
			case <-s.flusherQuit:
				// финальная запись перед выходом
				if err := s.saveToFile(); err != nil {
					log.Printf("in-memory store: flusher final save error: %v", err)
				}
				close(s.flusherDone)
				return
			}
		}
	}()
}

// StopFlusher останавливает фоновый flusher и дожидается его завершения.
// Этот метод предназначен для тестов и аккуратного завершения процесса.
func (s *inMemoryStore) StopFlusher() {
	// безопасно закрываем канал (если уже закрыт, это паникнет) — поэтому проверяем
	select {
	case <-s.flusherDone:
		// уже завершён
		return
	default:
	}
	close(s.flusherQuit)
	<-s.flusherDone
}

func (s *inMemoryStore) CreateOrUpdateUser(userID int64, username string) {
	// пустая реализация для скелета
}

func (s *inMemoryStore) CreateMeetingForUser(userID int64, title string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := time.Now().Format("20060102T150405.000000")
	s.meetings[id] = Meeting{ID: id, UserID: userID, Title: title, CreatedAt: time.Now()}
	return id
}

func (s *inMemoryStore) ListMeetingsForUser(userID int64) []Meeting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Meeting
	for _, m := range s.meetings {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out
}

func (s *inMemoryStore) MarkMeetingFailed(meetingID string, reason string) {
	s.mu.Lock()
	m := s.meetings[meetingID]
	m.Summary = "FAILED: " + reason
	s.meetings[meetingID] = m
	s.mu.Unlock()
}

func (s *inMemoryStore) SaveTranscript(meetingID string, text string) {
	s.mu.Lock()
	m := s.meetings[meetingID]
	m.Transcript = text
	s.meetings[meetingID] = m
	s.mu.Unlock()
}

func (s *inMemoryStore) SaveSummary(meetingID string, summary string) {
	s.mu.Lock()
	m := s.meetings[meetingID]
	m.Summary = summary
	s.meetings[meetingID] = m
	s.mu.Unlock()
}

func (s *inMemoryStore) GetTranscript(meetingID string) *Meeting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.meetings[meetingID]
	if !ok {
		return nil
	}
	return &m
}

func (s *inMemoryStore) SearchTranscripts(query string) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []SearchResult
	for _, m := range s.meetings {
		if query == "" {
			continue
		}
		if len(m.Transcript) > 0 && (contains(m.Transcript, query) || contains(m.Title, query)) {
			snippet := m.Transcript
			if len(snippet) > 120 {
				snippet = snippet[:120] + "..."
			}
			out = append(out, SearchResult{ID: m.ID, CreatedAt: m.CreatedAt, Snippet: snippet})
		}
	}
	return out
}
