package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFlusherPersists(t *testing.T) {
	// подготовить временный файл
	d := t.TempDir()
	p := filepath.Join(d, "cache.json")
	old := os.Getenv("INMEMORY_STORE_FILE")
	if err := os.Setenv("INMEMORY_STORE_FILE", p); err != nil {
		t.Fatalf("failed to set INMEMORY_STORE_FILE: %v", err)
	}
	defer func() {
		if err := os.Setenv("INMEMORY_STORE_FILE", old); err != nil {
			t.Fatalf("failed to restore INMEMORY_STORE_FILE: %v", err)
		}
	}()
	oldIv := os.Getenv("INMEMORY_FLUSH_INTERVAL_SECONDS")
	if err := os.Setenv("INMEMORY_FLUSH_INTERVAL_SECONDS", "1"); err != nil {
		t.Fatalf("failed to set INMEMORY_FLUSH_INTERVAL_SECONDS: %v", err)
	}
	defer func() {
		if err := os.Setenv("INMEMORY_FLUSH_INTERVAL_SECONDS", oldIv); err != nil {
			t.Fatalf("failed to restore INMEMORY_FLUSH_INTERVAL_SECONDS: %v", err)
		}
	}()

	s := NewInMemoryStore()
	si, ok := s.(*inMemoryStore)
	if !ok {
		t.Fatalf("expected *inMemoryStore, got %T", s)
	}

	id := si.CreateMeetingForUser(123, "test meeting")
	si.SaveTranscript(id, "hello world")
	si.SaveSummary(id, "short summary")

	// дождаться, пока flusher запишет файл (до ~3s)
	var found bool
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(p); err == nil {
			found = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !found {
		si.StopFlusher()
		t.Fatalf("cache file was not written by flusher: %s", p)
	}

	// прочитать файл и проверить содержимое
	b, err := os.ReadFile(p)
	if err != nil {
		si.StopFlusher()
		t.Fatalf("read cache: %v", err)
	}
	var m map[string]Meeting
	if err := json.Unmarshal(b, &m); err != nil {
		si.StopFlusher()
		t.Fatalf("unmarshal cache: %v", err)
	}
	if _, ok := m[id]; !ok {
		si.StopFlusher()
		t.Fatalf("meeting %s not found in persisted cache", id)
	}

	si.StopFlusher()
}

func TestLoadFromFile(t *testing.T) {
	// Подготовить файл с одной встречей
	d := t.TempDir()
	p := filepath.Join(d, "cache.json")
	m := map[string]Meeting{}
	id := time.Now().Format("20060102T150405.000000")
	m[id] = Meeting{ID: id, UserID: 321, Title: "loaded", CreatedAt: time.Now(), Transcript: "abc", Summary: "sum"}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatalf("write test cache: %v", err)
	}

	old := os.Getenv("INMEMORY_STORE_FILE")
	if err := os.Setenv("INMEMORY_STORE_FILE", p); err != nil {
		t.Fatalf("failed to set INMEMORY_STORE_FILE: %v", err)
	}
	defer func() {
		if err := os.Setenv("INMEMORY_STORE_FILE", old); err != nil {
			t.Fatalf("failed to restore INMEMORY_STORE_FILE: %v", err)
		}
	}()
	oldIv := os.Getenv("INMEMORY_FLUSH_INTERVAL_SECONDS")
	if err := os.Setenv("INMEMORY_FLUSH_INTERVAL_SECONDS", "60"); err != nil {
		t.Fatalf("failed to set INMEMORY_FLUSH_INTERVAL_SECONDS: %v", err)
	}
	defer func() {
		if err := os.Setenv("INMEMORY_FLUSH_INTERVAL_SECONDS", oldIv); err != nil {
			t.Fatalf("failed to restore INMEMORY_FLUSH_INTERVAL_SECONDS: %v", err)
		}
	}()

	s := NewInMemoryStore()
	si, ok := s.(*inMemoryStore)
	if !ok {
		t.Fatalf("expected *inMemoryStore, got %T", s)
	}

	m2 := si.ListMeetingsForUser(321)
	if len(m2) == 0 {
		si.StopFlusher()
		t.Fatalf("expected loaded meeting for user 321")
	}
	if m2[0].Transcript != "abc" {
		si.StopFlusher()
		t.Fatalf("unexpected transcript: %s", m2[0].Transcript)
	}

	si.StopFlusher()
}
