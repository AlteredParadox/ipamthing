//go:build !linux && !windows

package scan

import "fmt"

// ReadNeighbors is not implemented on this platform. Scanning still works
// for liveness (ICMP + TCP) and DNS; only MAC discovery is unavailable.
func ReadNeighbors() (map[string]string, error) {
	return nil, fmt.Errorf("neighbor-table reading not implemented on this platform")
}
