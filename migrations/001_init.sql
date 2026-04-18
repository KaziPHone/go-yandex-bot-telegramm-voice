-- 001_init.sql
-- Initial schema: users, meetings and transcripts tables.

BEGIN;

CREATE TABLE IF NOT EXISTS users (
	id BIGINT PRIMARY KEY,
	username TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS meetings (
	id TEXT PRIMARY KEY,
	user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
	title TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
	summary TEXT
);

CREATE TABLE IF NOT EXISTS transcripts (
	meeting_id TEXT PRIMARY KEY REFERENCES meetings(id) ON DELETE CASCADE,
	text TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Optional index to accelerate transcript search (ILike queries still use sequential scan for text)
CREATE INDEX IF NOT EXISTS idx_transcripts_text ON transcripts USING gin (to_tsvector('russian', COALESCE(text, '')));

COMMIT;
