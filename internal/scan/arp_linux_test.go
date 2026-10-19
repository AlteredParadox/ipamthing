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

//go:build linux

package scan

import "testing"

func TestParseNeighLine(t *testing.T) {
	cases := []struct {
		line    string
		wantIP  string
		wantMAC string
		wantOK  bool
	}{
		{
			"192.168.1.1 dev eth0 lladdr aa:bb:cc:dd:ee:ff REACHABLE",
			"192.168.1.1", "aa:bb:cc:dd:ee:ff", true,
		},
		{
			"fe80::1 dev wlan0 lladdr 11:22:33:44:55:66 router STALE",
			"fe80::1", "11:22:33:44:55:66", true,
		},
		{
			"192.168.1.50 dev eth0 FAILED",
			"", "", false,
		},
		{
			"192.168.1.51 dev eth0 lladdr 00:00:00:00:00:00 REACHABLE",
			"", "", false,
		},
		{
			"192.168.1.52 dev eth0 INCOMPLETE",
			"", "", false,
		},
		{
			"",
			"", "", false,
		},
		{
			"short",
			"", "", false,
		},
	}
	for _, c := range cases {
		ip, mac, ok := parseNeighLine(c.line)
		if ok != c.wantOK {
			t.Errorf("parseNeighLine(%q): ok = %v, want %v", c.line, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if ip != c.wantIP {
			t.Errorf("parseNeighLine(%q): ip = %q, want %q", c.line, ip, c.wantIP)
		}
		if mac != c.wantMAC {
			t.Errorf("parseNeighLine(%q): mac = %q, want %q", c.line, mac, c.wantMAC)
		}
	}
}
