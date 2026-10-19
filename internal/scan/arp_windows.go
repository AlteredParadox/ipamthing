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

//go:build windows

package scan

import (
	"net/netip"
	"os/exec"
	"strings"
)

// ReadNeighbors parses the output of `arp -a` on Windows. Sample:
//
//	Interface: 192.168.1.100 --- 0xc
//	  Internet Address      Physical Address      Type
//	  192.168.1.1           aa-bb-cc-dd-ee-ff     dynamic
//	  192.168.1.2           11-22-33-44-55-66     static
func ReadNeighbors() (map[string]string, error) {
	out, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		if f[0] == "Interface:" || f[0] == "Internet" {
			continue
		}
		ipStr := f[0]
		macStr := f[1]
		if macStr == "ff-ff-ff-ff-ff-ff" || macStr == "00-00-00-00-00-00" {
			continue
		}
		macStr = strings.ReplaceAll(macStr, "-", ":")
		if a, err := netip.ParseAddr(ipStr); err == nil {
			m[a.Unmap().String()] = macStr
		}
	}
	return m, nil
}
