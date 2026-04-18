-- migrations/000001_init.down.sql
DROP INDEX IF EXISTS idx_transcripts_text;
DROP TABLE IF EXISTS transcripts;
DROP TABLE IF EXISTS meetings;
DROP TABLE IF EXISTS users;