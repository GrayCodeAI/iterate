package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/GrayCodeAI/iterate/internal/agent"
)

// Session records one evolution run.
type Session struct {
	ID         int64
	StartedAt  time.Time
	FinishedAt time.Time
	Status     string // running, committed, reverted, error
	Provider   string
	RawOutput  string
	TestOutput string
	Error      string
	Events     <-chan agent.Event `json:"-"` // live stream, not persisted
}

// Store persists sessions in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database at path.
func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Save inserts or updates a session.
func (s *Store) Save(sess *Session) error {
	meta, _ := json.Marshal(map[string]string{
		"provider":    sess.Provider,
		"test_output": truncate(sess.TestOutput, 2000),
		"error":       sess.Error,
	})

	if sess.ID == 0 {
		result, err := s.db.Exec(`
			INSERT INTO sessions (started_at, finished_at, status, provider, raw_output, meta)
			VALUES (?, ?, ?, ?, ?, ?)`,
			sess.StartedAt, sess.FinishedAt, sess.Status, sess.Provider,
			truncate(sess.RawOutput, 8000), string(meta),
		)
		if err != nil {
			return err
		}
		sess.ID, _ = result.LastInsertId()
	} else {
		_, err := s.db.Exec(`
			UPDATE sessions SET finished_at=?, status=?, raw_output=?, meta=?
			WHERE id=?`,
			sess.FinishedAt, sess.Status,
			truncate(sess.RawOutput, 8000), string(meta),
			sess.ID,
		)
		return err
	}
	return nil
}

// List returns the last n sessions, newest first.
func (s *Store) List(n int) ([]Session, error) {
	rows, err := s.db.Query(`
		SELECT id, started_at, finished_at, status, provider, raw_output
		FROM sessions ORDER BY started_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var finishedAt sql.NullTime
		if err := rows.Scan(&sess.ID, &sess.StartedAt, &finishedAt,
			&sess.Status, &sess.Provider, &sess.RawOutput); err != nil {
			continue
		}
		if finishedAt.Valid {
			sess.FinishedAt = finishedAt.Time
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// Stats returns aggregate counts by status.
func (s *Store) Stats() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM sessions GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats[status] = count
	}
	return stats, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at  DATETIME NOT NULL,
			finished_at DATETIME,
			status      TEXT NOT NULL DEFAULT 'running',
			provider    TEXT NOT NULL,
			raw_output  TEXT,
			meta        TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at DESC);
	`)
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}
