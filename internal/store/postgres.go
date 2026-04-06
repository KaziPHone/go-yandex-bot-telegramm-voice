package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// PostgresStore реализует Store на базе PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore открывает соединение с Postgres по DSN и запускает миграции.
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	// простой ping
	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &PostgresStore{db: db}

	if err := s.migrateDB(); err != nil {
		return nil, err
	}

	return s, nil
}

// migrateDB выполняет миграции базы данных
func (s *PostgresStore) migrateDB() error {

	driver, err := postgres.WithInstance(s.db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("error creating migration driver: %w", err)
	}

	src := "file://" + os.Getenv("MIGRATIONS_PATH")

	migrator, err := migrate.NewWithDatabaseInstance(src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Print("migration success")
	return nil
}

// CreateOrUpdateUser вставляет пользователя, если он не существует.
func (s *PostgresStore) CreateOrUpdateUser(userID int64, username string) {
	_, _ = s.db.Exec(`INSERT INTO users (id, username, created_at) VALUES ($1,$2,$3)
        ON CONFLICT (id) DO UPDATE SET username = EXCLUDED.username`, userID, username, time.Now())
}

func (s *PostgresStore) CreateMeetingForUser(userID int64, title string) string {
	id := time.Now().Format("20060102T150405.000000")
	_, _ = s.db.Exec(`INSERT INTO meetings (id, user_id, title, created_at) VALUES ($1,$2,$3,$4)`, id, userID, title, time.Now())
	return id
}

func (s *PostgresStore) ListMeetingsForUser(userID int64) []Meeting {
	rows, err := s.db.Query(`SELECT id, title, created_at, summary FROM meetings WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var out []Meeting
	for rows.Next() {
		var m Meeting
		var created time.Time
		if err := rows.Scan(&m.ID, &m.Title, &created, &m.Summary); err != nil {
			continue
		}
		m.CreatedAt = created
		out = append(out, m)
	}
	return out
}

func (s *PostgresStore) MarkMeetingFailed(meetingID string, reason string) {
	_, _ = s.db.Exec(`UPDATE meetings SET summary = $1 WHERE id = $2`, "FAILED: "+reason, meetingID)
}

func (s *PostgresStore) SaveTranscript(meetingID string, text string) {
	now := time.Now()
	_, _ = s.db.Exec(`INSERT INTO transcripts (meeting_id, text, created_at) VALUES ($1,$2,$3)
        ON CONFLICT (meeting_id) DO UPDATE SET text = EXCLUDED.text, created_at = EXCLUDED.created_at`, meetingID, text, now)
}

func (s *PostgresStore) SaveSummary(meetingID string, summary string) {
	_, _ = s.db.Exec(`UPDATE meetings SET summary = $1 WHERE id = $2`, summary, meetingID)
}

func (s *PostgresStore) GetTranscript(meetingID string) *Meeting {
	var m Meeting
	var created sql.NullTime
	var summary sql.NullString
	var text sql.NullString
	err := s.db.QueryRow(`SELECT m.id, m.user_id, m.title, m.created_at, m.summary, t.text FROM meetings m LEFT JOIN transcripts t ON m.id = t.meeting_id WHERE m.id = $1`, meetingID).Scan(&m.ID, &m.UserID, &m.Title, &created, &summary, &text)
	if err != nil {
		return nil
	}
	if created.Valid {
		m.CreatedAt = created.Time
	}
	if summary.Valid {
		m.Summary = summary.String
	}
	if text.Valid {
		m.Transcript = text.String
	}
	return &m
}

func (s *PostgresStore) SearchTranscripts(query string) []SearchResult {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	// Простая ILIKE-поиск
	q := "%" + query + "%"
	rows, err := s.db.Query(`SELECT m.id, COALESCE(t.text,''), COALESCE(m.created_at, now()) FROM meetings m LEFT JOIN transcripts t ON m.id = t.meeting_id WHERE t.text ILIKE $1 OR m.title ILIKE $1 ORDER BY m.created_at DESC LIMIT 20`, q)
	if err != nil {
		return nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()
	var out []SearchResult
	for rows.Next() {
		var id string
		var text string
		var created time.Time
		if err := rows.Scan(&id, &text, &created); err != nil {
			continue
		}
		snippet := text
		idx := strings.Index(strings.ToLower(text), strings.ToLower(query))
		if idx >= 0 {
			start := idx - 40
			if start < 0 {
				start = 0
			}
			end := idx + 80
			if end > len(text) {
				end = len(text)
			}
			snippet = text[start:end]
		} else if len(snippet) > 120 {
			snippet = snippet[:120] + "..."
		}
		out = append(out, SearchResult{ID: id, CreatedAt: created, Snippet: snippet})
	}
	return out
}
