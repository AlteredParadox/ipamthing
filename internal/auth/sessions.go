package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

const (
	SessionCookieName = "ipamthing_session"
	SessionTTL        = 30 * 24 * time.Hour
)

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(userID int64) (*Session, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(buf)
	expires := time.Now().Add(SessionTTL)
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires.Unix(),
	)
	if err != nil {
		return nil, err
	}
	return &Session{Token: token, UserID: userID, ExpiresAt: expires}, nil
}

func (s *SessionStore) Get(token string) (*Session, error) {
	var userID int64
	var exp int64
	err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&userID, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if time.Now().Unix() > exp {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil, nil
	}
	return &Session{Token: token, UserID: userID, ExpiresAt: time.Unix(exp, 0)}, nil
}

func (s *SessionStore) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *SessionStore) PurgeLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
		}
	}
}
