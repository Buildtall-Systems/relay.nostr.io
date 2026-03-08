package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const sessionKey contextKey = "session"

func WithSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

func GetSession(r *http.Request) *Session {
	session, ok := r.Context().Value(sessionKey).(*Session)
	if !ok {
		return nil
	}
	return session
}

type Session struct {
	ID        string
	Npub      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(ctx context.Context, npub string, duration time.Duration) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generating session ID: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(duration)

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, npub, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		id, npub, now, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting session: %w", err)
	}

	return &Session{
		ID:        id,
		Npub:      npub,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (*Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx,
		`SELECT id, npub, created_at, expires_at FROM sessions WHERE id = ?`,
		id,
	).Scan(&session.ID, &session.Npub, &session.CreatedAt, &session.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		if err := s.Delete(ctx, id); err != nil {
			slog.Warn("failed to delete expired session", "id", id, "err", err)
		}
		return nil, nil
	}

	return &session, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	if err != nil {
		return fmt.Errorf("deleting expired sessions: %w", err)
	}
	return nil
}

func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
