package scan

import (
	"net/netip"
	"os"
	"os/exec"
	"strings"
)

// ReadNeighbors returns a map of canonical IP → MAC for entries with a
// completed resolution. Tries `ip neigh show` first (covers IPv4 + IPv6),
// falling back to /proc/net/arp (IPv4 only) if `ip` isn't on PATH.
func ReadNeighbors() (map[string]string, error) {
	if m, err := readIPNeigh(); err == nil && len(m) > 0 {
		return m, nil
	}
	return readProcArp()
}

func readIPNeigh() (map[string]string, error) {
	out, err := exec.Command("ip", "neigh", "show").Output()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		ipStr := f[0]
		var mac, state string
		for i := 1; i < len(f); i++ {
			switch f[i] {
			case "lladdr":
				if i+1 < len(f) {
					mac = f[i+1]
				}
			case "REACHABLE", "STALE", "DELAY", "PROBE", "PERMANENT", "NOARP", "FAILED", "INCOMPLETE":
				state = f[i]
			}
		}
		if mac == "" || mac == "00:00:00:00:00:00" {
			continue
		}
		if state == "FAILED" || state == "INCOMPLETE" {
			continue
		}
		if a, err := netip.ParseAddr(ipStr); err == nil {
			m[a.Unmap().String()] = mac
		}
	}
	return m, nil
}

func readProcArp() (map[string]string, error) {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for idx, ln := range lines {
		if idx == 0 {
			continue
		}
		f := strings.Fields(ln)
		if len(f) < 4 {
			continue
		}
		ip, flags, mac := f[0], f[2], f[3]
		if mac == "00:00:00:00:00:00" {
			continue
		}
		if flags == "0x0" {
			continue
		}
		m[ip] = mac
	}
	return m, nil
}
