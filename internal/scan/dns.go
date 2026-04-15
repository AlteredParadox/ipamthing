package scan

import (
	"context"
	"net"
	"strings"
	"time"
)

func reverseDNS(ctx context.Context, ip string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}
