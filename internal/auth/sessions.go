// ipamthing — a lightweight single-binary IPAM.
// Copyright (C) 2026 AlteredParadox
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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

// DeleteOthers invalidates every session for userID except keepToken. Used
// to rotate sessions on login so a successful auth logs the user out of
// any other devices or browsers that had a prior session.
func (s *SessionStore) DeleteOthers(userID int64, keepToken string) error {
	_, err := s.db.Exec(
		`DELETE FROM sessions WHERE user_id = ? AND token != ?`,
		userID, keepToken,
	)
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
