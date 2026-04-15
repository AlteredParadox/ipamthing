package scan

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"time"
)

func tcpProbe(ctx context.Context, ip netip.Addr, ports []int, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	for _, p := range ports {
		if ctx.Err() != nil {
			return false
		}
		dialCtx, cancel := context.WithTimeout(ctx, timeout)
		conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(p)))
		cancel()
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}
