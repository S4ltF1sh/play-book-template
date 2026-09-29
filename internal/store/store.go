// Package store persists user data (progress, quiz attempts, settings) in SQLite.
package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// modernc/sqlite is not safe for concurrent writes on one connection pool
	// beyond SQLite's own locking; a single connection keeps things simple.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS progress (
  section_id TEXT PRIMARY KEY,
  status     TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS quiz_attempts (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  quiz_id    TEXT NOT NULL,
  score      INTEGER NOT NULL,
  total      INTEGER NOT NULL,
  answers    TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Progress() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT section_id, status FROM progress`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, st string
		if err := rows.Scan(&id, &st); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

func (s *Store) SetProgress(sectionID, status string) error {
	if status == "" { // clearing
		_, err := s.db.Exec(`DELETE FROM progress WHERE section_id = ?`, sectionID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO progress (section_id, status, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(section_id) DO UPDATE SET status=excluded.status, updated_at=excluded.updated_at`,
		sectionID, status, time.Now().Unix())
	return err
}

type QuizAttempt struct {
	ID        int64  `json:"id"`
	QuizID    string `json:"quiz_id"`
	Score     int    `json:"score"`
	Total     int    `json:"total"`
	Answers   string `json:"answers"`
	CreatedAt int64  `json:"created_at"`
}

func (s *Store) AddQuizAttempt(quizID string, score, total int, answersJSON string) error {
	_, err := s.db.Exec(`
		INSERT INTO quiz_attempts (quiz_id, score, total, answers, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		quizID, score, total, answersJSON, time.Now().Unix())
	return err
}

func (s *Store) QuizAttempts(quizID string) ([]QuizAttempt, error) {
	rows, err := s.db.Query(`
		SELECT id, quiz_id, score, total, answers, created_at
		FROM quiz_attempts WHERE quiz_id = ? ORDER BY created_at DESC`, quizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QuizAttempt{}
	for rows.Next() {
		var a QuizAttempt
		if err := rows.Scan(&a.ID, &a.QuizID, &a.Score, &a.Total, &a.Answers, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) Setting(key, fallback string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return fallback
	}
	return v
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
