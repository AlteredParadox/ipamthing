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
	"testing"
)

const (
	testFallback   = "/default"
	testRemoteAddr = "192.168.1.10:45678"
	testClientIP   = "10.0.0.1"
	testOrigin     = "http://example.com/"
)

func TestSafeRedirect(t *testing.T) {
	cases := []struct {
		target   string
		fallback string
		want     string
	}{
		{"", testFallback, testFallback},
		{"/subnets", testFallback, "/subnets"},
		{"/subnets/1", testFallback, "/subnets/1"},
		{"/hosts/42?tab=events", testFallback, "/hosts/42?tab=events"},
		{"https://evil.example/", testFallback, testFallback},
		{"//evil.example/", testFallback, testFallback},
		{`/\evil.example`, testFallback, testFallback},
		{"evil.example", testFallback, testFallback},
		{"javascript:alert(1)", testFallback, testFallback},
	}
	for _, c := range cases {
		got := safeRedirect(c.target, c.fallback)
		if got != c.want {
			t.Errorf("safeRedirect(%q, %q) = %q, want %q", c.target, c.fallback, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"café", 4, "caf"},
	}
	for _, c := range cases {
		got := truncate(c.input, c.max)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.input, c.max, got, c.want)
		}
	}
}

func TestTruncateUTF8Boundary(t *testing.T) {
	input := "aaa💥bbb"
	got := truncate(input, 5)
	if len(got) > 5 {
		t.Errorf("truncate should not exceed max bytes, got %d", len(got))
	}
	for i := 0; i < len(got); i++ {
		if got[i] >= 0x80 && got[i] < 0xC0 {
			t.Errorf("truncate produced invalid UTF-8 continuation byte at position %d", i)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  hello   world  ", "hello world"},
		{"one", "one"},
		{"\ttab\nnewline", "tab newline"},
	}
	for _, c := range cases {
		got := normalizeTags(c.input)
		if got != c.want {
			t.Errorf("normalizeTags(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		trustProxy bool
		want       string
	}{
		{"direct", testRemoteAddr, "", false, "192.168.1.10"},
		{"xff ignored without trust", testRemoteAddr, testClientIP, false, "192.168.1.10"},
		{"xff single with trust", testRemoteAddr, testClientIP, true, testClientIP},
		{"xff rightmost with trust", testRemoteAddr, "spoof, 10.0.0.99", true, "10.0.0.99"},
		{"xff rightmost trimmed", testRemoteAddr, "spoof , 10.0.0.99 ", true, "10.0.0.99"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remoteAddr
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			got := clientIP(r, c.trustProxy)
			if got != c.want {
				t.Errorf("clientIP() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIsHTTPS(t *testing.T) {
	t.Run("plain http", func(t *testing.T) {
		r, _ := http.NewRequest("GET", testOrigin, nil)
		if isHTTPS(r, false) {
			t.Error("should be false for plain HTTP")
		}
	})
	t.Run("xff proto without trust", func(t *testing.T) {
		r, _ := http.NewRequest("GET", testOrigin, nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		if isHTTPS(r, false) {
			t.Error("should be false without trustProxy")
		}
	})
	t.Run("xff proto with trust", func(t *testing.T) {
		r, _ := http.NewRequest("GET", testOrigin, nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		if !isHTTPS(r, true) {
			t.Error("should be true with trustProxy and X-Forwarded-Proto: https")
		}
	})
}
