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
	"sync"
	"time"
)

// LoginLimiter is an in-memory per-key rate limiter for failed logins.
// It locks out a key (typically client IP) after too many failures within a
// sliding window. Successful logins clear the key. On process restart all
// state is dropped.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginRecord
	maxFails int
	window   time.Duration
	lockout  time.Duration
}

type loginRecord struct {
	count       int
	firstAt     time.Time
	lockedUntil time.Time
}

func NewLoginLimiter(maxFails int, window, lockout time.Duration) *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string]*loginRecord),
		maxFails: maxFails,
		window:   window,
		lockout:  lockout,
	}
}

// Allow returns (true, 0) if the caller may attempt a login, or
// (false, retryAfter) if they are currently locked out.
func (l *LoginLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	rec := l.attempts[key]
	if rec == nil {
		return true, 0
	}
	if now.Before(rec.lockedUntil) {
		return false, rec.lockedUntil.Sub(now)
	}
	if now.Sub(rec.firstAt) > l.window {
		delete(l.attempts, key)
	}
	return true, 0
}

// Fail records a failed attempt. Once the count within the window reaches
// maxFails, the key is locked out for lockout.
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	rec := l.attempts[key]
	if rec == nil || now.Sub(rec.firstAt) > l.window {
		l.attempts[key] = &loginRecord{count: 1, firstAt: now}
		return
	}
	rec.count++
	if rec.count >= l.maxFails {
		rec.lockedUntil = now.Add(l.lockout)
	}
}

// Success clears any failure record for the key.
func (l *LoginLimiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// PurgeLoop periodically removes records whose lockouts have expired and
// whose windows have elapsed, bounding memory usage.
func (l *LoginLimiter) PurgeLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			l.mu.Lock()
			for k, rec := range l.attempts {
				if now.After(rec.lockedUntil) && now.Sub(rec.firstAt) > l.window {
					delete(l.attempts, k)
				}
			}
			l.mu.Unlock()
		}
	}
}
