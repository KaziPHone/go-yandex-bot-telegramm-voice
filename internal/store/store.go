package store

import (
	"time"
)

// Минимальное in-memory хранилище для скелета и локальной разработки.

type Meeting struct {
	ID         string
	UserID     int64
	Title      string
	CreatedAt  time.Time
	Summary    string
	Transcript string
}

type SearchResult struct {
	ID        string
	CreatedAt time.Time
	Snippet   string
}

type Store interface {
	CreateOrUpdateUser(userID int64, username string)
	CreateMeetingForUser(userID int64, title string) string
	ListMeetingsForUser(userID int64) []Meeting
	MarkMeetingFailed(meetingID string, reason string)
	SaveTranscript(meetingID string, text string)
	SaveSummary(meetingID string, summary string)
	GetTranscript(meetingID string) *Meeting
	SearchTranscripts(query string) []SearchResult
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (stringIndex(haystack, needle) >= 0)
}

// наивный индекс — избегаем импорта strings для простоты реализации скелета
func stringIndex(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
