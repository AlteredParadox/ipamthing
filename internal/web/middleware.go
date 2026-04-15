package web

import (
	"context"
	"net/http"
	"strings"

	"ipamthing/internal/auth"
)

type ctxKey string

const userIDKey ctxKey = "userID"

func (s *Server) firstRun(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/setup" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if count == 0 {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		sess, err := s.sessions.Get(c.Value)
		if err != nil || sess == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
