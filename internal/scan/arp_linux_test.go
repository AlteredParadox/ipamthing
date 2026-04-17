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
