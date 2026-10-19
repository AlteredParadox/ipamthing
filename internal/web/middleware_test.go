package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testActionPath  = "/action"
	testFormContent = "application/x-www-form-urlencoded"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestSecurityHeaders(t *testing.T) {
	s := &Server{}
	handler := s.securityHeaders(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	}
	for header, want := range expected {
		got := rec.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header missing")
	}
	for _, directive := range []string{"default-src", "script-src", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive %q", directive)
		}
	}
}

func TestCSRFAllowsGET(t *testing.T) {
	s := &Server{}
	handler := s.csrf(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET should pass through, got %d", rec.Code)
	}
}

func TestCSRFBlocksPOSTWithoutToken(t *testing.T) {
	s := &Server{}
	handler := s.csrf(okHandler())

	req := httptest.NewRequest("POST", testActionPath, strings.NewReader("x=1"))
	req.Header.Set(headerContentType, testFormContent)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF should be 403, got %d", rec.Code)
	}
}

func TestCSRFBlocksMismatchedToken(t *testing.T) {
	s := &Server{}
	handler := s.csrf(okHandler())

	req := httptest.NewRequest("POST", testActionPath,
		strings.NewReader("csrf_token=wrong-value"))
	req.Header.Set(headerContentType, testFormContent)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: "correct-value"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("POST with mismatched CSRF should be 403, got %d", rec.Code)
	}
}

func TestCSRFAcceptsMatchingToken(t *testing.T) {
	s := &Server{}
	handler := s.csrf(okHandler())

	token := "a]b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	req := httptest.NewRequest("POST", testActionPath,
		strings.NewReader("csrf_token="+token))
	req.Header.Set(headerContentType, testFormContent)
	req.AddCookie(&http.Cookie{Name: "csrf", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST with matching CSRF should be 200, got %d", rec.Code)
	}
}

func TestCSRFSkipsStaticPaths(t *testing.T) {
	s := &Server{}
	handler := s.csrf(okHandler())

	req := httptest.NewRequest("POST", "/static/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST to /static/ should skip CSRF, got %d", rec.Code)
	}
}

func TestLimitBodySmall(t *testing.T) {
	s := &Server{}
	handler := s.limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusRequestEntityTooLarge)
			return
		}
		w.Write(body)
	}))

	req := httptest.NewRequest("POST", "/", strings.NewReader("small body"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("small body should succeed, got %d", rec.Code)
	}
}

func TestLimitBodyLarge(t *testing.T) {
	s := &Server{}
	handler := s.limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	bigBody := strings.NewReader(strings.Repeat("x", 2*1024*1024))
	req := httptest.NewRequest("POST", "/", bigBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("2MiB body should have been rejected by MaxBytesReader")
	}
}
