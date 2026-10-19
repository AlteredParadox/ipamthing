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

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeLegalText(t *testing.T) {
	h := serveLegalText([]byte("GNU AFFERO"), "missing")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/license", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get(headerContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain", ct)
	}
	if rec.Body.String() != "GNU AFFERO" {
		t.Errorf("body = %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	serveLegalText(nil, "missing").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/license", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("empty body status = %d, want 500", rec.Code)
	}
}

func TestSourceURLPinning(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"", sourceBaseURL},
		{"dev", sourceBaseURL},
		{"v1.2.3", sourceBaseURL + "/tree/v1.2.3"},
		{"v1.2.3-4-gabc1234", sourceBaseURL},
		{"abc1234-dirty", sourceBaseURL},
		{"6ddccf13af76b49c7df9ba42ff29d92c0b49f52a", sourceBaseURL + "/tree/6ddccf13af76b49c7df9ba42ff29d92c0b49f52a"},
	}
	for _, c := range cases {
		s := &Server{version: c.version}
		rec := httptest.NewRecorder()
		s.handleSource(rec, httptest.NewRequest(http.MethodGet, "/source", nil))
		if rec.Code != http.StatusFound {
			t.Errorf("version %q: status = %d, want 302", c.version, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Errorf("version %q: Location = %q, want %q", c.version, got, c.want)
		}
	}
}

// The legal routes must be reachable before first-run setup: firstRun
// would otherwise query the users table (nil here, so a panic would mean
// the exemption is broken) and redirect to /setup.
func TestFirstRunExemptsLegalPaths(t *testing.T) {
	s := &Server{}
	for _, p := range []string{"/license", "/third-party-licenses", "/source"} {
		called := false
		h := s.firstRun(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if !called || rec.Code != http.StatusOK {
			t.Errorf("%s: firstRun did not pass through (called=%v code=%d)", p, called, rec.Code)
		}
	}
	if isLegalPath("/licenses") || isLegalPath("/license/") || isLegalPath("/sources") {
		t.Error("isLegalPath matched a non-legal path")
	}
}
