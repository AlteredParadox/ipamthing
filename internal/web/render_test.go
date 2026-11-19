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
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ipamthing/internal/db"
)

func TestNavFor(t *testing.T) {
	cases := []struct {
		page     string
		wantNav  string
		wantBare bool
	}{
		{"login", "", true},
		{"setup", "", true},
		{"dashboard", "dashboard", false},
		{"subnets", "subnets", false},
		{"hosts", "subnets", false},
		{"host", "subnets", false},
		{"events", "events", false},
		{"settings", "settings", false},
	}
	for _, c := range cases {
		nav, bare := navFor(c.page)
		if nav != c.wantNav || bare != c.wantBare {
			t.Errorf("navFor(%q) = (%q, %v), want (%q, %v)", c.page, nav, bare, c.wantNav, c.wantBare)
		}
	}
}

// renderPage runs one page through the real embedded templates and returns
// the HTML. uid 0 renders as a logged-out visitor.
func renderPage(t *testing.T, s *Server, page string, uid int64) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if uid != 0 {
		r = r.WithContext(context.WithValue(r.Context(), userIDKey, uid))
	}
	rec := httptest.NewRecorder()
	s.render(rec, r, page, pageData{Title: page})
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200", page, rec.Code)
	}
	return rec.Body.String()
}

// Login and first-run setup are a bare card: no sidebar to navigate with,
// but the AGPL links must still be on the page. Logged-out pages are dark.
func TestRenderBarePages(t *testing.T) {
	s := &Server{tmpls: parseTemplates(), version: "v9.9.9"}
	for _, page := range []string{"login", "setup"} {
		body := renderPage(t, s, page, 0)
		if strings.Contains(body, "<aside>") {
			t.Errorf("%s: rendered the sidebar", page)
		}
		for _, want := range []string{
			`data-theme="dark"`, `class="login-card"`, `name="csrf_token"`,
			`href="/license"`, `href="/third-party-licenses"`, `href="/source"`, "v9.9.9",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing %q", page, want)
			}
		}
	}
}

// Every signed-in page carries the sidebar with exactly one highlighted
// entry, and honours the theme, layout and density stored on the account.
func TestRenderSidebarAndStoredPrefs(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(
		`INSERT INTO users (id, username, password_hash, ui_theme, ui_layout, ui_density, created_at)
		 VALUES (1, 'a', 'x', 'light', 'wide', 'compact', 0), (2, 'b', 'x', 'dark', 'normal', 'standard', 0)`,
	); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	s := &Server{tmpls: parseTemplates(), db: d, version: "dev"}

	body := renderPage(t, s, "settings", 1)
	for _, want := range []string{
		`data-theme="light"`, `data-layout="wide"`, `data-density="compact"`,
		`<a class="nav active" href="/settings">`, `action="/logout"`,
		`href="/license"`, `href="/third-party-licenses"`, `href="/source"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings (light user): missing %q", want)
		}
	}
	if n := strings.Count(body, "nav active"); n != 1 {
		t.Errorf("settings: %d active nav entries, want 1", n)
	}
	if strings.Contains(body, `class="login-card"`) {
		t.Error("settings: rendered the bare login card")
	}

	body = renderPage(t, s, "subnets", 2)
	for _, want := range []string{
		`data-theme="dark"`, `data-layout="normal"`, `data-density="standard"`,
		`<a class="nav active" href="/subnets">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("subnets (dark user): missing %q", want)
		}
	}
}

func TestRenderUnknownPage(t *testing.T) {
	s := &Server{tmpls: parseTemplates()}
	rec := httptest.NewRecorder()
	s.render(rec, httptest.NewRequest(http.MethodGet, "/", nil), "nope", pageData{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
