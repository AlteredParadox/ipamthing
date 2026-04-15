package web

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"ipamthing/internal/auth"
)

// securityHeaders sets a conservative set of response headers that harden
// the browser-side attack surface (clickjacking, MIME sniffing, referer
// leakage) and a CSP that limits scripts/styles/form posts to same-origin.
// Inline scripts are not allowed — any JS lives under /static/.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self'; " +
		"img-src 'self' data:; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

// csrf enforces double-submit cookie CSRF on state-changing requests.
// For GET/HEAD/OPTIONS it is a no-op. For other methods, it requires a
// "csrf_token" form value that matches the "csrf" cookie byte-for-byte.
// render() is responsible for issuing the cookie on the first GET.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		// Skip static files (they're served by a sub-mux but guard anyway).
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie("csrf")
		if err != nil || c.Value == "" {
			http.Error(w, "CSRF cookie missing — reload the page and retry", http.StatusForbidden)
			return
		}
		submitted := r.FormValue("csrf_token")
		if submitted == "" || subtle.ConstantTimeCompare([]byte(submitted), []byte(c.Value)) != 1 {
			http.Error(w, "CSRF token mismatch — reload the page and retry", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
