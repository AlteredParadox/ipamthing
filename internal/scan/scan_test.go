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

package scan

import "testing"

func TestEnumerateIPsV4(t *testing.T) {
	cases := []struct {
		cidr  string
		wantN int
		first string
		last  string
	}{
		{"192.168.1.0/24", 254, "192.168.1.1", "192.168.1.254"},
		{"10.0.0.0/30", 2, "10.0.0.1", "10.0.0.2"},
		{"10.0.0.0/31", 2, "10.0.0.0", "10.0.0.1"},
		{"10.0.0.5/32", 1, "10.0.0.5", "10.0.0.5"},
	}
	for _, c := range cases {
		got, err := enumerateIPs(c.cidr)
		if err != nil {
			t.Fatalf("%s: %v", c.cidr, err)
		}
		if len(got) != c.wantN {
			t.Errorf("%s: got %d IPs, want %d", c.cidr, len(got), c.wantN)
			continue
		}
		if got[0].String() != c.first {
			t.Errorf("%s: first = %s, want %s", c.cidr, got[0], c.first)
		}
		if got[len(got)-1].String() != c.last {
			t.Errorf("%s: last = %s, want %s", c.cidr, got[len(got)-1], c.last)
		}
	}
}

func TestEnumerateIPsV6(t *testing.T) {
	cases := []struct {
		cidr  string
		wantN int
		first string
		last  string
	}{
		{"2001:db8::/120", 256, "2001:db8::", "2001:db8::ff"},
		{"2001:db8::/126", 4, "2001:db8::", "2001:db8::3"},
		{"2001:db8::1/128", 1, "2001:db8::1", "2001:db8::1"},
	}
	for _, c := range cases {
		got, err := enumerateIPs(c.cidr)
		if err != nil {
			t.Fatalf("%s: %v", c.cidr, err)
		}
		if len(got) != c.wantN {
			t.Errorf("%s: got %d IPs, want %d", c.cidr, len(got), c.wantN)
			continue
		}
		if got[0].String() != c.first {
			t.Errorf("%s: first = %s, want %s", c.cidr, got[0], c.first)
		}
		if got[len(got)-1].String() != c.last {
			t.Errorf("%s: last = %s, want %s", c.cidr, got[len(got)-1], c.last)
		}
	}
}

func TestEnumerateIPsTooLarge(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0/8", "2001:db8::/64"} {
		if _, err := enumerateIPs(cidr); err == nil {
			t.Errorf("expected error for %s, got nil", cidr)
		}
	}
}
