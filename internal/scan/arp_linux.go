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
		if ip, mac, ok := parseNeighLine(line); ok {
			m[ip] = mac
		}
	}
	return m, nil
}

func parseNeighLine(line string) (ip, mac string, ok bool) {
	f := strings.Fields(line)
	if len(f) < 2 {
		return "", "", false
	}
	var macAddr, state string
	for i := 1; i < len(f); i++ {
		switch f[i] {
		case "lladdr":
			if i+1 < len(f) {
				macAddr = f[i+1]
			}
		case "REACHABLE", "STALE", "DELAY", "PROBE", "PERMANENT", "NOARP", "FAILED", "INCOMPLETE":
			state = f[i]
		}
	}
	if macAddr == "" || macAddr == "00:00:00:00:00:00" {
		return "", "", false
	}
	if state == "FAILED" || state == "INCOMPLETE" {
		return "", "", false
	}
	a, err := netip.ParseAddr(f[0])
	if err != nil {
		return "", "", false
	}
	return a.Unmap().String(), macAddr, true
}

func readProcArp() (map[string]string, error) {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, ln := range strings.Split(string(data), "\n")[1:] {
		f := strings.Fields(ln)
		if len(f) < 4 {
			continue
		}
		ip, flags, mac := f[0], f[2], f[3]
		if mac == "00:00:00:00:00:00" || flags == "0x0" {
			continue
		}
		m[ip] = mac
	}
	return m, nil
}
