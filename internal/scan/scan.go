package scan

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"
)

type Result struct {
	IP       string
	Alive    bool
	MAC      string
	Hostname string
	Method   string
}

type Scanner struct {
	icmp        *ICMP
	Timeout     time.Duration
	TCPPorts    []int
	Workers     int
	DNSTimeout  time.Duration
	DNSOverall  time.Duration
	logger      *slog.Logger
	icmpEnabled bool
}

func NewScanner(logger *slog.Logger) *Scanner {
	s := &Scanner{
		Timeout:    1500 * time.Millisecond,
		TCPPorts:   []int{22, 80, 443, 445, 3389},
		Workers:    64,
		DNSTimeout: 2 * time.Second,
		DNSOverall: 15 * time.Second,
		logger:     logger,
	}
	icmp, err := NewICMP(logger)
	if err != nil {
		logger.Warn("ICMP scanner unavailable; TCP/ARP only", "err", err)
		return s
	}
	s.icmp = icmp
	s.icmpEnabled = true
	logger.Info("ICMP scanner ready", "mode", icmp.Mode())
	return s
}

func (s *Scanner) ICMPAvailable() bool { return s.icmpEnabled }

func (s *Scanner) Scan(ctx context.Context, cidr string) ([]Result, error) {
	ips, err := enumerateIPs(cidr)
	if err != nil {
		return nil, err
	}
	results := make([]Result, len(ips))
	for i, ip := range ips {
		results[i].IP = ip.String()
	}

	if s.icmp != nil {
		s.applyICMP(ctx, ips, results)
	}
	s.tcpSweep(ctx, ips, results)
	s.enrichWithNeighbors(results)
	s.resolveDNS(ctx, results)

	return results, nil
}

func (s *Scanner) applyICMP(ctx context.Context, ips []netip.Addr, results []Result) {
	resp := s.icmp.Sweep(ctx, ips, s.Timeout)
	for i, ip := range ips {
		if resp[ip] {
			results[i].Alive = true
			results[i].Method = "icmp"
		}
	}
}

func (s *Scanner) enrichWithNeighbors(results []Result) {
	neighbors, err := ReadNeighbors()
	if err != nil {
		s.logger.Debug("neighbor read failed", "err", err)
		return
	}
	for i := range results {
		mac, ok := neighbors[results[i].IP]
		if !ok {
			continue
		}
		results[i].MAC = mac
		if !results[i].Alive {
			results[i].Alive = true
			results[i].Method = "arp"
		}
	}
}

func (s *Scanner) tcpSweep(ctx context.Context, ips []netip.Addr, results []Result) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.Workers)
	for i := range results {
		if results[i].Alive {
			continue
		}
		i := i
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if tcpProbe(ctx, ips[i], s.TCPPorts, s.Timeout) {
				results[i].Alive = true
				results[i].Method = "tcp"
			}
		}()
	}
	wg.Wait()
}

func (s *Scanner) resolveDNS(ctx context.Context, results []Result) {
	dnsCtx, cancel := context.WithTimeout(ctx, s.DNSOverall)
	defer cancel()
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.Workers)
	for i := range results {
		if !results[i].Alive {
			continue
		}
		i := i
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i].Hostname = reverseDNS(dnsCtx, results[i].IP, s.DNSTimeout)
		}()
	}
	wg.Wait()
}

// enumerateIPs expands a CIDR into concrete addresses. IPv4 is capped at /16
// (64K hosts) and skips the network/broadcast for prefixes larger than /31.
// IPv6 is capped at /120 (256 hosts) and includes every address in the block.
func enumerateIPs(cidr string) ([]netip.Addr, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, err
	}
	prefix = prefix.Masked()
	addr := prefix.Addr()
	bits := prefix.Bits()

	var hostBits, maxHostBits, fullBits int
	if addr.Is4() {
		fullBits = 32
		maxHostBits = 16
	} else {
		fullBits = 128
		maxHostBits = 8
	}
	hostBits = fullBits - bits
	if hostBits > maxHostBits {
		return nil, fmt.Errorf("prefix /%d too large (max /%d for this family)", bits, fullBits-maxHostBits)
	}

	count := uint64(1) << uint64(hostBits)
	skipEndpoints := addr.Is4() && hostBits > 1

	out := make([]netip.Addr, 0, count)
	cur := addr
	for i := uint64(0); i < count; i++ {
		if !(skipEndpoints && (i == 0 || i == count-1)) {
			out = append(out, cur)
		}
		cur = cur.Next()
		if !cur.IsValid() {
			break
		}
	}
	return out, nil
}
