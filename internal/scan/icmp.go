package scan

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	protoICMPv4 = 1
	protoICMPv6 = 58
	modeRaw     = "raw"
	modeUDP     = "udp"
)

// ICMP wraps per-family echo sweeping for IPv4 and IPv6.
type ICMP struct {
	v4 *icmpFamily
	v6 *icmpFamily
}

type icmpFamily struct {
	conn *icmp.PacketConn
	mode string
	v6   bool
	mu   sync.Mutex
}

// NewICMP opens whatever sockets the host permits. Returns an error only if
// neither family is usable.
func NewICMP(logger *slog.Logger) (*ICMP, error) {
	v4, err4 := openICMPFamily(false)
	v6, err6 := openICMPFamily(true)
	if v4 == nil && v6 == nil {
		return nil, fmt.Errorf("no ICMP socket (v4=%v, v6=%v)", err4, err6)
	}
	if v4 != nil {
		logger.Info("ICMPv4 ready", "mode", v4.mode)
	} else {
		logger.Warn("ICMPv4 unavailable", "err", err4)
	}
	if v6 != nil {
		logger.Info("ICMPv6 ready", "mode", v6.mode)
	} else {
		logger.Warn("ICMPv6 unavailable", "err", err6)
	}
	return &ICMP{v4: v4, v6: v6}, nil
}

func (i *ICMP) HasV4() bool { return i != nil && i.v4 != nil }
func (i *ICMP) HasV6() bool { return i != nil && i.v6 != nil }

func (i *ICMP) Mode() string {
	var parts []string
	if i.v4 != nil {
		parts = append(parts, "v4="+i.v4.mode)
	}
	if i.v6 != nil {
		parts = append(parts, "v6="+i.v6.mode)
	}
	if len(parts) == 0 {
		return "none"
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func openICMPFamily(v6 bool) (*icmpFamily, error) {
	var rawNet, udpNet, listenAddr string
	if v6 {
		rawNet, udpNet, listenAddr = "ip6:ipv6-icmp", "udp6", "::"
	} else {
		rawNet, udpNet, listenAddr = "ip4:icmp", "udp4", "0.0.0.0"
	}
	if c, err := icmp.ListenPacket(rawNet, listenAddr); err == nil {
		return &icmpFamily{conn: c, mode: modeRaw, v6: v6}, nil
	}
	c, err := icmp.ListenPacket(udpNet, listenAddr)
	if err != nil {
		return nil, err
	}
	return &icmpFamily{conn: c, mode: modeUDP, v6: v6}, nil
}

func (i *ICMP) Sweep(ctx context.Context, targets []netip.Addr, timeout time.Duration) map[netip.Addr]bool {
	var v4, v6 []netip.Addr
	for _, t := range targets {
		if t.Is4() {
			v4 = append(v4, t)
		} else {
			v6 = append(v6, t)
		}
	}
	responses := make(map[netip.Addr]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup
	collect := func(m map[netip.Addr]bool) {
		mu.Lock()
		for k, v := range m {
			responses[k] = v
		}
		mu.Unlock()
	}
	if len(v4) > 0 && i.v4 != nil {
		wg.Add(1)
		go func() { defer wg.Done(); collect(i.v4.sweep(ctx, v4, timeout)) }()
	}
	if len(v6) > 0 && i.v6 != nil {
		wg.Add(1)
		go func() { defer wg.Done(); collect(i.v6.sweep(ctx, v6, timeout)) }()
	}
	wg.Wait()
	return responses
}

func (f *icmpFamily) sweep(ctx context.Context, targets []netip.Addr, timeout time.Duration) map[netip.Addr]bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	responses := make(map[netip.Addr]bool)
	buf := make([]byte, 1500)

	_ = f.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	for {
		if _, _, err := f.conn.ReadFrom(buf); err != nil {
			break
		}
	}

	var msgType, expected icmp.Type
	var proto int
	if f.v6 {
		msgType, expected, proto = ipv6.ICMPTypeEchoRequest, ipv6.ICMPTypeEchoReply, protoICMPv6
	} else {
		msgType, expected, proto = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply, protoICMPv4
	}

	id := os.Getpid() & 0xffff
	for seq, target := range targets {
		if ctx.Err() != nil {
			return responses
		}
		msg := icmp.Message{
			Type: msgType, Code: 0,
			Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("ipamthing")},
		}
		body, err := msg.Marshal(nil)
		if err != nil {
			continue
		}
		var dst net.Addr
		if f.mode == modeRaw {
			dst = &net.IPAddr{IP: target.AsSlice()}
		} else {
			dst = &net.UDPAddr{IP: target.AsSlice()}
		}
		_, _ = f.conn.WriteTo(body, dst)
	}

	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			break
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if remaining > 100*time.Millisecond {
			remaining = 100 * time.Millisecond
		}
		_ = f.conn.SetReadDeadline(time.Now().Add(remaining))
		n, peer, err := f.conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			break
		}
		var ipBytes []byte
		switch p := peer.(type) {
		case *net.UDPAddr:
			ipBytes = p.IP
		case *net.IPAddr:
			ipBytes = p.IP
		}
		if len(ipBytes) == 0 {
			continue
		}
		var addr netip.Addr
		if !f.v6 {
			if v4 := net.IP(ipBytes).To4(); v4 != nil {
				ipBytes = v4
			}
		}
		a, ok := netip.AddrFromSlice(ipBytes)
		if !ok {
			continue
		}
		addr = a.Unmap()
		msg, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			continue
		}
		if msg.Type != expected {
			continue
		}
		responses[addr] = true
	}
	return responses
}
